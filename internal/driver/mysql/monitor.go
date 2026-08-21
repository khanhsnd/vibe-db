package mysql

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a MySQL server is doing, and what it was started with. Nothing here spells a kill: that is
// `sqlkit.Watch`'s, which this session embeds — `KILL QUERY` and `KILL CONNECTION` are flags on the
// dialect. What is here is three catalogue reads.
//
// **A thread is what MySQL calls a connection**, and `information_schema.processlist` is one row per
// thread the server holds. An account without `PROCESS` sees its own threads and no error, which is
// the honest answer to "what is this server doing" asked by somebody who may only see their own — the
// same arrangement the account list makes.

// Processes reads the thread list. `Statement` is the whole of what the thread is running: the column
// is `info`, which MySQL truncates only in `SHOW PROCESSLIST` and not here.
func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	const query = "SELECT id AS `ID`," +
		" user AS `User`," +
		" host AS `Host`," +
		" db AS `Database`," +
		" command AS `Command`," +
		" time AS `Time`," +
		" state AS `State`," +
		" info AS `Statement`" +
		" FROM information_schema.processlist" +
		" ORDER BY id"

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

// ownProcess is the thread this read came back on, which is what marks a row as one of ours. It is a
// read of its own rather than a column of the list above, so the grid draws what a reader would see
// in any other client; and it answers with nothing rather than an error, a mark being worth less than
// the list it is on.
func (s *session) ownProcess(ctx context.Context) string {
	var id string
	if err := s.db.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&id); err != nil {
		return ""
	}
	return id
}

// ServerStatus is what the server has counted since it started, and is asked **globally**: the
// session's own counters are the counters of one pooled connection, which is not a fact about the
// server at all.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	return s.facts(ctx, "SHOW GLOBAL STATUS")
}

func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	return s.facts(ctx, "SHOW GLOBAL VARIABLES")
}

// facts is either of the two reads above: both answer with MySQL's own two columns,
// `Variable_name` and `Value`, which is what a reader of this engine already knows them by.
func (s *session) facts(ctx context.Context, query string) (driver.ServerFacts, error) {
	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.ServerFacts{}, translateError(err)
	}
	return driver.ServerFacts{Columns: result.Columns, Rows: result.Rows}, nil
}

// mysqlMetrics is which of `SHOW GLOBAL STATUS`'s several hundred numbers the four charts are drawn
// from, and what each is called on one. MySQL is the easiest of the six: every number here is in the
// one list the Status pane already draws, so this read is that read with a table run over it.
//
// The throughput panel is `Innodb_rows_*` rather than `Com_select` and friends, and the difference is
// the whole point of that panel: a `Com_select` is somebody *asking*, and a row read is what the
// server actually moved to answer them. A server on another storage engine reports none of the four,
// and the panel is empty rather than flat — see `driver.PickMetrics`.
var mysqlMetrics = []driver.MetricSpec{
	{Panel: domain.MetricSessions, Name: "Threads_connected", Label: "Connected",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "Threads_running", Label: "Running",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "Threads_cached", Label: "Cached",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},

	{Panel: domain.MetricTraffic, Name: "Bytes_received", Label: "Received from clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	{Panel: domain.MetricTraffic, Name: "Bytes_sent", Label: "Sent to clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},

	{Panel: domain.MetricQueries, Name: "Questions", Label: "Statements",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "Slow_queries", Label: "Slow",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "Com_commit", Label: "Commits",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "Com_rollback", Label: "Rollbacks",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},

	{Panel: domain.MetricThroughput, Name: "Innodb_rows_read", Label: "Rows read",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "Innodb_rows_inserted", Label: "Rows inserted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "Innodb_rows_updated", Label: "Rows updated",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "Innodb_rows_deleted", Label: "Rows deleted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
}

// ServerMetrics is the four charts' numbers, all of them out of the one global status. It is asked
// globally for the reason ServerStatus is: a session's own counters are one pooled connection's.
func (s *session) ServerMetrics(ctx context.Context) (driver.ServerMetrics, error) {
	facts, err := s.facts(ctx, "SHOW GLOBAL STATUS")
	if err != nil {
		return driver.ServerMetrics{}, err
	}

	pairs := driver.MetricPairs(facts.Columns, facts.Rows, "Variable_name", "Value")
	return driver.ServerMetrics{Metrics: driver.PickMetrics(mysqlMetrics, pairs)}, nil
}
