package app

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/transfer"
)

// exportTimeout is what a whole export is given, and it is a dump's two hours for the same reason:
// a hundred tables of a hundred thousand rows is a long job that is going fine.
const exportTimeout = 2 * time.Hour

// ExportJobInput is a whole export: which tables, in what format, and where the files go.
//
// **One file per table**, named after the table. A wizard that also asked for a file *name* would be
// asking a question with no answer for the six-table case, and every format here holds one table —
// even a workbook, which could hold six sheets and does not, because a streamed export does not know
// the six before it writes the first.
type ExportJobInput struct {
	Namespace
	// RunID is minted by the caller so a running export can be stopped — Cancel takes it.
	RunID   string          `json:"runId"`
	Format  string          `json:"format"`
	Tables  []string        `json:"tables"`
	Folder  string          `json:"folder"`
	Options TransferOptions `json:"options"`
	// MaxRows caps each table; zero is every row of it, which is what an export usually means.
	MaxRows int `json:"maxRows"`
}

// ExportedFile is what one table came to. `error` is the server's own wording and is empty for a
// table that went out cleanly, so the log reads as the file run's does.
type ExportedFile struct {
	Table string `json:"table"`
	Path  string `json:"path"`
	Rows  int    `json:"rows"`
	Error string `json:"error"`
}

type ExportJobResult struct {
	Files     []ExportedFile    `json:"files"`
	Rows      int               `json:"rows"`
	Failed    int               `json:"failed"`
	ElapsedMs int64             `json:"elapsedMs"`
	Messages  []TransferMessage `json:"messages"`
}

// RunExport writes each table into a file of its own in the folder the reader picked.
//
// The rows are **streamed, not paged**, exactly as a dump's are: a page counts the table again for
// every page it cuts, and an export wants each row once in one pass — so each batch is written to
// the file as it arrives and a table of a million rows costs one batch of memory.
//
// A table that fails does not stop the rest. An export changes nothing on the server, so there is
// nothing for a half-finished one to have left inconsistent — and a reader exporting forty tables
// would rather have thirty-nine and a message than a stop at the seventh.
func (a *ConnectionAPI) RunExport(
	sessionID string,
	input ExportJobInput,
) (ExportJobResult, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return ExportJobResult{}, wrapError(err)
	}
	if err := input.validate(); err != nil {
		return ExportJobResult{}, wrapError(err)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, exportTimeout)
	defer release()

	started := time.Now()
	result := ExportJobResult{
		Files:    []ExportedFile{},
		Messages: []TransferMessage{transferMessage("Writing to " + input.Folder)},
	}

	for _, table := range input.Tables {
		file, err := a.exportTable(ctx, sessionID, schema, input, table)
		if err != nil {
			_, message := wireFields(err)
			result.Files = append(result.Files,
				ExportedFile{Table: table, Path: file.Path, Error: message})
			result.Messages = append(result.Messages, transferFailure(table, err))
			result.Failed++

			// The reader's own Stop ends the export; a table the server refused does not.
			if ctx.Err() != nil {
				result.Messages = append(result.Messages, transferMessage("Stopped"))
				break
			}
			continue
		}

		result.Files = append(result.Files, file)
		result.Rows += file.Rows
		result.Messages = append(result.Messages,
			transferMessage(table+" → "+filepath.Base(file.Path)+" · "+rowsText(file.Rows)))
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

// exportTable writes one table into one file. The file is made **before the first row is read**, so
// a folder this account cannot write to fails while nothing has been read — the order `dump.go`
// opens its file in, and for the same reason.
func (a *ConnectionAPI) exportTable(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	input ExportJobInput,
	table string,
) (ExportedFile, error) {
	ns := input.Namespace.toDriver()

	structure, err := schema.TableStructure(ctx, ns, table)
	if err != nil {
		return ExportedFile{}, err
	}

	columns := make([]string, 0, len(structure.Columns))
	for _, column := range structure.Columns {
		columns = append(columns, column.Name)
	}

	path, err := input.pathFor(table)
	if err != nil {
		return ExportedFile{}, err
	}

	writer, err := transfer.Create(input.Format, path, input.Options.toTransfer())
	if err != nil {
		return ExportedFile{}, err
	}
	if err := writer.Begin(table, columns); err != nil {
		writer.Close()
		return ExportedFile{Path: path}, err
	}

	// A failure inside the stream cannot be returned from it, so it is kept and answered with once
	// the read is over — and no further batch is written after one, which would put rows into the
	// file after the batch that could not be written.
	file := ExportedFile{Table: table, Path: path}
	var failed error

	page := driver.TablePage{
		Namespace: ns,
		Table:     table,
		Limit:     input.MaxRows,
		Stream: &driver.RowStream{
			Rows: func(batch [][]*string) {
				if failed != nil {
					return
				}
				if failed = writer.Rows(batch); failed == nil {
					file.Rows += len(batch)
				}
			},
			Counted: func(int) {},
		},
	}

	if _, err := a.sessions.TableRows(ctx, sessionID, page); err != nil {
		writer.Close()
		return file, err
	}
	if failed != nil {
		writer.Close()
		return file, failed
	}

	// Close is where a format with a footer finally writes it, and where a full volume is finally
	// heard from, so it is the last thing that can fail and is answered with rather than deferred.
	if err := writer.Close(); err != nil {
		return file, err
	}
	return file, nil
}

func (in ExportJobInput) validate() error {
	if in.Folder == "" {
		return domain.ValidationError{Field: "folder", Message: "is required"}
	}
	if len(in.Tables) == 0 {
		return domain.ValidationError{Field: "tables", Message: "is empty"}
	}
	if _, err := transfer.FormatNamed(in.Format); err != nil {
		return err
	}
	return nil
}

// pathFor is where one table's file lands. The table's name is what the file is called, with the
// characters a file name may not hold replaced rather than dropped — so two tables differing only
// in punctuation do not write over each other — and the path is checked to be *inside* the folder
// the reader picked, which a table called `../etc` would otherwise not be.
func (in ExportJobInput) pathFor(table string) (string, error) {
	name := strings.Map(func(char rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, char) || char < 0x20 {
			return '_'
		}
		return char
	}, table)

	format, err := transfer.FormatNamed(in.Format)
	if err != nil {
		return "", err
	}

	path := filepath.Join(in.Folder, name+"."+format.Extension)
	if filepath.Dir(path) != filepath.Clean(in.Folder) {
		return "", domain.ValidationError{Field: "table", Message: "cannot be written as a file name"}
	}
	return path, nil
}

func rowsText(rows int) string {
	if rows == 1 {
		return "1 row"
	}
	return strconv.Itoa(rows) + " rows"
}
