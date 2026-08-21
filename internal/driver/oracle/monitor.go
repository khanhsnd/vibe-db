package oracle

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What an Oracle instance is doing, and what it was started with. The one kill it has —
// `ALTER SYSTEM KILL SESSION` — is a flag on the dialect and `sqlkit.Watch` runs it.
//
// **A session here is the pair `sid,serial#`**, which is the whole reason `driver.ProcessRef` carries
// a serial: a sid is handed to the next session the moment this one ends, so a statement naming one
// alone would end whoever came after. The list therefore draws both, and the kill quotes both.
//
// **There is no cancel.** `ALTER SYSTEM CANCEL SQL` arrived in 18c and is addressed by the statement's
// own id rather than by the session, so an app that guessed at it would either end the wrong thing or
// be refused by every older server. It says so instead — see the dialect's `CancelStatementSQL`.
//
// Every read here is of a `v$` view, which needs `SELECT_CATALOG_ROLE` or the privilege on its own.
// A reader without it gets the server's own refusal, which names the view: there is nothing narrower
// to fall back to, the `all_` and `user_` views having no counterpart for what a session is doing.

// Processes reads the session list with whatever each one is running. `v$sql` is joined rather than
// read per row, because the statement text lives with the *cursor* and not with the session.
func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	// `type` is drawn rather than filtered on: an instance holds fifty background sessions of its own,
	// and leaving them out would be this app deciding which of a server's sessions count. The box over
	// the pane is where a reader narrows it to `USER`.
	//
	// A date is read as text — `TO_CHAR` — because every value in this app reaches the grid as one, and
	// letting the client decide the format would put a different date on the screen per driver build.
	const query = `
		SELECT s.sid AS "ID",
		       s.serial# AS "Serial",
		       s.username AS "User",
		       s.status AS "State",
		       s.type AS "Type",
		       s.osuser AS "OS User",
		       s.machine AS "Machine",
		       s.program AS "Program",
		       s.event AS "Wait",
		       s.seconds_in_wait AS "Waiting",
		       TO_CHAR(s.logon_time, 'YYYY-MM-DD HH24:MI:SS') AS "Connected",
		       q.sql_text AS "Statement"
		FROM v$session s
		LEFT JOIN v$sql q ON q.sql_id = s.sql_id
		ORDER BY s.sid`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.ProcessList{}, translateError(err)
	}
	return driver.ProcessList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.ProcessRefs(result.Columns, result.Rows, "ID", "Serial", s.ownProcess(ctx)),
	}, nil
}

// ownProcess is the session this read came back on, which is what marks a row as one of ours. It is
// read out of the connection's own context rather than out of `v$session`, so it answers even for a
// reader who may not see the view at all.
func (s *session) ownProcess(ctx context.Context) string {
	const query = `SELECT SYS_CONTEXT('USERENV', 'SID') FROM dual`

	var sid string
	if err := s.db.QueryRowContext(ctx, query).Scan(&sid); err != nil {
		return ""
	}
	return sid
}

// ServerStatus is `v$sysstat`, one row per number the instance has been counting since it started,
// with the class the server sorts it into.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	const query = `
		SELECT name AS "Name",
		       value AS "Value",
		       class AS "Class"
		FROM v$sysstat
		ORDER BY name`

	return s.facts(ctx, query)
}

// ServerVariables is `v$parameter`: what the instance was started with, what it reads as, whether it
// is still the default and whether it can be changed at all without a restart.
func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	const query = `
		SELECT name AS "Name",
		       display_value AS "Value",
		       isdefault AS "Default",
		       ismodifiable AS "Modifiable",
		       description AS "Description"
		FROM v$parameter
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

// oracleMetrics is what the four charts are drawn from. All but the sessions come out of `v$sysstat`,
// which is this engine's `SHOW STATUS` under another name; the sessions are counted in `v$session`,
// there being no counter for "how many are connected right now".
//
// **The traffic panel is SQL*Net and so is genuinely bytes on the wire**, which two of the six engines
// here cannot say. The throughput panel is blocks rather than rows, because that is what an Oracle
// instance counts: a logical read is a block found in the cache and a physical read is one that was
// not, and the pair of them together is the honest reading of how hard the instance is working.
var oracleMetrics = []driver.MetricSpec{
	{Panel: domain.MetricSessions, Name: "active", Label: "Active",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "inactive", Label: "Inactive",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "sessions", Label: "User sessions",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},

	{Panel: domain.MetricTraffic, Name: "bytes received via SQL*Net from client",
		Label: "Received from clients", Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	{Panel: domain.MetricTraffic, Name: "bytes sent via SQL*Net to client",
		Label: "Sent to clients", Kind: domain.MetricCounter, Unit: domain.MetricBytes},

	{Panel: domain.MetricQueries, Name: "execute count", Label: "Executions",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "parse count (total)", Label: "Parses",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "user commits", Label: "Commits",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "user rollbacks", Label: "Rollbacks",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},

	{Panel: domain.MetricThroughput, Name: "session logical reads", Label: "Logical reads",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "physical reads", Label: "Physical reads",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "db block changes", Label: "Block changes",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "table scan rows gotten", Label: "Rows scanned",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
}

// ServerMetrics is the four charts' numbers in one read. `v$session` is counted in the same statement
// as `v$sysstat` is read, so a session count and a rate on the two charts beside each other are the
// same instant rather than two.
//
// The statistic names travel as the strings this engine keeps them under — spaces, a `*` and all —
// because they are what the table above matches on, and renaming them here would put a name on the
// chart that is not the one a reader would find in `v$sysstat`.
func (s *session) ServerMetrics(ctx context.Context) (driver.ServerMetrics, error) {
	const query = `
		SELECT 'active' AS "Name", COUNT(*) AS "Value"
		  FROM v$session WHERE type = 'USER' AND status = 'ACTIVE'
		UNION ALL SELECT 'inactive', COUNT(*)
		  FROM v$session WHERE type = 'USER' AND status = 'INACTIVE'
		UNION ALL SELECT 'sessions', COUNT(*)
		  FROM v$session WHERE type = 'USER'
		UNION ALL SELECT name, value FROM v$sysstat
		 WHERE name IN ('bytes received via SQL*Net from client',
		                'bytes sent via SQL*Net to client',
		                'execute count',
		                'parse count (total)',
		                'user commits',
		                'user rollbacks',
		                'session logical reads',
		                'physical reads',
		                'db block changes',
		                'table scan rows gotten')`

	facts, err := s.facts(ctx, query)
	if err != nil {
		return driver.ServerMetrics{}, err
	}

	pairs := driver.MetricPairs(facts.Columns, facts.Rows, "Name", "Value")
	return driver.ServerMetrics{Metrics: driver.PickMetrics(oracleMetrics, pairs)}, nil
}
