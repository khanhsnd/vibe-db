package redis

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// What a Redis server has already run **slowly**. Nothing here touches `sqlkit.Profile`: there is no
// statement to spell, every one of these being a command whose arguments are its own words — the
// arrangement `monitor.go` has, and for the same reason.
//
// **`SLOWLOG` is a ring in memory**, not a table and not a file. The server keeps the last
// `slowlog-max-len` commands that took longer than `slowlog-log-slower-than`, and both are ordinary
// settings a `CONFIG SET` moves. It is the whole of what this engine records, and it is a server
// fact rather than a database one — a key space being numbered rather than named, there is nothing
// per-database for it to belong to.
//
// **The threshold is in microseconds**, which is the one place in this app a number is not
// milliseconds. A reader is asked in milliseconds everywhere, so the conversion happens here — and
// `0` means *log everything* on this engine while `-1` means *log nothing*, which is why turning the
// recording off is not the same as setting the threshold to zero.
//
// **What it keeps is the command as it ran**, arguments and all, so a row is something a reader can
// copy into a tab as it stands.

// slowLogThreshold and slowLogLength are what a Redis server calls the two settings behind this pane.
const (
	slowLogThreshold = "slowlog-log-slower-than"
	slowLogLength    = "slowlog-max-len"
)

// slowLogOff is what the threshold is set to for "record nothing". It is the server's own word for
// it, and it is why disabling is a `CONFIG SET` rather than a switch of its own.
const slowLogOff = "-1"

// slowQueryLimit is what a request with no limit of its own reads. The ring is bounded by the server
// anyway — 128 entries by default — so this is a ceiling rather than a page.
const slowQueryLimit = 200

// SlowQueries reads the ring, newest first, which is the order the server keeps it in. It is not
// sorted by duration here: what a reader watches a slow log for is what just happened, and a list
// reordered by size would hide the entry they were waiting for behind last week's worst.
func (s *session) SlowQueries(
	ctx context.Context,
	_ driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.SlowQueries{}, err
	}

	entries, err := client.SlowLogGet(ctx, int64(limit)).Result()
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}

	list := driver.SlowQueries{
		Columns: []driver.Column{
			{Name: "ID", Type: "string", FullType: "string"},
			{Name: "When", Type: "string", FullType: "string"},
			{Name: "Duration ms", Type: "string", FullType: "string"},
			{Name: "Client", Type: "string", FullType: "string"},
			{Name: "Name", Type: "string", FullType: "string"},
			{Name: "Command", Type: "string", FullType: "string"},
		},
		Rows: [][]*string{},
		Refs: []driver.SlowQueryRef{},
	}

	for _, entry := range entries {
		command := strings.Join(entry.Args, " ")
		list.Rows = append(list.Rows, []*string{
			cell(strconv.FormatInt(entry.ID, 10)),
			cell(entry.Time.Format(time.RFC3339)),
			cell(strconv.FormatFloat(float64(entry.Duration.Microseconds())/1000, 'f', 3, 64)),
			cell(entry.ClientAddr),
			cell(entry.ClientName),
			cell(command),
		})
		// A recorded command runs as it stands, and there is **nothing to put in front of it**: a
		// Redis database is a number rather than a name, and `SELECT` is refused on this app's own
		// connections anyway — see the key space. What the reader copies is what the server ran.
		list.Refs = append(list.Refs, driver.SlowQueryRef{Runnable: command, Complete: true})
	}
	return list, nil
}

// ProfileConfig reads the two settings. A hosted Redis usually refuses `CONFIG` outright, and that
// refusal is the server's own words — a reader who cannot read the configuration is told so rather
// than shown a server that appears to record nothing.
func (s *session) ProfileConfig(
	ctx context.Context,
	_ driver.Namespace,
) (driver.ProfileConfig, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ProfileConfig{}, err
	}

	settings, err := client.ConfigGet(ctx, "slowlog-*").Result()
	if err != nil {
		return driver.ProfileConfig{}, translateError(err)
	}

	microseconds, _ := strconv.Atoi(settings[slowLogThreshold])

	return driver.ProfileConfig{
		// A negative threshold is the server recording nothing. Zero is not: it is recording
		// *everything*, which is on rather than off however unwise it is.
		Enabled:      microseconds >= 0,
		ThresholdMs:  microseconds / 1000,
		Configurable: true,
		Ready:        true,
		Detail: []driver.ProfileFact{
			{Label: slowLogThreshold, Value: settings[slowLogThreshold] + "µs"},
			{Label: slowLogLength, Value: settings[slowLogLength]},
		},
	}, nil
}

// ProfileStatements spells the changes as the commands a reader would type into the tab beside this
// one — `ProcessStatements`' arrangement.
//
// A `CONFIG SET` **outlives the connection that made it** and is classified destructive for exactly
// that reason wherever a reader types one; here the class is the change's own
// (`domain.ProfileHazard`), the setting being a profiler's rather than a server's shape.
func (*session) ProfileStatements(changes []driver.ProfileChange) []string {
	statements := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.Action {
		case domain.ProfileEnable:
			statements = append(statements,
				"CONFIG SET "+slowLogThreshold+" "+microsecondsOf(change.ThresholdMs))
		case domain.ProfileDisable:
			statements = append(statements, "CONFIG SET "+slowLogThreshold+" "+slowLogOff)
		case domain.ProfileReset:
			statements = append(statements, "SLOWLOG RESET")
		}
	}
	return statements
}

// ApplyProfileChanges runs exactly what ProfileStatements showed, in that order, stopping at the
// first failure. Every value goes as an **argument** rather than into text, so nothing here is
// quoted — the reason `monitor.go` gives for the same thing.
func (s *session) ApplyProfileChanges(
	ctx context.Context,
	_ driver.Namespace,
	changes []driver.ProfileChange,
) error {
	for _, change := range changes {
		if change.Action == domain.ProfileEnable && change.ThresholdMs < 0 {
			return domain.ValidationError{
				Field:   "threshold",
				Message: "is not a number of milliseconds",
			}
		}
	}

	client, err := s.client(driver.Namespace{})
	if err != nil {
		return err
	}

	for _, change := range changes {
		switch change.Action {
		case domain.ProfileEnable:
			err = client.ConfigSet(ctx, slowLogThreshold, microsecondsOf(change.ThresholdMs)).Err()
		case domain.ProfileDisable:
			err = client.ConfigSet(ctx, slowLogThreshold, slowLogOff).Err()
		case domain.ProfileReset:
			err = client.Do(ctx, "SLOWLOG", "RESET").Err()
		default:
			return domain.ValidationError{Field: "action", Message: "is not a change to profiling"}
		}
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

// microsecondsOf is the reader's milliseconds in the unit this server keeps the setting in. A
// threshold of zero is left as zero rather than being read as "no threshold": on this engine that is
// the reader asking for every command to be recorded, which is a thing they may mean.
func microsecondsOf(milliseconds int) string {
	return strconv.Itoa(milliseconds * 1000)
}
