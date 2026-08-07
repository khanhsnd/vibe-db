package history

import (
	"sync"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Log is every statement this app has run, in the order it ran them. It is memory alone and goes
// with the window, which is the difference between it and Repository: a Repository keeps a
// statement so it can be found and run again, and this says what has just happened — a question
// nobody asks of a window that has closed.
//
// It is written from whichever goroutine ran the statement and read from the one answering the
// viewer, so it holds a lock of its own.
type Log struct {
	mu      sync.Mutex
	entries []domain.LogEntry
}

func NewLog() *Log {
	return &Log{}
}

// Note keeps one statement. What the log holds afterwards is domain.Note's rule rather than a
// caller's, exactly as Append leaves domain.Remember's to it.
func (l *Log) Note(entry domain.LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = domain.Note(l.entries, entry)
}

// Since is every entry numbered past seq, which is how a viewer already holding some asks for the
// rest rather than being handed the whole log again. A seq below the oldest entry still kept
// answers with everything there is — see domain.LogEntry.Seq for why the gap is left visible.
func (l *Log) Since(seq int64) []domain.LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]domain.LogEntry, 0, len(l.entries))
	for _, entry := range l.entries {
		if entry.Seq > seq {
			out = append(out, entry)
		}
	}
	return out
}

// Clear forgets every statement, numbering included: what a reader who has just emptied a log
// wants next is line 1, not line 4 032.
func (l *Log) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = nil
}
