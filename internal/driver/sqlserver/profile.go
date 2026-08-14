package sqlserver

import (
	"context"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a SQL Server instance has already run **slowly**. Nothing here spells a change: that is
// `sqlkit.Profile`'s, and on this engine there is only one — emptying the plan cache.
//
// **This engine records whether or not anybody asked.** `sys.dm_exec_query_stats` holds a row per
// plan in the cache with what every run of it cost, so there is no switch to turn on and no
// threshold to set: what a reader picks is which end of the list to read. That is
// `ProfileConfig.Configurable` false, and it is the honest answer rather than a dialog whose every
// control is dead.
//
// **The text is the batch's own**, read through `sys.dm_exec_sql_text` and cut to the statement's
// own offsets, so what reaches a query tab is the statement rather than the procedure it sat in.
// Where the statement was parameterized the text carries `@P1` and the like — which the reader can
// see, and which this adapter does not claim to have normalized away, the offsets being the server's
// own words either way.

// slowQueryLimit is what a request with no limit of its own reads. A plan cache holds thousands and
// a read that asked for all of them would be a read nobody could wait for.
const slowQueryLimit = 200

// SlowQueries reads the cache, worst **in total** first — the order that finds the statement costing
// the server the most, rather than the one unlucky run.
//
// The offsets are in bytes of `nvarchar`, which is why they are halved and why the end offset of
// `-1` — the server's way of saying "to the end" — is read before the arithmetic rather than after.
func (s *session) SlowQueries(
	ctx context.Context,
	_ driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	query := `SELECT TOP (` + strconv.Itoa(limit) + `)
		       CAST(qs.total_elapsed_time / 1000.0 AS decimal(18, 2)) AS [Total ms],
		       qs.execution_count AS [Calls],
		       CAST(qs.total_elapsed_time / 1000.0 / qs.execution_count
		            AS decimal(18, 2)) AS [Mean ms],
		       qs.total_logical_reads AS [Logical Reads],
		       qs.last_execution_time AS [Last Run],
		       DB_NAME(t.dbid) AS [Database],
		       SUBSTRING(t.text,
		                 qs.statement_start_offset / 2 + 1,
		                 (CASE qs.statement_end_offset
		                       WHEN -1 THEN DATALENGTH(t.text)
		                       ELSE qs.statement_end_offset
		                  END - qs.statement_start_offset) / 2 + 1) AS [Statement]
		  FROM sys.dm_exec_query_stats AS qs
		 CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) AS t
		 ORDER BY qs.total_elapsed_time DESC`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	return driver.SlowQueries{
		Columns: result.Columns,
		Rows:    result.Rows,
		// `dbid` is null for an ad-hoc batch that named its objects in full, and such a row simply
		// carries no `USE` — which is right, it belongs to no one database.
		Refs: sqlkit.SlowQueryRefs(
			result.Columns, result.Rows, sqlkit.SQLServer, "Statement", "Database", true),
	}, nil
}

// ProfileConfig says this engine is always recording and that there is nothing to set. It asks the
// server nothing: that a plan cache exists is a fact about SQL Server rather than about the instance
// in front of it, and a read that could fail would be a read that made the pane look conditional.
func (s *session) ProfileConfig(
	context.Context,
	driver.Namespace,
) (driver.ProfileConfig, error) {
	return driver.ProfileConfig{
		Enabled:      true,
		Configurable: false,
		Ready:        true,
		Detail: []driver.ProfileFact{
			{Label: "Source", Value: "sys.dm_exec_query_stats"},
			{Label: "Recording", Value: "Always, as long as the plan stays in the cache"},
		},
	}, nil
}
