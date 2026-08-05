package app

import (
	"fmt"
	"os"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// scriptFileTimeout is what a whole file is given, rather than the minute one statement gets: a
// dump of a few hundred thousand rows is a long job that is going fine. Stopping it is the run
// registry's — the reader's own Stop — which is the honest bound on something this size.
const scriptFileTimeout = 2 * time.Hour

// scriptFileRows caps what a statement in a file may hand back. A file run shows no rows at all —
// it reports what each statement *did* — so reading a SELECT's whole result would be carrying
// megabytes across the bridge for a grid that is not there.
const scriptFileRows = 1

// scriptEncoding is one way of reading a file's bytes as text. The list is closed and the frontend
// mirrors it, exactly as the engine and TLS lists are mirrored: a name that is not here is refused
// rather than guessed at.
type scriptEncoding struct {
	name    string
	label   string
	decoder encoding.Encoding
}

// The UTF-8 entry is UTF8BOM rather than UTF8: its decoder strips a leading byte order mark, and a
// file exported by a Windows tool nearly always carries one — left in place it would become part
// of the first statement's first keyword.
var scriptEncodings = []scriptEncoding{
	{"utf-8", "UTF-8", unicode.UTF8BOM},
	{"utf-16le", "UTF-16 LE", unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)},
	{"utf-16be", "UTF-16 BE", unicode.UTF16(unicode.BigEndian, unicode.UseBOM)},
	{"windows-1252", "Windows-1252 (Western European)", charmap.Windows1252},
	{"windows-1258", "Windows-1258 (Vietnamese)", charmap.Windows1258},
	{"iso-8859-1", "ISO-8859-1 (Latin 1)", charmap.ISO8859_1},
}

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

	started := time.Now()
	result := ScriptFileResult{Statements: make([]StatementOutcome, 0, len(statements))}

	for _, statement := range statements {
		query := driver.Query{
			Namespace: input.Namespace.toDriver(),
			SQL:       statement,
			MaxRows:   scriptFileRows,
		}

		at := time.Now()
		ran, err := a.sessions.Query(ctx, sessionID, query)
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
		if !input.ContinueOnError || ctx.Err() != nil {
			break
		}
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

// readScriptFile reads the file as the named encoding. A file that cannot be opened and one whose
// bytes are not that encoding fail the same way and for the same reason: the reader picked the
// wrong file or the wrong encoding, and both are answered in the dialog they picked them in.
func readScriptFile(path, name string) (string, error) {
	if path == "" {
		return "", domain.ValidationError{Field: "path", Message: "is required"}
	}

	chosen, err := scriptEncodingNamed(name)
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrFileUnreadable, err)
	}

	decoded, err := chosen.decoder.NewDecoder().Bytes(raw)
	if err != nil {
		return "", fmt.Errorf("%w: not %s", domain.ErrFileUnreadable, chosen.label)
	}
	return string(decoded), nil
}

func scriptEncodingNamed(name string) (scriptEncoding, error) {
	for _, candidate := range scriptEncodings {
		if candidate.name == name {
			return candidate, nil
		}
	}
	return scriptEncoding{}, domain.ValidationError{Field: "encoding", Message: "is not supported"}
}
