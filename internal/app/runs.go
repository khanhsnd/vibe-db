package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
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

// stopped names a failure that is **this app's clock** rather than the server's silence, and it is
// the one seam that does so: a deadline set here reaches the driver as an ordinary cancelled
// context, so what comes back is whatever that engine calls a cut-off statement — a timeout on
// four of them, and a *cancellation* on PostgreSQL, which uses one code for both. Left alone, the
// first blames a server that is answering perfectly well and the second blames a reader who never
// pressed Stop.
//
// It reclassifies only where **our** context is what expired and the error is one of those two, so
// a server's own `statement_timeout`, a reader's Stop, and a syntax error that happened to land at
// the same moment all keep their own answer. Order matters: a Stop cancels the context without a
// deadline, so `ctx.Err()` says which of the two happened.
func stopped(ctx context.Context, err error) error {
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, domain.ErrConnectionTimeout) || errors.Is(err, domain.ErrQueryCancelled) {
		return domain.ErrRunDeadline
	}
	return err
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
