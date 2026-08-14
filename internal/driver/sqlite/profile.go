package sqlite

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A SQLite database is a **file**, and a file keeps no record of what was asked of it. There is no
// server counting how long a statement took, nowhere it would write such a count, and no setting
// that would turn the counting on — a statement runs inside this program's own process and is over.
//
// So every one of these is `domain.Unsupported` naming it, for the reason the whole of `monitor.go`
// in this adapter is. What a reader wants here is the one thing this engine does have: `EXPLAIN
// QUERY PLAN` on a statement they type, which is the Explain button on a query tab.
//
// The frontend never asks. `EngineInfo.hasProfiler` is false for this engine, so no `Slow Queries`
// pane is drawn.

const noRecording = "a slow query log: a SQLite database is a file, and nothing records what it ran"

func (*session) SlowQueries(
	context.Context,
	driver.Namespace,
	int,
) (driver.SlowQueries, error) {
	return driver.SlowQueries{}, domain.Unsupported(noRecording)
}

func (*session) ProfileConfig(
	context.Context,
	driver.Namespace,
) (driver.ProfileConfig, error) {
	return driver.ProfileConfig{}, domain.Unsupported(noRecording)
}

// ProfileStatements spells nothing, which is what an empty preview means everywhere in this port.
func (*session) ProfileStatements([]driver.ProfileChange) []string {
	return nil
}

func (*session) ApplyProfileChanges(
	context.Context,
	driver.Namespace,
	[]driver.ProfileChange,
) error {
	return domain.Unsupported(noRecording)
}
