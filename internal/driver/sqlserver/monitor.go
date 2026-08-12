package sqlserver

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a SQL Server instance is doing, and what it was started with. The one kill it has —
// `KILL <spid>` — is a flag on the dialect and `sqlkit.Watch` runs it; there is no cancel here,
// because this engine has no statement that stops a request and leaves its session.
//
// **A session and a request are two things**, which is the shape of every read below: a session is
// somebody connected, and a request is what that session is running *now* — so the list is
// `dm_exec_sessions` with `dm_exec_requests` beside it, and a session running nothing carries blanks
// rather than being left out. Reading either needs `VIEW SERVER STATE`; an account without it sees
// its own session alone, which is the server's own rule.

// Processes reads the session list with whatever each one is running. The statement text comes from
// `dm_exec_sql_text`, which takes a handle rather than being a column — that is what `OUTER APPLY` is
// for, and it is why a session with nothing in flight still appears.
func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	const query = `
		SELECT s.session_id AS [ID],
		       s.login_name AS [User],
		       s.host_name AS [Host],
		       DB_NAME(s.database_id) AS [Database],
		       s.status AS [State],
		       r.command AS [Command],
		       r.wait_type AS [Wait],
		       r.total_elapsed_time AS [Elapsed ms],
		       CASE WHEN s.is_user_process = 1 THEN 'user' ELSE 'system' END AS [Type],
		       s.program_name AS [Program],
		       t.text AS [Statement]
		FROM sys.dm_exec_sessions s
		LEFT JOIN sys.dm_exec_requests r ON r.session_id = s.session_id
		OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) t
		ORDER BY s.session_id`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.ProcessList{}, translateError(err)
	}
	return driver.ProcessList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.ProcessRefs(result.Columns, result.Rows, "ID", "", s.ownProcess(ctx)),
	}, nil
}

// ownProcess is the session this read came back on, which is what marks a row as one of ours. A pool
// holds a handful of connections, so it names one of ours rather than the only one — see
// `driver.Monitor.Processes`.
func (s *session) ownProcess(ctx context.Context) string {
	var spid string
	if err := s.db.QueryRowContext(ctx, "SELECT CAST(@@SPID AS nvarchar(16))").Scan(&spid); err != nil {
		return ""
	}
	return spid
}

// ServerStatus is `dm_os_performance_counters`, which is where this engine keeps everything another
// would call a status variable. It is a long list on purpose: the pane it fills has a box to narrow
// it, and a shorter list would be this app choosing which counters matter.
//
// The two names are trimmed because the server stores them padded to a fixed width, and a column of
// trailing spaces is a column nobody can read a value beside.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	const query = `
		SELECT RTRIM(object_name) AS [Object],
		       RTRIM(counter_name) AS [Name],
		       RTRIM(instance_name) AS [Instance],
		       cntr_value AS [Value]
		FROM sys.dm_os_performance_counters
		ORDER BY RTRIM(object_name), RTRIM(counter_name), RTRIM(instance_name)`

	return s.facts(ctx, query)
}

// ServerVariables is `sys.configurations`, this engine's own name for what everybody else calls a
// variable: what it was set to, what is actually in force, and what it will take. The values are cast
// because the columns are `sql_variant`, which no client reads as text on its own.
func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	const query = `
		SELECT name AS [Name],
		       CAST(value AS nvarchar(128)) AS [Value],
		       CAST(value_in_use AS nvarchar(128)) AS [In Use],
		       CAST(minimum AS nvarchar(128)) AS [Minimum],
		       CAST(maximum AS nvarchar(128)) AS [Maximum],
		       is_dynamic AS [Takes Effect At Once],
		       is_advanced AS [Advanced],
		       description AS [Description]
		FROM sys.configurations
		ORDER BY name`

	return s.facts(ctx, query)
}

func (s *session) facts(ctx context.Context, query string) (driver.ServerFacts, error) {
	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.ServerFacts{}, translateError(err)
	}
	return driver.ServerFacts{Columns: result.Columns, Rows: result.Rows}, nil
}
