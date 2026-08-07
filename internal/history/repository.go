// Package history keeps the statements a reader has run, twice over and for two different
// questions: Repository holds one entry per statement so that one can be found and run again, and
// Log holds every run of every statement in order so that a reader can watch what the app is doing.
package history

import "github.com/khanhsnd/vibe-db/internal/domain"

// Repository is the log, newest entry first. Append is its only write besides Clear, because what
// a log holds after one more statement is domain.Remember's rule rather than a caller's.
type Repository interface {
	List() ([]domain.HistoryEntry, error)
	Append(entry domain.HistoryEntry) error
	Clear() error
}
