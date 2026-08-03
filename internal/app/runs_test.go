package app

import (
	"context"
	"errors"
	"testing"
	"time"
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
