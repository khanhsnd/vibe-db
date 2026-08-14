package mysql

import (
	"context"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// What a MySQL server has already run **slowly**. Nothing here spells a change: that is
// `sqlkit.Profile`'s, which this session embeds — the switch, the threshold and the reset are all
// flags on the dialect. What is here is two catalogue reads.
//
// **The slow log is read as a table, not as a file.** MySQL will write it either way, and `FILE` is
// the default; a file on the server's own disk is not something this app can open, so turning the
// recording on sets `log_output` to `TABLE` in the same breath. That is why the switch is two
// statements and why the pane says which of the two settings is missing when a server has one and
// not the other.
//
// **`mysql.slow_log` holds the statement as it ran**, values and all, which is the whole reason this
// engine reads there rather than from `performance_schema`'s digest summary: a digest has already
// replaced every value with a `?`, so a statement copied out of one is not a statement any server
// will take. What this pane shows is what actually ran.

// slowQueryLimit is what a request with no limit of its own reads. The log is a table and a busy
// server fills it, so a read that asked for all of it would be a read nobody could wait for.
const slowQueryLimit = 200

// SlowQueries reads the log newest first. `query_time` and `lock_time` are `TIME(6)` columns rather
// than numbers, which is how MySQL keeps them, and they are handed to the grid in the engine's own
// spelling for the reason every other catalogue read is.
func (s *session) SlowQueries(
	ctx context.Context,
	_ driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	query := "SELECT start_time AS `Started`," +
		" query_time AS `Duration`," +
		" lock_time AS `Locked`," +
		" rows_sent AS `Rows Sent`," +
		" rows_examined AS `Rows Examined`," +
		" db AS `Database`," +
		" user_host AS `Account`," +
		" CONVERT(sql_text USING utf8mb4) AS `Statement`" +
		" FROM mysql.slow_log" +
		" ORDER BY start_time DESC" +
		" LIMIT " + strconv.Itoa(limit)

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	return driver.SlowQueries{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs: sqlkit.SlowQueryRefs(
			result.Columns, result.Rows, sqlkit.MySQL, "Statement", "Database", true),
	}, nil
}

// ProfileConfig reads the three settings that decide whether the pane above will hold anything, and
// says which of them is in the way rather than leaving the reader with an empty list and no reason.
func (s *session) ProfileConfig(
	ctx context.Context,
	_ driver.Namespace,
) (driver.ProfileConfig, error) {
	const query = "SHOW GLOBAL VARIABLES WHERE Variable_name IN" +
		" ('slow_query_log', 'long_query_time', 'log_output')"

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.ProfileConfig{}, translateError(err)
	}

	settings := map[string]string{}
	for _, row := range result.Rows {
		if len(row) < 2 || row[0] == nil || row[1] == nil {
			continue
		}
		settings[strings.ToLower(*row[0])] = *row[1]
	}

	// `log_output` is a list — `TABLE,FILE` is a server writing both — so the question is whether
	// `TABLE` is among them rather than whether it is the whole of it.
	toTable := strings.Contains(strings.ToUpper(settings["log_output"]), "TABLE")
	on := strings.EqualFold(settings["slow_query_log"], "ON")

	config := driver.ProfileConfig{
		Enabled:      on && toTable,
		ThresholdMs:  milliseconds(settings["long_query_time"]),
		Configurable: true,
		Ready:        true,
		Detail: []driver.ProfileFact{
			{Label: "slow_query_log", Value: settings["slow_query_log"]},
			{Label: "log_output", Value: settings["log_output"]},
			{Label: "long_query_time", Value: settings["long_query_time"] + "s"},
		},
	}

	// A server recording to a file is recording — it is simply recording where this app cannot
	// read. Saying so is the whole point: the reader is one setting away rather than looking at a
	// feature that appears not to work.
	if on && !toTable {
		config.Ready = false
		config.Reason = "This server writes its slow log to a file, which this app cannot read." +
			" Turning the recording on here sets log_output to TABLE as well."
	}
	return config, nil
}

// milliseconds reads `long_query_time`, which MySQL keeps as a number of **seconds** and often with
// a decimal — `0.000100` is the smallest it takes. A value it cannot read is no threshold rather
// than a zero, zero being what "record everything" would mean if it were written back.
func milliseconds(seconds string) int {
	value, err := strconv.ParseFloat(strings.TrimSpace(seconds), 64)
	if err != nil {
		return 0
	}
	return int(value * 1000)
}
