package app

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/transfer"
)

// importTimeout is what a whole import is given, and it is a file run's two hours rather than a
// read's fifteen seconds for the same reason: a file of a few hundred thousand rows is a long job
// that is going fine, and a clock that cut it off would be the only thing wrong with it. Stopping
// it is the reader's own Stop, through the run registry.
const importTimeout = 2 * time.Hour

// previewRows is how much of a file the mapping step is shown. It is a *sample*: enough to see that
// the delimiter is right and that the columns landed where they were meant to, and few enough that
// pointing the wizard at a gigabyte costs a screenful of reading rather than a gigabyte of it.
const previewRows = 50

// defaultImportBatch is how many rows one statement carries when the reader has not said. It is a
// compromise between round trips and `max_allowed_packet`, and the Advanced step is where a reader
// whose server disagrees says so.
const defaultImportBatch = 200

// ImportSource is the file an import reads and how to read it. It is the whole of what the first
// four steps of the wizard fill in, and nothing in it has anything to do with a database — which is
// why the preview needs no session.
type ImportSource struct {
	Format  string          `json:"format"`
	Path    string          `json:"path"`
	Options TransferOptions `json:"options"`
}

// ImportPreview is what the file turned out to hold: the fields it names and the head of its rows,
// which is what the mapping step draws.
type ImportPreview struct {
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}

// FieldMapping is one of the file's fields pointed at one of the table's columns.
//
// `source` is empty for a column the file has nothing for, which is left out of the write entirely
// so the server's own default applies — that is the difference between "not mentioned" and NULL, and
// it is the reason a mapping is a list rather than two parallel ones.
type FieldMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	// Key says this column is part of what identifies a stored row, which every mode but Append and
	// Copy finds its rows by.
	Key bool `json:"key"`
}

// ImportInput is a whole import: the file, the table, and what the rows are to do when they get
// there.
type ImportInput struct {
	Namespace
	ImportSource
	// RunID is minted by the caller so a running import can be stopped — Cancel takes it, exactly
	// as it takes a file run's.
	RunID    string         `json:"runId"`
	Table    string         `json:"table"`
	Mode     string         `json:"mode"`
	Mappings []FieldMapping `json:"mappings"`
	// Batch is how many rows one statement carries, for the modes whose statement can carry more
	// than one. Zero takes defaultImportBatch.
	Batch int `json:"batch"`
	// ContinueOnError runs the rest of the file after a statement the server rejects, exactly as a
	// script file's does. Off, the import stops where it stopped — which is what a file whose rows
	// depend on each other wants, and what a reader who is watching the first error wants too.
	ContinueOnError bool `json:"continueOnError"`
}

// ImportReview is what an import is going to do, answered before it does any of it. `confirm` is
// the whole point of the shape: it is domain.Environment.NeedsConfirmation's answer, worked out on
// this side from the mode and the connection's environment, so the wizard never re-derives a rule
// about what is dangerous — the same bargain ScriptReview strikes for a script.
type ImportReview struct {
	Hazard      string `json:"hazard"`
	Confirm     bool   `json:"confirm"`
	Environment string `json:"environment"`
	// Statement is the first statement the import would run, built from the file's own first rows.
	// It is what a reader checks before they let a hundred thousand of its like go to a server.
	Statement string `json:"statement"`
}

// ImportResult is what the import did. Rows read and rows written are two numbers on purpose: a
// batch of two hundred rows is one statement, and a mode that changes nothing writes none of them.
type ImportResult struct {
	Read       int               `json:"read"`
	Statements int               `json:"statements"`
	Written    int64             `json:"written"`
	Failed     int               `json:"failed"`
	ElapsedMs  int64             `json:"elapsedMs"`
	Messages   []TransferMessage `json:"messages"`
}

// PreviewImportFile reads the head of the file and answers with what it holds. It needs no session
// at all — a file is a file — and it is what the wizard's mapping step is drawn from.
func (*ConnectionAPI) PreviewImportFile(source ImportSource) (ImportPreview, error) {
	options := source.Options.toTransfer()
	options.Limit = previewRows

	reader, err := transfer.Open(source.Format, source.Path, options)
	if err != nil {
		return ImportPreview{}, wrapError(err)
	}
	defer reader.Close()

	preview := ImportPreview{Columns: reader.Columns(), Rows: [][]*string{}}
	for {
		row, err := reader.Next()
		if err == io.EOF {
			return preview, nil
		}
		if err != nil {
			return ImportPreview{}, wrapError(err)
		}
		preview.Rows = append(preview.Rows, row)
	}
}

// ReviewImport answers what this import will do before it does it, and whether it has to be
// answered for first. The statement it shows is built from the file's own first rows through the
// very method the run will use, so what is previewed and what runs cannot drift — the same rule
// `CreateDatabaseSQL` follows.
func (a *ConnectionAPI) ReviewImport(sessionID string, input ImportInput) (ImportReview, error) {
	session, err := a.sessions.Get(sessionID)
	if err != nil {
		return ImportReview{}, wrapError(err)
	}

	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return ImportReview{}, wrapError(err)
	}

	mode, write, err := input.plan()
	if err != nil {
		return ImportReview{}, wrapError(err)
	}

	hazard := domain.ImportHazard(mode, len(write.Keys) > 0)
	review := ImportReview{
		Hazard:      string(hazard),
		Confirm:     session.Profile.Environment.NeedsConfirmation(hazard),
		Environment: string(session.Profile.Environment),
	}

	preview, err := a.PreviewImportFile(input.ImportSource)
	if err != nil {
		// A file that cannot be read is the *next* step's failure to report, not this one's: the
		// hazard is a fact about the mode and is true whether or not the file opens.
		return review, nil
	}

	write.Rows = input.take(preview, min(len(preview.Rows), write.Batch))
	if statements := schema.RowStatements(input.Namespace.toDriver(), input.Table, write); len(statements) > 0 {
		review.Statement = statements[0]
	}
	return review, nil
}

// RunImport writes a file into a table, and says what each batch of it did.
//
// It is `RunFile`'s sibling and shares its shape for the reason they share a reader's expectations:
// a long job, no rows coming back, a transcript of what happened, and a rejected statement that need
// not end the run. What it adds is the file being *data* rather than statements — so the statements
// are built here, through the driver, and a value the reader typed nowhere near a server still
// reaches one only as a quoted literal.
func (a *ConnectionAPI) RunImport(sessionID string, input ImportInput) (ImportResult, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return ImportResult{}, wrapError(err)
	}

	mode, write, err := input.plan()
	if err != nil {
		return ImportResult{}, wrapError(err)
	}

	reader, err := transfer.Open(input.Format, input.Path, input.Options.toTransfer())
	if err != nil {
		return ImportResult{}, wrapError(err)
	}
	defer reader.Close()

	ctx, release := a.runs.begin(a.ctx, input.RunID, importTimeout)
	defer release()

	started := time.Now()
	result := ImportResult{Messages: []TransferMessage{
		transferMessage("Reading " + input.Path),
	}}

	// Copy is the one mode that touches rows the file says nothing about, and it does so *before*
	// the first of them is written — which is why it is a mode and not a checkbox: a run that
	// emptied the table and then failed to open the file would be the worst outcome available.
	if mode == domain.ImportCopy {
		if err := schema.EmptyTable(ctx, input.Namespace.toDriver(), input.Table); err != nil {
			result.Messages = append(result.Messages, transferFailure("Emptying "+input.Table, err))
			result.Failed++
			result.ElapsedMs = time.Since(started).Milliseconds()
			return result, nil
		}
		result.Messages = append(result.Messages,
			transferMessage("Emptied "+input.Table+" before writing"))
	}

	a.writeRows(ctx, sessionID, schema, reader, input, write, &result)

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

// writeRows reads the file batch by batch and writes each batch as it fills, so an import of a
// million rows costs one batch of memory — the same bargain a dump strikes on the way out.
func (a *ConnectionAPI) writeRows(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	reader transfer.Reader,
	input ImportInput,
	write driver.RowWrite,
	result *ImportResult,
) {
	sources := index(reader.Columns())
	batch := make([][]*string, 0, write.Batch)

	flush := func() bool {
		if len(batch) == 0 {
			return true
		}
		write.Rows = batch
		carry := a.runStatements(ctx, sessionID, schema, input, write, result)
		batch = batch[:0]
		return carry
	}

	for {
		row, err := reader.Next()
		if err == io.EOF {
			flush()
			return
		}
		if err != nil {
			result.Messages = append(result.Messages, transferFailure("Reading the file", err))
			result.Failed++
			return
		}

		result.Read++
		batch = append(batch, mapped(row, sources, input.Mappings))

		if len(batch) >= write.Batch && !flush() {
			return
		}
	}
}

// runStatements runs what one batch of rows became, and answers with whether the import carries on.
func (a *ConnectionAPI) runStatements(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	input ImportInput,
	write driver.RowWrite,
	result *ImportResult,
) bool {
	ns := input.Namespace.toDriver()

	for _, statement := range schema.RowStatements(ns, input.Table, write) {
		result.Statements++

		ran, err := a.sessions.Query(ctx, sessionID, driver.Query{
			Namespace: ns,
			SQL:       statement,
			MaxRows:   scriptFileRows,
		})
		if err == nil {
			result.Written += ran.RowsAffected
			continue
		}

		result.Failed++
		result.Messages = append(result.Messages,
			transferFailure("Row "+strconv.Itoa(result.Read), err))

		// A cancelled run stops whatever was asked for: Continue on error is about statements the
		// *server* refused, and carrying on would ignore the reader's own Stop.
		if !input.ContinueOnError || ctx.Err() != nil {
			result.Messages = append(result.Messages, transferMessage("Stopped"))
			return false
		}
	}
	return true
}

// plan is the import as the driver takes it: the mode, the columns being written, and which of them
// find a stored row. It is where a wizard that has not been filled in is refused — a mode that needs
// a key and was given none would otherwise reach every row in the table.
func (in ImportInput) plan() (domain.ImportMode, driver.RowWrite, error) {
	if in.Table == "" {
		return "", driver.RowWrite{}, domain.NameError{What: "table"}
	}

	mode := domain.ImportMode(in.Mode)
	write, ok := writeModes[mode]
	if !ok {
		return "", driver.RowWrite{}, domain.ValidationError{
			Field:   "mode",
			Message: "is not supported",
		}
	}

	columns, keys := []string{}, []string{}
	for _, mapping := range in.Mappings {
		if mapping.Source == "" || mapping.Target == "" {
			continue
		}
		columns = append(columns, mapping.Target)
		if mapping.Key {
			keys = append(keys, mapping.Target)
		}
	}

	if len(columns) == 0 {
		return "", driver.RowWrite{}, domain.ValidationError{
			Field:   "mapping",
			Message: "matches no column",
		}
	}
	if mode.NeedsKey() && len(keys) == 0 {
		return "", driver.RowWrite{}, domain.ValidationError{
			Field:   "key",
			Message: "is required by this import mode",
		}
	}

	batch := in.Batch
	if batch <= 0 {
		batch = defaultImportBatch
	}

	return mode, driver.RowWrite{
		Mode:    write,
		Columns: columns,
		Keys:    keys,
		Batch:   batch,
	}, nil
}

// writeModes is the wizard's mode against the driver's. Copy is Append with the table emptied
// first, which is why the two share a write and differ only in what RunImport does before it.
var writeModes = map[domain.ImportMode]driver.RowWriteMode{
	domain.ImportAppend: driver.WriteAppend,
	domain.ImportCopy:   driver.WriteAppend,
	domain.ImportUpdate: driver.WriteUpdate,
	domain.ImportUpsert: driver.WriteUpsert,
	domain.ImportSkip:   driver.WriteSkip,
	domain.ImportDelete: driver.WriteDelete,
}

// take is the head of a preview as the rows a statement would be built from, for ReviewImport.
func (in ImportInput) take(preview ImportPreview, count int) [][]*string {
	sources := index(preview.Columns)

	rows := make([][]*string, 0, count)
	for _, row := range preview.Rows[:count] {
		rows = append(rows, mapped(row, sources, in.Mappings))
	}
	return rows
}

// mapped is one row of the file as one row of the table: the file's fields put under the columns
// the reader pointed them at, in the order `plan` collected those columns in. A mapping naming a
// field the file has not got writes NULL rather than failing — the file is what it is, and the
// mapping step is where a reader sees which fields it really carries.
func mapped(row []*string, sources map[string]int, mappings []FieldMapping) []*string {
	values := make([]*string, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.Source == "" || mapping.Target == "" {
			continue
		}
		if at, known := sources[mapping.Source]; known && at < len(row) {
			values = append(values, row[at])
			continue
		}
		values = append(values, nil)
	}
	return values
}

func index(names []string) map[string]int {
	at := make(map[string]int, len(names))
	for position, name := range names {
		at[name] = position
	}
	return at
}
