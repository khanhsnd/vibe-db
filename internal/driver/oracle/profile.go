package oracle

import (
	"context"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What an Oracle instance has already run **slowly**. Nothing here spells a change, and on this
// engine there is none to spell: `sqlkit.Profile` answers `domain.Unsupported` for all three, the
// dialect's profiling flags being blank on purpose — see the note beside them.
//
// **`v$sqlarea` is one row per statement in the shared pool**, with what every run of it cost, and
// it is there whether or not anybody asked. So there is no switch and no threshold, exactly as on
// SQL Server; and there is no reset either, the only thing that would empty it being a flush of the
// whole instance's parsed statements.
//
// **What it keeps carries bind variables** — `:1`, or the names a program gave them — because that
// is how Oracle shares one cursor between a thousand runs. A statement copied out of this pane is
// therefore one the reader supplies the binds for before any server will take it.

// slowQueryLimit is what a request with no limit of its own reads.
const slowQueryLimit = 200

// SlowQueries reads the shared pool, worst **in total** first. `elapsed_time` is in microseconds,
// which is why it is divided on the way out: what the pane draws is milliseconds on every engine, so
// two servers can be read side by side without the reader converting in their head.
//
// Reading it wants `SELECT` on `v_$sqlarea`, which not every account has. An account without it
// meets the server's own refusal, naming the view — the arrangement every other privileged read in
// this adapter has.
func (s *session) SlowQueries(
	ctx context.Context,
	_ driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	query := `SELECT * FROM (
		  SELECT ROUND(elapsed_time / 1000, 2) AS "Total ms",
		         executions AS "Calls",
		         ROUND(elapsed_time / 1000 / GREATEST(executions, 1), 2) AS "Mean ms",
		         disk_reads AS "Disk Reads",
		         buffer_gets AS "Buffer Gets",
		         parsing_schema_name AS "Schema",
		         sql_fulltext AS "Statement"
		    FROM v$sqlarea
		   ORDER BY elapsed_time DESC)
		 WHERE ROWNUM <= ` + strconv.Itoa(limit)

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	return driver.SlowQueries{
		Columns: result.Columns,
		Rows:    result.Rows,
		// Not complete: a shared cursor carries its bind variables rather than its values. The
		// schema that parsed it is what a script enters, this engine having no database below it.
		Refs: sqlkit.SlowQueryRefs(
			result.Columns, result.Rows, sqlkit.Oracle, "Statement", "Schema", false),
	}, nil
}

// ProfileConfig says this engine is always recording and that there is nothing to set — SQL Server's
// answer, and for the same reason: a shared pool is a fact about Oracle rather than about the
// instance in front of it.
func (s *session) ProfileConfig(
	context.Context,
	driver.Namespace,
) (driver.ProfileConfig, error) {
	return driver.ProfileConfig{
		Enabled:      true,
		Configurable: false,
		Ready:        true,
		Detail: []driver.ProfileFact{
			{Label: "Source", Value: "v$sqlarea"},
			{Label: "Recording", Value: "Always, as long as the cursor stays in the shared pool"},
		},
	}, nil
}
