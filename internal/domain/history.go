package domain

import (
	"strings"
	"time"
)

// HistoryLimit is how many statements the log keeps. It caps the file as much as the list: a
// reader looking for something they ran scrolls tens of entries and not thousands, and the oldest
// of them is the one they are least likely to want back.
const HistoryLimit = 500

// HistoryEntry is one Execute the reader asked for, kept so it can be found and run again. It
// carries the namespace it ran in because a statement without one may not run at all — the tables
// it names live in a database, and another one names none of them.
//
// Its outcome is the **last** run's: how many rows came back, how long it took, and the server's
// own wording if it was rejected. Runs is how many times the statement has been run, which is what
// keeps a statement kept once rather than a hundred times honest.
type HistoryEntry struct {
	ProfileID    string
	Database     string
	Schema       string
	SQL          string
	Rows         int
	RowsAffected int64
	ElapsedMs    int64
	ErrorCode    string
	Error        string
	Runs         int
	RanAt        time.Time
}

// Remember puts an entry at the head of the log: newest first, one entry per statement, and never
// more than HistoryLimit of them.
//
// A statement run again is **moved** to the head rather than added beside itself, its Runs counted
// up and its outcome replaced by the newer one. Five hundred copies of one SELECT is not a list
// anybody reads, and what a reader wants back is the statement rather than the hour they last ran
// it. The text is trimmed, so the entry is the statement and not the blank lines around it.
func Remember(log []HistoryEntry, entry HistoryEntry) []HistoryEntry {
	entry.SQL = strings.TrimSpace(entry.SQL)
	entry.Runs = 1

	kept := make([]HistoryEntry, 0, len(log)+1)
	kept = append(kept, entry)

	for _, past := range log {
		if past.sameStatement(entry) {
			kept[0].Runs = past.Runs + 1
			continue
		}
		kept = append(kept, past)
	}

	if len(kept) > HistoryLimit {
		kept = kept[:HistoryLimit]
	}
	return kept
}

// sameStatement is when two entries are one statement: the same text, on the same connection, in
// the same namespace. The same text against another database is another statement, that being
// where the tables it names live.
func (e HistoryEntry) sameStatement(other HistoryEntry) bool {
	return e.ProfileID == other.ProfileID &&
		e.Database == other.Database &&
		e.Schema == other.Schema &&
		e.SQL == other.SQL
}
