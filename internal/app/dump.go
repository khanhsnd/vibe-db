package app

import (
	"bufio"
	"context"
	"os"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// dumpTimeout is what a whole dump is given, and it is a file run's two hours rather than a read's
// fifteen seconds for the same reason: a dump of a few hundred thousand rows is a long job that is
// going fine, and a clock that cut it off would be the only thing wrong with it.
const dumpTimeout = 2 * time.Hour

// DumpInput is what to write out and how much of it.
type DumpInput struct {
	Namespace
	// Table is the one table to dump, and is empty for a dump of every table the namespace holds.
	Table string `json:"table"`
	// Selections allows one dump file to contain selected tables from multiple namespaces.
	Selections []DumpSelection `json:"selections"`
	// WithData writes the rows after the shape. Off, the file holds the declarations alone — which
	// is the difference between the two items the menu offers.
	WithData bool `json:"withData"`
}

type DumpSelection struct {
	Namespace
	Tables []string `json:"tables"`
	// All asks the server for the complete table list; false with an empty Tables list means none.
	All bool `json:"all"`
}

// dumpWhat is which halves of a table go into a file: what would declare it, and the rows it holds.
// A dump from the menu always writes the first and asks about the second; a backup's Advanced tab is
// where either of them can be left out.
type dumpWhat struct {
	structure bool
	data      bool
}

// dumpFile is what a dump of an engine is a file *of*. It is read off `domain.Engine.Documents`
// rather than off the engine's name: a document store's statements are commands and not SQL, so a
// file of them is a script the shell runs and calling it `.sql` would be a lie about its contents.
type dumpFile struct {
	label     string
	extension string
}

func dumpFileFor(engine domain.Engine) dumpFile {
	if engine.Documents() {
		return dumpFile{label: "Shell script", extension: "js"}
	}
	return dumpFile{label: "SQL script", extension: "sql"}
}

// DumpObjects writes what would make these objects again to a file the reader picks, and answers
// with the path it wrote. An empty path is a dismissed dialog rather than a failure, exactly as
// ExportRows reads.
//
// The shape is asked of the driver — `CreateStatements` — and the rows, when they are asked for, of
// the same read a table tab streams: each batch is written as it arrives, so a dump of a million
// rows costs one batch of memory rather than a table's worth. **The dialog comes first**, before
// anything is read, because a reader who dismisses it should not have waited for a table they were
// not going to keep.
func (a *ConnectionAPI) DumpObjects(sessionID string, input DumpInput) (string, error) {
	session, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	file := dumpFileFor(session.Profile.Engine)
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Dump " + file.label,
		DefaultFilename: input.fileName() + "." + file.extension,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: file.label, Pattern: "*." + file.extension},
		},
	})
	if err != nil {
		return "", wrapError(err)
	}
	if path == "" {
		return "", nil
	}

	ctx, cancel := a.withTimeout(dumpTimeout)
	defer cancel()

	if err := a.writeDump(ctx, sessionID, schema, path, input); err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// writeDump opens the file and puts every table into it in turn. The file is opened before the first
// table is read, so a path this account cannot write to fails while nothing has been read.
func (a *ConnectionAPI) writeDump(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	path string,
	input DumpInput,
) error {
	selections, err := a.dumpSelections(ctx, sessionID, input)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	what := dumpWhat{structure: true, data: input.WithData}

	writer := bufio.NewWriter(out)
	for _, selection := range selections {
		ns := selection.Namespace.toDriver()
		for _, table := range selection.Tables {
			if _, err := a.dumpTable(ctx, sessionID, schema, writer, ns, table, what); err != nil {
				return err
			}
		}
	}

	if err := writer.Flush(); err != nil {
		return err
	}
	// A buffered write reports nothing until it is flushed and a flush reports nothing about the
	// disk, so the close is where a full volume is finally heard from — which is why it is checked
	// here as well as deferred.
	return out.Close()
}

func (a *ConnectionAPI) dumpSelections(ctx context.Context, sessionID string, input DumpInput) ([]DumpSelection, error) {
	if len(input.Selections) > 0 {
		out := make([]DumpSelection, 0, len(input.Selections))
		for _, selection := range input.Selections {
			tables := selection.Tables
			if selection.All {
				var err error
				tables, err = a.namespaceTables(ctx, sessionID, selection.Namespace)
				if err != nil {
					return nil, err
				}
			}
			out = append(out, DumpSelection{Namespace: selection.Namespace, Tables: tables})
		}
		return out, nil
	}
	tables, err := a.dumpTables(ctx, sessionID, input)
	if err != nil {
		return nil, err
	}
	return []DumpSelection{{Namespace: input.Namespace, Tables: tables}}, nil
}

// dumpTables is what the dump covers: the one table named, or every table the namespace holds. A
// view is not among them — it holds no rows of its own and its declaration is not a table's — and
// nor is a routine or a trigger, which is why the menu offers this on a table and on the node
// owning tables. All three are edited as [their own definition](docs/features/definitions.md).
func (a *ConnectionAPI) dumpTables(
	ctx context.Context,
	sessionID string,
	input DumpInput,
) ([]string, error) {
	if input.Table != "" {
		return []string{input.Table}, nil
	}
	return a.namespaceTables(ctx, sessionID, input.Namespace)
}

// namespaceTables is every table a namespace holds, which is what a dump of one covers and what a
// backup covers when the reader has narrowed nothing.
func (a *ConnectionAPI) namespaceTables(
	ctx context.Context,
	sessionID string,
	ns Namespace,
) ([]string, error) {
	objects, err := a.sessions.Objects(ctx, sessionID, ns.toDriver())
	if err != nil {
		return nil, err
	}
	return objects.Tables, nil
}

// dumpTable writes one table — its shape, then its rows, as far as `what` asks for — and answers
// with how many rows it wrote, which is what a backup's log reports per table.
//
// It is the one place a table becomes a file of statements: a dump from the tree and a backup both
// come through here, so a backup restores by exactly the same statements a dump re-runs.
//
// The rows are **streamed rather than paged**. A page counts the table again for every page it cuts,
// and a dump wants each row once in one pass — so the read is unbounded and each batch is written on
// the goroutine doing the scan, which is what keeps the file one batch of memory wide however many
// rows the table holds.
func (a *ConnectionAPI) dumpTable(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	out *bufio.Writer,
	ns driver.Namespace,
	table string,
	what dumpWhat,
) (int, error) {
	if what.structure {
		statements, err := schema.CreateStatements(ctx, ns, table)
		if err != nil {
			return 0, err
		}
		if err := writeStatements(out, statements); err != nil {
			return 0, err
		}
	}
	if !what.data {
		return 0, endTable(out)
	}

	// The column names have to be known before the first batch arrives, so they are read from the
	// shape rather than from the result the read ends with. It is the same read the rows themselves
	// are ordered by on all three engines, so a value cannot land under another column's name.
	structure, err := schema.TableStructure(ctx, ns, table)
	if err != nil {
		return 0, err
	}
	columns := make([]string, 0, len(structure.Columns))
	for _, column := range structure.Columns {
		columns = append(columns, column.Name)
	}

	// A failure inside the stream cannot be returned from it, so it is kept and answered with once
	// the read is over — and no further batch is written after one, which would put statements into
	// the file after the one that could not be written.
	var failed error
	rows := 0
	page := driver.TablePage{
		Namespace: ns,
		Table:     table,
		Stream: &driver.RowStream{
			Rows: func(batch [][]*string) {
				if failed != nil {
					return
				}
				failed = writeStatements(out, schema.RowStatements(ns, table, driver.RowWrite{
					Mode:    driver.WriteAppend,
					Columns: columns,
					Rows:    batch,
					Batch:   1,
				}))
				if failed == nil {
					rows += len(batch)
				}
			},
			Counted: func(int) {},
		},
	}

	if _, err := a.sessions.TableRows(ctx, sessionID, page); err != nil {
		return rows, err
	}
	if failed != nil {
		return rows, failed
	}
	return rows, endTable(out)
}

// writeStatements puts each statement on its own lines and terminates it with a semicolon, which is
// what both SQL engines and the shell read back — the same terminator ExportRows writes.
func writeStatements(out *bufio.Writer, statements []string) error {
	for _, statement := range statements {
		if _, err := out.WriteString(statement + ";\n"); err != nil {
			return err
		}
	}
	return nil
}

// endTable separates one table from the next. It is a blank line per *table* rather than per
// statement, so a dump of a million rows is a million lines and not two.
func endTable(out *bufio.Writer) error {
	_, err := out.WriteString("\n")
	return err
}

// fileName is what the save dialog suggests: the table dumped, or the namespace whose tables are —
// which is the schema on an engine that has them and the database on one that does not.
func (in DumpInput) fileName() string {
	for _, name := range []string{in.Table, in.Schema, in.Database} {
		if name != "" {
			return name
		}
	}
	return "dump"
}
