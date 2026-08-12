package postgres

import (
	"context"
	"database/sql"

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
