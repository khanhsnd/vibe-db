package app

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// journalCap is how many entries are kept. Five hundred is a few minutes of a busy reader and a few
// kilobytes: it is a **window**, not a ledger, and it goes when the app does.
const journalCap = 500

// objectEntry is one thing this app did about one object, and what came of it.
type objectEntry struct {
	When   time.Time
	Action string
	Bucket string
	Key    string
	// Outcome is `ok`, or the failure's own code — `FORBIDDEN`, `UNKNOWN_OBJECT` — which is what
	// makes this pane worth opening: a permission that is missing shows up here as the request that
	// was refused rather than as a dialog somebody has already dismissed.
	Outcome string
	Detail  string
	Elapsed time.Duration
}

// objectJournal is what **this app** did about the objects a reader has been looking at.
//
// It is deliberately not a service access log, and the pane says so. A service's own access log is a
// *bucket setting* that writes log objects into another bucket, usually is not turned on, and would
// have to be found, listed, downloaded and parsed — so a client that showed one would either be
// showing nothing or showing somebody else's gigabytes. What a reader actually wants when they open
// a Log tab is *what did this program just ask for, and what did it get back*, and that is a
// question only this side can answer.
//
// It lives in `internal/app` rather than in `internal/objectstore` for the same reason: it is a
// record of the **bound methods**, not of the protocol.
type objectJournal struct {
	mu      sync.Mutex
	entries []objectEntry
}

// note records one request. It takes the moment the request *started* rather than a duration,
// because every call site already has that and none of them should have to do the arithmetic.
func (j *objectJournal) note(action, bucket, key string, started time.Time, err error) {
	entry := objectEntry{
		When:    started,
		Action:  action,
		Bucket:  bucket,
		Key:     key,
		Outcome: "ok",
		Elapsed: time.Since(started),
	}
	if err != nil {
		entry.Outcome, entry.Detail = outcomeOf(err)
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = append(j.entries, entry)
	if len(j.entries) > journalCap {
		j.entries = j.entries[len(j.entries)-journalCap:]
	}
}

// outcomeOf is the failure as the pane draws it: the code the UI would have switched on, and the
// message the reader would have been shown. It reads the wire envelope the bound method already
// produced, so the pane and the dialog cannot disagree about what happened.
func outcomeOf(err error) (string, string) {
	var wrapped wireError
	if errors.As(err, &wrapped) {
		return string(wrapped.Code), wrapped.Message
	}
	return "INTERNAL", err.Error()
}

// forKey is what happened to one object, newest first. It is filtered here rather than kept in a map
// per key because a window of five hundred is quicker to walk than a map is to keep, and because a
// key the reader has since deleted still has a history worth reading.
func (j *objectJournal) forKey(bucket, key string, limit int) []objectEntry {
	j.mu.Lock()
	defer j.mu.Unlock()

	out := make([]objectEntry, 0, limit)
	for at := len(j.entries) - 1; at >= 0 && len(out) < limit; at-- {
		entry := j.entries[at]
		if entry.Bucket != bucket {
			continue
		}
		// A folder's own entry — a delete of a prefix, a download of one — belongs to every object
		// under it as well as to itself, which is what makes the pane useful on a row that was part
		// of something bigger.
		if entry.Key == key || (strings.HasSuffix(entry.Key, "/") && strings.HasPrefix(key, entry.Key)) {
			out = append(out, entry)
		}
	}
	return out
}
