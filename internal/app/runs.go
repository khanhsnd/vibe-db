package app

import (
	"context"
	"sync"
	"time"
)

// runRegistry keeps the cancel func of every in-flight query so a second bound
// call can stop it. The frontend mints the run id, because RunQuery only returns
// once the statement is done and Stop has to work before that.
type runRegistry struct {
	mu   sync.Mutex
	runs map[string]context.CancelFunc
}

func newRunRegistry() *runRegistry {
	return &runRegistry{runs: make(map[string]context.CancelFunc)}
}

// begin derives a cancellable context for id. The returned release must run when
// the query finishes, whatever the outcome.
func (r *runRegistry) begin(
	parent context.Context,
	id string,
	timeout time.Duration,
) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	if id == "" {
		return ctx, cancel
	}

	r.mu.Lock()
	r.runs[id] = cancel
	r.mu.Unlock()

	return ctx, func() {
		r.mu.Lock()
		delete(r.runs, id)
		r.mu.Unlock()
		cancel()
	}
}

// cancel stops the run if it is still going, and is a no-op otherwise.
func (r *runRegistry) cancel(id string) {
	r.mu.Lock()
	cancel, running := r.runs[id]
	delete(r.runs, id)
	r.mu.Unlock()

	if running {
		cancel()
	}
}
