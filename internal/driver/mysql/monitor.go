package mysql

import (
	"context"

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
