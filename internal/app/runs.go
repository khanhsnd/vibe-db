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
//
// A timeout of zero or less gives the run no deadline at all, which is what a read of every row
// needs: how long a million rows take is not knowable ahead of time, so stopping it is the only
// honest bound.
func (r *runRegistry) begin(
	parent context.Context,
	id string,
	timeout time.Duration,
) (context.Context, func()) {
	ctx, cancel := bound(parent, timeout)
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

// bound is context.WithTimeout, except that a non-positive timeout means no deadline.
func bound(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
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
