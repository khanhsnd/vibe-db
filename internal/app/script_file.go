package app

import (
	"context"
	"os"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/session"
	"github.com/khanhsnd/vibe-db/internal/transfer"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// scriptFileTimeout is what a whole file is given, rather than the minute one statement gets: a
// dump of a few hundred thousand rows is a long job that is going fine. Stopping it is the run
// registry's — the reader's own Stop — which is the honest bound on something this size.
const scriptFileTimeout = 2 * time.Hour

// scriptFileRows caps what a statement in a file may hand back. A file run shows no rows at all —
// it reports what each statement *did* — so reading a SELECT's whole result would be carrying
// megabytes across the bridge for a grid that is not there.
const scriptFileRows = 1

// ScriptFileInput is a file on this machine and where to run it. The path is the reader's own —
// see ChooseScriptFile, which is how one is picked — and nothing but the statements inside it
// ever reaches a server.
type ScriptFileInput struct {
	Namespace
	// RunID is minted by the caller so a file already running can be stopped — Cancel takes it.
	RunID    string `json:"runId"`
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
	// ContinueOnError runs the rest of the file after a statement the server rejects. Off, the
	// run stops where it stopped, which is what a file whose statements depend on each other wants.
	ContinueOnError bool `json:"continueOnError"`
}

// ScriptFileResult is what a file did, one entry per statement in the order they ran. Rows are
// deliberately not among them — see scriptFileRows.
type ScriptFileResult struct {
	Statements []StatementOutcome `json:"statements"`
	Failed     int                `json:"failed"`
	ElapsedMs  int64              `json:"elapsedMs"`
}

// ChooseScriptFile raises the host's own file picker and answers with the path it was given. An
// empty path is a dismissed dialog rather than a failure, exactly as ExportRows reads.
func (a *QueryAPI) ChooseScriptFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Execute SQL file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL and JavaScript files", Pattern: "*.sql;*.js;*.txt"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// RunFile runs every statement of a file against this session, in the order the file holds them.
//
// It is Run's sibling and differs in exactly two ways, both of which follow from the file being a
// file rather than a buffer: **no rows are kept**, because there is no grid to put them in, and a
// rejected statement need not end the run — see ContinueOnError. Where a statement *ends* is still
// the driver's answer, not this file's: the text goes through the same Split the editor's does.
//
// Nothing here reaches the history log. A log holds statements a reader can put back in an editor,
// and a file of ten thousand of them is a file, which is where it already is.
func (a *QueryAPI) RunFile(sessionID string, input ScriptFileInput) (ScriptFileResult, error) {
	text, err := readScriptFile(input.Path, input.Encoding)
	if err != nil {
		return ScriptFileResult{}, wrapError(err)
	}

	statements, err := a.sessions.Split(sessionID, text)
	if err != nil {
		return ScriptFileResult{}, wrapError(err)
	}
	if len(statements) == 0 {
		return ScriptFileResult{}, wrapError(domain.ErrEmptyStatement)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, scriptFileTimeout)
	defer release()

	ns := input.Namespace.toDriver()
	return runStatements(ctx, a.sessions, sessionID, ns, statements, input.ContinueOnError), nil
}

// runStatements runs each statement against one session in the order it was given them, and answers
// with what every one of them did.
//
// It is shared because a file run and a restore are the same job under two names: a list of
// statements, run one after another, stopping where the reader asked it to. Neither of them spells a
// statement and neither reads a row back — see scriptFileRows.
func runStatements(
	ctx context.Context,
	sessions *session.Manager,
	sessionID string,
	ns driver.Namespace,
	statements []string,
	continueOnError bool,
) ScriptFileResult {
	started := time.Now()
	result := ScriptFileResult{Statements: make([]StatementOutcome, 0, len(statements))}

	for _, statement := range statements {
		query := driver.Query{Namespace: ns, SQL: statement, MaxRows: scriptFileRows}

		at := time.Now()
		ran, err := sessions.Query(ctx, sessionID, query)
		elapsed := time.Since(at)

		if err == nil {
			// How many rows it *changed* is the whole of what a file run reports, and it is the
			// only number here the cap above cannot make untrue.
			result.Statements = append(result.Statements, StatementOutcome{
				SQL:          statement,
				RowsAffected: ran.RowsAffected,
				ElapsedMs:    elapsed.Milliseconds(),
			})
			continue
		}

		result.Statements = append(result.Statements, failedStatement(statement, elapsed, err))
		result.Failed++

		// A cancelled run stops whatever was asked for: Continue on error is about statements the
		// *server* refused, and carrying on would ignore the reader's own Stop.
		if !continueOnError || ctx.Err() != nil {
			break
		}
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result
}

// readScriptFile reads the file as the named encoding. A file that cannot be opened and one whose
// bytes are not that encoding fail the same way and for the same reason: the reader picked the
// wrong file or the wrong encoding, and both are answered in the dialog they picked them in.
//
// The encoding list is `transfer.Encodings` and not one of its own. Every file this app opens asks
// the same question — a script to run, a CSV to import, a workbook to write — and one list is what
// keeps the answer the same in all three places.
func readScriptFile(path, name string) (string, error) {
	if path == "" {
		return "", domain.ValidationError{Field: "path", Message: "is required"}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", domain.FileError{Err: err}
	}
	return transfer.Decode(raw, name)
}
