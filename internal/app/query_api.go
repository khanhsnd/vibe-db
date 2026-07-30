package app

import (
	"time"

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

// Run executes one statement and blocks until it finishes, fails, or is cancelled.
func (a *QueryAPI) Run(sessionID string, input QueryInput) (QueryResult, error) {
	ctx, release := a.runs.begin(a.ctx, input.RunID, statementTimeout)
	defer release()

	started := time.Now()
	result, err := a.sessions.Query(ctx, sessionID, input.toDriver())
	elapsed := time.Since(started)

	if err != nil {
		return QueryResult{}, wrapError(err)
	}
	return toQueryResultDTO(result, elapsed), nil
}

// Cancel stops the run with this id. Unknown ids are ignored: the query may have
// finished between the click and this call.
func (a *QueryAPI) Cancel(runID string) {
	a.runs.cancel(runID)
}
