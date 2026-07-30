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
