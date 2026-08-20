package objectstore

import (
	"context"
	"errors"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Retryable marks a failure that is worth asking again about: a service that answered 500, one that
// asked to be slowed down, a connection that dropped in the middle of a body. It is the adapter's
// to raise — only the protocol knows which of its answers mean "not now" — and this package's to
// act on.
//
// It is a wrapper and not a sentinel so the reason survives: a transfer that gave up after five
// tries still reports what the service actually said, which is the only useful sentence about it.
type Retryable struct {
	Err error
}

func (e Retryable) Error() string {
	return e.Err.Error()
}

func (e Retryable) Unwrap() error {
	return e.Err
}

// Temporary says the failure is one of the two kinds worth another attempt: the adapter said so, or
// it is the network itself — a host that could not be reached and a request that timed out are both
// ordinary halfway through a twenty-gigabyte download over somebody's VPN.
func Temporary(err error) bool {
	var retryable Retryable
	if errors.As(err, &retryable) {
		return true
	}
	return errors.Is(err, domain.ErrHostUnreachable) || errors.Is(err, domain.ErrConnectionTimeout)
}

const (
	// Attempts is how many times one request is made before a transfer gives up on it. Five with
	// the backoff below is about six seconds of patience, which covers a throttle and a dropped
	// connection and does not cover a service that is down.
	Attempts = 5
	// backoffStep is the first wait, doubled each time: 200ms, 400ms, 800ms, 1.6s.
	backoffStep = 200 * time.Millisecond
)

// Retry makes the request again while it is worth doing so, and answers with the **last** failure
// rather than the first: what the reader wants to know is why it finally gave up.
//
// A cancelled context ends it at once and is never retried — a reader who pressed Stop is not
// waiting to be tried harder for.
func Retry(ctx context.Context, attempt func() error) error {
	wait := backoffStep
	var err error

	for try := 0; try < Attempts; try++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		err = attempt()
		if err == nil || !Temporary(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
	return err
}
