package postgres

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a PostgreSQL server is doing, and what it was started with. Nothing here spells a kill:
// `pg_cancel_backend` and `pg_terminate_backend` are flags on the dialect and `sqlkit.Watch` runs
// them. What is here is three reads.
//
// **A backend is what PostgreSQL calls a connection**, and it is a process of the operating system:
// `pg_stat_activity` is one row per backend of the whole **cluster**, which is why every read here
// goes to the session's own database and still answers for every database on the server. An account
// that is not a superuser and holds no `pg_read_all_stats` sees its own backends' statements and
// somebody else's as blank rather than being refused, which is the server's own rule.

// Processes reads the backend list. `Running` is how long the current statement has been going, and
// is spelled here rather than in the reader for the reason MySQL's KB arithmetic is: the interval
// belongs beside the number the server keeps.
func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	const query = `
		SELECT pid AS "ID",
		       usename AS "User",
		       client_addr AS "Client",
		       datname AS "Database",
		       state AS "State",
		       wait_event_type AS "Wait",
		       backend_type AS "Type",
		       to_char(now() - query_start, 'HH24:MI:SS') AS "Running",
		       query AS "Statement"
		FROM pg_catalog.pg_stat_activity
		ORDER BY pid`

	db, err := s.pool("")
	if err != nil {
		return driver.ProcessList{}, err
	}

	result, err := sqlkit.Read(ctx, db, query, 0, nil)
	if err != nil {
		return driver.ProcessList{}, translateError(err)
	}
	return driver.ProcessList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.ProcessRefs(result.Columns, result.Rows, "ID", "", s.ownProcess(ctx, db)),
	}, nil
}

// ownProcess is the backend this read came back on, which is what marks a row as one of ours. A
// session holds a pool per database, so this names one of ours rather than the only one — see
// `driver.Monitor.Processes`.
func (s *session) ownProcess(ctx context.Context, db *sql.DB) string {
	var pid string
	if err := db.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		return ""
	}
	return pid
}

// ServerStatus is `pg_stat_database`, one row per database with every counter the server keeps about
// it. It is **not** a list of name and value pairs, because that is not what this engine counts with:
// there is no `SHOW STATUS` here, and folding twenty columns of fourteen databases into pairs would
// be inventing a shape the server has not got. The columns are the engine's own, exactly as a table
// list's are.
//
// `SELECT *` is deliberate: the columns of this view have grown with the releases, and naming them
// would be choosing which server versions this pane works on.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	return s.facts(ctx, `SELECT * FROM pg_catalog.pg_stat_database ORDER BY datname`)
}

// ServerVariables is `pg_settings`, which is what `SHOW ALL` reads and rather more: a setting here
// carries its unit, where it came from and whether it is waiting for a restart, and all of that is
// what a reader is looking at the list to find out.
func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	const query = `
		SELECT name AS "Name",
		       setting AS "Value",
		       unit AS "Unit",
		       category AS "Category",
		       context AS "Context",
		       source AS "Source",
		       pending_restart AS "Pending Restart",
		       short_desc AS "Description"
		FROM pg_catalog.pg_settings
		ORDER BY name`

	return s.facts(ctx, query)
}

func (s *session) facts(ctx context.Context, query string) (driver.ServerFacts, error) {
	db, err := s.pool("")
	if err != nil {
		return driver.ServerFacts{}, err
	}

	result, err := sqlkit.Read(ctx, db, query, 0, nil)
	if err != nil {
		return driver.ServerFacts{}, translateError(err)
	}
	return driver.ServerFacts{Columns: result.Columns, Rows: result.Rows}, nil
}

// postgresMetrics is what the four charts are drawn from, named by the query below rather than by a
// catalogue: this engine keeps its sessions in one view and its counters in another, and has no
// `SHOW STATUS` to pick either out of.
//
// **The traffic panel is blocks and not bytes on the wire.** PostgreSQL counts nothing about its own
// sockets, so what is honestly there is the traffic it moved to answer: pages off the disk, and pages
// it already had. The labels say which, because a panel that called either of them "sent to clients"
// would be this app inventing a number the server never kept.
//
// **There is no statement counter here either** without an extension, so the queries panel is what a
// bare server does count — transactions, and the deadlocks it resolved. What ran *slowly* is the
// [Slow Queries pane](../../../docs/features/profiler.md), which is where `pg_stat_statements` is read.
var postgresMetrics = []driver.MetricSpec{
	{Panel: domain.MetricSessions, Name: "active", Label: "Active",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "idle", Label: "Idle",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "idle_in_transaction", Label: "Idle in transaction",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "backends", Label: "Backends",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},

	{Panel: domain.MetricTraffic, Name: "blks_read", Label: "Read from disk",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	{Panel: domain.MetricTraffic, Name: "blks_hit", Label: "Read from cache",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},

	{Panel: domain.MetricQueries, Name: "xact_commit", Label: "Commits",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "xact_rollback", Label: "Rollbacks",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "deadlocks", Label: "Deadlocks",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},

	{Panel: domain.MetricThroughput, Name: "tup_returned", Label: "Rows returned",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "tup_fetched", Label: "Rows fetched",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "tup_inserted", Label: "Rows inserted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "tup_updated", Label: "Rows updated",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "tup_deleted", Label: "Rows deleted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
}

// ServerMetrics is the four charts' numbers in **one** read. The sessions are counted in
// `pg_stat_activity` and everything else is summed over `pg_stat_database`, and the two are one query
// so that every point on every chart is the same instant — two reads would draw a session count from
// one moment beside a rate from another.
//
// The sums go over the whole view rather than over this connection's database, because that is what
// the panes beside this one already report: `pg_stat_activity` is the cluster's backends, and a chart
// of one database's rows beside a count of every database's backends would be two servers on one
// screen. A page is multiplied out by `block_size` rather than by 8192, this being a build-time
// setting and the server the only one that knows it.
func (s *session) ServerMetrics(ctx context.Context) (driver.ServerMetrics, error) {
	const query = `
		SELECT 'active' AS "Name", count(*)::float8 AS "Value"
		  FROM pg_catalog.pg_stat_activity WHERE state = 'active'
		UNION ALL SELECT 'idle', count(*)::float8
		  FROM pg_catalog.pg_stat_activity WHERE state = 'idle'
		UNION ALL SELECT 'idle_in_transaction', count(*)::float8
		  FROM pg_catalog.pg_stat_activity WHERE state LIKE 'idle in transaction%'
		UNION ALL SELECT 'backends', count(*)::float8
		  FROM pg_catalog.pg_stat_activity
		UNION ALL SELECT 'blks_read',
		       coalesce(sum(blks_read), 0)::float8 * current_setting('block_size')::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'blks_hit',
		       coalesce(sum(blks_hit), 0)::float8 * current_setting('block_size')::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'xact_commit', coalesce(sum(xact_commit), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'xact_rollback', coalesce(sum(xact_rollback), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'deadlocks', coalesce(sum(deadlocks), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'tup_returned', coalesce(sum(tup_returned), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'tup_fetched', coalesce(sum(tup_fetched), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'tup_inserted', coalesce(sum(tup_inserted), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'tup_updated', coalesce(sum(tup_updated), 0)::float8
		  FROM pg_catalog.pg_stat_database
		UNION ALL SELECT 'tup_deleted', coalesce(sum(tup_deleted), 0)::float8
		  FROM pg_catalog.pg_stat_database`

	facts, err := s.facts(ctx, query)
	if err != nil {
		return driver.ServerMetrics{}, err
	}

	pairs := driver.MetricPairs(facts.Columns, facts.Rows, "Name", "Value")
	return driver.ServerMetrics{Metrics: driver.PickMetrics(postgresMetrics, pairs)}, nil
}
