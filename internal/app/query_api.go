package app

import (
	"errors"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/session"
)

const statementTimeout = 60 * time.Second

type QueryAPI struct {
	apiContext

	sessions *session.Manager
	runs     *runRegistry
}

func NewQueryAPI(sessions *session.Manager) *QueryAPI {
	return &QueryAPI{
		apiContext: newAPIContext(),
		sessions:   sessions,
		runs:       newRunRegistry(),
	}
}

// Run executes whatever the editor holds and blocks until it finishes, fails, or is cancelled.
// The text is cut into the statements it carries and they are run in order, stopping at the
// first the server rejects.
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

	script := ScriptResult{Statements: make([]StatementOutcome, 0, len(statements))}

	for _, statement := range statements {
		query := input.toDriver()
		query.SQL = statement

		started := time.Now()
		result, err := a.sessions.Query(ctx, sessionID, query)
		elapsed := time.Since(started)

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
	outcome := StatementOutcome{SQL: sql, ElapsedMs: elapsed.Milliseconds()}

	var wire wireError
	if errors.As(wrapError(err), &wire) {
		outcome.ErrorCode = string(wire.Code)
		outcome.Error = wire.Message
		return outcome
	}

	outcome.ErrorCode = string(CodeInternal)
	outcome.Error = err.Error()
	return outcome
}

// Cancel stops the run with this id. Unknown ids are ignored: the query may have
// finished between the click and this call.
func (a *QueryAPI) Cancel(runID string) {
	a.runs.cancel(runID)
}
