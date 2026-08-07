package app

import (
	"context"
	"errors"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/history"
	"github.com/khanhsnd/vibe-db/internal/session"
)

const statementTimeout = 60 * time.Second

type QueryAPI struct {
	apiContext

	sessions *session.Manager
	history  history.Repository
	log      *history.Log
	runs     *runRegistry
}

func NewQueryAPI(sessions *session.Manager, kept history.Repository, log *history.Log) *QueryAPI {
	return &QueryAPI{
		apiContext: newAPIContext(),
		sessions:   sessions,
		history:    kept,
		log:        log,
		runs:       newRunRegistry(),
	}
}

// Run executes whatever the editor holds and blocks until it finishes, fails, or is cancelled.
// The text is cut into the statements it carries and they are run in order, stopping at the
// first the server rejects. Whatever becomes of it, the text is kept in the history log — see
// remember.
//
// How a rejection is reported depends on how many statements there were, and deliberately so:
//
//   - **One statement fails as a failure**, exactly as it always has, so the output panel shows
//     the server's own wording where the reader is already looking.
//   - **A script reports it as a message**, because the statements before it did run and saying
//     only "failed" would hide that. The rows shown are the last result set the script produced.
func (a *QueryAPI) Run(sessionID string, input QueryInput) (ScriptResult, error) {
	statements, err := a.sessions.Split(sessionID, input.SQL)
	if err != nil {
		return ScriptResult{}, wrapError(err)
	}
	if len(statements) == 0 {
		return ScriptResult{}, wrapError(domain.ErrEmptyStatement)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, statementTimeout)
	defer release()

	started := time.Now()
	script, failure := a.runScript(ctx, sessionID, input, statements)
	a.remember(sessionID, input, script, failure, time.Since(started))

	return script, failure
}

func (a *QueryAPI) runScript(
	ctx context.Context,
	sessionID string,
	input QueryInput,
	statements []string,
) (ScriptResult, error) {
	script := ScriptResult{Statements: make([]StatementOutcome, 0, len(statements))}
	noting := newStatementLog(a.log, a.sessions, sessionID, input.Namespace)

	for _, statement := range statements {
		query := input.toDriver()
		query.SQL = statement

		started := time.Now()
		result, err := a.sessions.Query(ctx, sessionID, query)
		elapsed := time.Since(started)
		noting.note(statement, elapsed, err)

		if err != nil {
			if len(statements) == 1 {
				return ScriptResult{}, wrapError(err)
			}
			script.Statements = append(script.Statements, failedStatement(statement, elapsed, err))
			return script, nil
		}

		script.Statements = append(script.Statements, ranStatement(statement, elapsed, result))
		// The rows shown are the last set a statement produced, so a script that ends in a
		// SELECT shows it and one that ends in an UPDATE keeps the last set it did return.
		if len(result.Columns) > 0 || len(statements) == 1 {
			held := script.Statements
			script = ScriptResult{QueryResult: toQueryResultDTO(result, elapsed), Statements: held}
		}
	}

	return script, nil
}

func ranStatement(sql string, elapsed time.Duration, result driver.QueryResult) StatementOutcome {
	return StatementOutcome{
		SQL:          sql,
		Rows:         len(result.Rows),
		RowsAffected: result.RowsAffected,
		ElapsedMs:    elapsed.Milliseconds(),
	}
}

// failedStatement carries the same code and wording a thrown failure would have, so a script's
// messages read exactly as the output panel does for one statement.
func failedStatement(sql string, elapsed time.Duration, err error) StatementOutcome {
	code, message := wireFields(err)
	return StatementOutcome{
		SQL:       sql,
		ElapsedMs: elapsed.Milliseconds(),
		ErrorCode: code,
		Error:     message,
	}
}

// wireFields is the code and the wording a failure would cross the wire with, for the two places
// that report one as data rather than by returning it.
func wireFields(err error) (string, string) {
	var wire wireError
	if errors.As(wrapError(err), &wire) {
		return string(wire.Code), wire.Message
	}
	return string(CodeInternal), err.Error()
}

// Review reads what a script is going to do, and what is already wrong with it, without sending
// any of it to the server. It answers two questions the editor asks continuously: which statements
// have to be confirmed before Execute may run them, and where the lexer can already prove
// something is wrong — an unclosed quote, a bracket with no partner.
//
// It is deliberately not a parser. Whether a column exists or a clause is in the right order is
// the server's answer to give, and a client that guessed would be wrong often enough that a reader
// learned to ignore it — see sqlkit.Review.
func (a *QueryAPI) Review(sessionID, sql string) (ScriptReview, error) {
	review, environment, err := a.sessions.Review(sessionID, sql)
	if err != nil {
		return ScriptReview{}, wrapError(err)
	}
	return toReviewDTO(review, environment), nil
}

// Format lays a script out and answers with it. Nothing a server would notice changes: only the
// whitespace between tokens and the case of unquoted keywords, since a string, a quoted name and a
// comment are single tokens written back byte for byte. A script the lexer cannot make sense of
// comes back exactly as it went in — Review is what says why.
func (a *QueryAPI) Format(sessionID, sql string) (string, error) {
	formatted, err := a.sessions.Format(sessionID, sql)
	if err != nil {
		return "", wrapError(err)
	}
	return formatted, nil
}

// ListHistory answers with the statements this app has run, newest first — one entry per statement
// of a namespace, however many times it has been run. It is every connection's, because a log
// filtered by the pane showing it can still say what a connection that has gone once did.
func (a *QueryAPI) ListHistory() ([]HistoryEntry, error) {
	entries, err := a.history.List()
	if err != nil {
		return nil, wrapError(err)
	}
	return toHistoryDTOs(entries), nil
}

// ClearHistory forgets every statement. There is no way to forget one: a log is what it is, and a
// reader who wants it gone wants it gone.
func (a *QueryAPI) ClearHistory() error {
	return wrapError(a.history.Clear())
}

// ListLog answers with the statements the app has run since it started, oldest first, and only the
// ones numbered past `after` — which is how the viewer asks for what it has not already drawn
// rather than being handed the whole log every second. Nothing here can fail: the log is memory,
// and a bound method that cannot fail does not pretend it can.
func (a *QueryAPI) ListLog(after int64) []LogEntry {
	return toLogDTOs(a.log.Since(after))
}

// ClearLog empties the viewer's log. It forgets nothing a server did and nothing the *history*
// holds — those are two different logs, kept for two different questions; see history.Log.
func (a *QueryAPI) ClearLog() {
	a.log.Clear()
}

// statementLog is where a run's statements are noted, and who ran them. The connection is read
// once for the whole run rather than once a statement: a session's name and engine cannot change
// while its statements do.
type statementLog struct {
	log *history.Log
	// who is every field of an entry that is the run's rather than the statement's.
	who domain.LogEntry
}

func newStatementLog(
	log *history.Log,
	sessions *session.Manager,
	sessionID string,
	ns Namespace,
) statementLog {
	who := domain.LogEntry{Database: ns.Database, Schema: ns.Schema}
	// A session that has gone leaves the line unnamed rather than unwritten. What ran is the fact
	// the log exists for, and it is not worth losing to a connection that closed underneath it.
	if opened, err := sessions.Get(sessionID); err == nil {
		who.Connection = opened.Profile.Name
		who.Engine = opened.Profile.Engine
	}
	return statementLog{log: log, who: who}
}

// note keeps one statement whatever became of it, a statement the server refused being the one a
// reader watching a log is most often looking for.
func (s statementLog) note(sql string, elapsed time.Duration, err error) {
	entry := s.who
	entry.RanAt = time.Now()
	entry.SQL = sql
	entry.ElapsedMs = elapsed.Milliseconds()
	if err != nil {
		_, entry.Error = wireFields(err)
	}
	s.log.Note(entry)
}

// remember keeps the text the reader executed whatever became of it, a rejected statement being
// among the likeliest to be wanted back. The outcome recorded is the run's as a whole: the rows the
// grid ended up with, and the wording of the statement it stopped at.
//
// A log that cannot be written is dropped rather than reported. It is not a failure the reader can
// act on, and it is certainly not one the query they just ran should be reported as; ListHistory is
// where a broken log says so.
func (a *QueryAPI) remember(
	sessionID string,
	input QueryInput,
	script ScriptResult,
	failure error,
	elapsed time.Duration,
) {
	opened, err := a.sessions.Get(sessionID)
	if err != nil {
		return
	}

	entry := domain.HistoryEntry{
		ProfileID:    opened.Profile.ID,
		Database:     input.Database,
		Schema:       input.Schema,
		SQL:          input.SQL,
		Rows:         len(script.Rows),
		RowsAffected: script.RowsAffected,
		ElapsedMs:    elapsed.Milliseconds(),
		RanAt:        time.Now(),
	}

	if failure != nil {
		entry.ErrorCode, entry.Error = wireFields(failure)
	} else if stopped, ok := script.stopped(); ok {
		entry.ErrorCode, entry.Error = stopped.ErrorCode, stopped.Error
	}

	_ = a.history.Append(entry)
}

// stopped is the statement a script stopped at, if it stopped at one.
func (s ScriptResult) stopped() (StatementOutcome, bool) {
	for _, statement := range s.Statements {
		if statement.Error != "" {
			return statement, true
		}
	}
	return StatementOutcome{}, false
}

// Cancel stops the run with this id. Unknown ids are ignored: the query may have
// finished between the click and this call.
func (a *QueryAPI) Cancel(runID string) {
	a.runs.cancel(runID)
}
