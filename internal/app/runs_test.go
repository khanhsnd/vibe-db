package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestRunRegistryCancelStopsTheRun(t *testing.T) {
	registry := newRunRegistry()

	ctx, release := registry.begin(context.Background(), "run-1", time.Minute)
	defer release()

	registry.cancel("run-1")

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Errorf("ctx.Err() = %v, want context.Canceled", ctx.Err())
	}
}

func TestRunRegistryReleaseForgetsTheRun(t *testing.T) {
	registry := newRunRegistry()

	_, release := registry.begin(context.Background(), "run-1", time.Minute)
	release()

	if len(registry.runs) != 0 {
		t.Errorf("registry holds %d runs after release, want 0", len(registry.runs))
	}
}

func TestRunRegistryCancelIgnoresUnknownRuns(t *testing.T) {
	newRunRegistry().cancel("never-started")
}

func TestRunRegistryBeginAppliesTheTimeout(t *testing.T) {
	registry := newRunRegistry()

	ctx, release := registry.begin(context.Background(), "run-1", time.Nanosecond)
	defer release()

	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Errorf("ctx.Err() = %v, want context.DeadlineExceeded", ctx.Err())
	}
}

// A read of every row is bounded by Stop rather than by a clock, so it must carry no deadline
// for one to expire.
func TestRunRegistryBeginWithoutTimeoutHasNoDeadline(t *testing.T) {
	registry := newRunRegistry()

	ctx, release := registry.begin(context.Background(), "run-1", noDeadline)
	defer release()

	if deadline, ok := ctx.Deadline(); ok {
		t.Errorf("ctx has deadline %v, want none", deadline)
	}

	registry.cancel("run-1")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Errorf("ctx.Err() = %v, want context.Canceled", ctx.Err())
	}
}

func TestRowsTimeout(t *testing.T) {
	if got := rowsTimeout(true); got != noDeadline {
		t.Errorf("rowsTimeout(streamed) = %v, want none", got)
	}
	if got := rowsTimeout(false); got != queryTimeout {
		t.Errorf("rowsTimeout(bounded) = %v, want %v", got, queryTimeout)
	}
}

func TestRunDeadlineIsTheReadersPress(t *testing.T) {
	if got := runDeadline("run-1"); got != noDeadline {
		t.Errorf("runDeadline(%q) = %v, want no deadline: a run with Stop behind it needs no clock",
			"run-1", got)
	}
	if got := runDeadline(""); got != statementTimeout {
		t.Errorf("runDeadline(\"\") = %v, want %v: nothing on this side could stop it",
			got, statementTimeout)
	}
}

func TestStoppedNamesThisAppsClock(t *testing.T) {
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	live, release := context.WithCancel(context.Background())
	defer release()

	pressed, stop := context.WithCancel(context.Background())
	stop()

	sqlError := errors.New("syntax error at or near \"slect\"")

	tests := []struct {
		name string
		ctx  context.Context
		in   error
		want error
	}{
		// Our clock ran out, and the engine reported it as either of the two things engines report.
		{"deadline reported as a timeout", expired, domain.ErrConnectionTimeout, domain.ErrRunDeadline},
		{"deadline reported as a cancel", expired, domain.ErrQueryCancelled, domain.ErrRunDeadline},
		// The server's own limit, or the reader's own press: both keep their own answer.
		{"server timeout with time to spare", live, domain.ErrConnectionTimeout, domain.ErrConnectionTimeout},
		{"reader pressed Stop", pressed, domain.ErrQueryCancelled, domain.ErrQueryCancelled},
		// A failure that has nothing to do with a clock is left alone even past the deadline.
		{"statement refused", expired, sqlError, sqlError},
		{"nothing went wrong", expired, nil, nil},
	}

	for _, test := range tests {
		if got := stopped(test.ctx, test.in); !errors.Is(got, test.want) && got != test.want {
			t.Errorf("%s: stopped(...) = %v, want %v", test.name, got, test.want)
		}
	}
}
