package postgres

import (
	"context"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a PostgreSQL server has already run **slowly**. Nothing here spells a change: that is
// `sqlkit.Profile`'s, and on this engine the switch is an extension rather than a setting — which is
// the whole of what makes this adapter's half of it different from MySQL's.
//
// **`pg_stat_statements` counts every statement and this pane picks the slow ones out.** There is no
// threshold to set: the extension does not have one, and the setting that does —
// `log_min_duration_statement` — writes to a file on the server's own disk that this app cannot open.
// A box for a number that changed nothing on this pane would be worse than no box.
//
// **What it keeps is normalized.** Every value has already been replaced by `$1` before the row is
// written, which is how one statement's thousand runs become one row — so a statement copied out of
// this pane is one the reader fills in before any server will take it.

// slowQueryLimit is what a request with no limit of its own reads.
const slowQueryLimit = 200

// SlowQueries reads the extension's view, slowest **in total** first. Total rather than mean is the
// honest order for a list somebody is hunting a problem in: a statement taking 20ms and run a
// million times is the one worth finding, and it would be nowhere near the top of a list ordered by
// how long one run took.
func (s *session) SlowQueries(
	ctx context.Context,
	ns driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	db, err := s.pool(ns.Database)
	if err != nil {
		return driver.SlowQueries{}, err
	}

	// The view is the **cluster's**, not the database's: `pg_stat_statements` holds a row per
	// statement per database, so a read that did not narrow it would show a reader statements from
	// databases they are not connected to. The extension is installed per database and the pane is
	// read per database; this is what makes the two agree.
	total, mean := s.timeColumns(ctx, ns)
	query := `SELECT round(` + total + `::numeric, 2) AS "Total ms",
		       calls AS "Calls",
		       round(` + mean + `::numeric, 2) AS "Mean ms",
		       rows AS "Rows",
		       current_database() AS "Database",
		       query AS "Statement"
		  FROM pg_stat_statements
		 WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		 ORDER BY ` + total + ` DESC
		 LIMIT ` + strconv.Itoa(limit)

	result, err := sqlkit.Read(ctx, db, query, 0, nil)
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	return driver.SlowQueries{
		Columns: result.Columns,
		Rows:    result.Rows,
		// Not complete: every value was replaced by `$1` before the row was written.
		Refs: sqlkit.SlowQueryRefs(
			result.Columns, result.Rows, sqlkit.Postgres, "Statement", "Database", false),
	}, nil
}

// timeColumns is what this server's extension calls its two timings. They were renamed in
// `pg_stat_statements` 1.8, which shipped with PostgreSQL 13 and split what had been one number into
// execution and planning; a server older than that has the earlier pair and no others.
//
// It is read from the catalogue rather than derived from the server's version, because the extension
// has a version of its own: a PostgreSQL 13 that was upgraded from 12 keeps whatever version of the
// extension was installed until somebody runs `ALTER EXTENSION … UPDATE`.
func (s *session) timeColumns(ctx context.Context, ns driver.Namespace) (total, mean string) {
	const query = `SELECT count(*)
	                 FROM pg_catalog.pg_attribute a
	                 JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
	                WHERE c.relname = 'pg_stat_statements'
	                  AND a.attname = 'total_exec_time'`

	db, err := s.pool(ns.Database)
	if err != nil {
		return "total_time", "mean_time"
	}

	found := 0
	if err := db.QueryRowContext(ctx, query).Scan(&found); err != nil || found == 0 {
		return "total_time", "mean_time"
	}
	return "total_exec_time", "mean_exec_time"
}

// ProfileConfig says whether the extension is installed in **this** database, which is the question
// that decides whether the pane above holds anything: an extension is a per-database object, so a
// reader who installed it in one database has not installed it in another.
//
// A server that has the extension available but not created is one statement away, and saying so is
// the difference between a feature that looks broken and one that is waiting.
func (s *session) ProfileConfig(
	ctx context.Context,
	ns driver.Namespace,
) (driver.ProfileConfig, error) {
	const query = `SELECT
		  (SELECT count(*) FROM pg_catalog.pg_extension WHERE extname = 'pg_stat_statements'),
		  (SELECT count(*) FROM pg_catalog.pg_available_extensions
		    WHERE name = 'pg_stat_statements')`

	db, err := s.pool(ns.Database)
	if err != nil {
		return driver.ProfileConfig{}, err
	}

	installed, available := 0, 0
	if err := db.QueryRowContext(ctx, query).Scan(&installed, &available); err != nil {
		return driver.ProfileConfig{}, translateError(err)
	}

	config := driver.ProfileConfig{
		Enabled:      installed > 0,
		Configurable: true,
		Ready:        installed > 0,
		Detail: []driver.ProfileFact{
			{Label: "Extension", Value: "pg_stat_statements"},
			{Label: "Installed in this database", Value: yesNo(installed > 0)},
			{Label: "Available on this server", Value: yesNo(available > 0)},
		},
	}

	switch {
	case installed > 0:
	case available > 0:
		config.Reason = "pg_stat_statements is available but not installed in this database." +
			" Turning the recording on creates it."
	default:
		// The extension has to be loaded at startup, which is a line in a configuration file and a
		// restart of the server — neither of which this app can do, and neither of which it should
		// pretend to by offering a switch that will be refused.
		config.Reason = "This server does not offer pg_stat_statements." +
			" It has to be added to shared_preload_libraries and the server restarted."
	}
	return config, nil
}
