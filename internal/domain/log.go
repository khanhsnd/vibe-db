package domain

import (
	"strings"
	"time"
)

// LogLimit is how many statements the log holds. It is a window on what this app has just been
// doing rather than a record of everything it ever did — that is the history, which is kept on
// disk and holds one entry per statement — so what falls out of it is the oldest.
const LogLimit = 1000

// LogEntry is one statement as it was sent, kept so a reader can watch what the app is doing.
//
// It is never merged with another, which is the whole of what separates it from a HistoryEntry: a
// statement run twice is two entries, because what a log answers is *what happened and when*, and
// the same statement a minute apart is exactly what somebody reading one is looking for.
//
// Connection and Engine are copied rather than named by an id, because a log outlives both the tab
// the statement was typed in and the connection it ran on, and a line naming neither says nothing.
type LogEntry struct {
	// Seq numbers the entries so a reader already holding some can ask for the rest. It counts up
	// for as long as the app runs and is never reused: entries fall off the front rather than
	// being renumbered, so a gap in the numbers is a reader who looked away too long and the log
	// says so rather than pretending otherwise.
	Seq        int64
	RanAt      time.Time
	Connection string
	Engine     Engine
	Database   string
	Schema     string
	SQL        string
	ElapsedMs  int64
	// Error is the server's own wording, and is empty for a statement that ran.
	Error string
}

// Note puts an entry at the end of the log and numbers it one past the last. Past LogLimit the
// oldest goes: a log is a window, and the entry a reader is least likely to still want is the one
// furthest behind them.
func Note(log []LogEntry, entry LogEntry) []LogEntry {
	entry.SQL = strings.TrimSpace(entry.SQL)
	entry.Seq = 1
	if last := len(log) - 1; last >= 0 {
		entry.Seq = log[last].Seq + 1
	}

	log = append(log, entry)
	if len(log) > LogLimit {
		log = log[len(log)-LogLimit:]
	}
	return log
}
