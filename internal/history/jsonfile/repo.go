package jsonfile

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	fileName = "history.json"
	version  = 1
)

// record is an entry as the file spells it. The time is RFC 3339, by way of time.Time's own JSON,
// so the log stays readable by anything that reads JSON.
type record struct {
	ProfileID    string    `json:"profileId"`
	Database     string    `json:"database"`
	Schema       string    `json:"schema"`
	SQL          string    `json:"sql"`
	Rows         int       `json:"rows"`
	RowsAffected int64     `json:"rowsAffected"`
	ElapsedMs    int64     `json:"elapsedMs"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	Error        string    `json:"error,omitempty"`
	Runs         int       `json:"runs"`
	RanAt        time.Time `json:"ranAt"`
}

type document struct {
	Version int      `json:"version"`
	Entries []record `json:"entries"`
}

type Repo struct {
	file *jsonstore.File[document]
}

func New(path string) *Repo {
	return &Repo{file: jsonstore.New[document](path)}
}

func DefaultPath() (string, error) {
	return jsonstore.Path(fileName)
}

func (r *Repo) List() ([]domain.HistoryEntry, error) {
	doc, err := r.file.Read()
	if err != nil {
		return nil, err
	}
	return toDomainAll(doc.Entries), nil
}

// Append hands the whole log to domain.Remember rather than pushing onto it here, so where an
// entry lands, when it replaces one and how many are kept are decided in one place.
func (r *Repo) Append(entry domain.HistoryEntry) error {
	return r.file.Update(func(doc *document) error {
		doc.Version = version
		doc.Entries = fromDomainAll(domain.Remember(toDomainAll(doc.Entries), entry))
		return nil
	})
}

func (r *Repo) Clear() error {
	return r.file.Update(func(doc *document) error {
		doc.Version = version
		doc.Entries = nil
		return nil
	})
}

func toDomainAll(records []record) []domain.HistoryEntry {
	entries := make([]domain.HistoryEntry, 0, len(records))
	for _, rec := range records {
		entries = append(entries, domain.HistoryEntry{
			ProfileID:    rec.ProfileID,
			Database:     rec.Database,
			Schema:       rec.Schema,
			SQL:          rec.SQL,
			Rows:         rec.Rows,
			RowsAffected: rec.RowsAffected,
			ElapsedMs:    rec.ElapsedMs,
			ErrorCode:    rec.ErrorCode,
			Error:        rec.Error,
			Runs:         rec.Runs,
			RanAt:        rec.RanAt,
		})
	}
	return entries
}

func fromDomainAll(entries []domain.HistoryEntry) []record {
	records := make([]record, 0, len(entries))
	for _, entry := range entries {
		records = append(records, record{
			ProfileID:    entry.ProfileID,
			Database:     entry.Database,
			Schema:       entry.Schema,
			SQL:          entry.SQL,
			Rows:         entry.Rows,
			RowsAffected: entry.RowsAffected,
			ElapsedMs:    entry.ElapsedMs,
			ErrorCode:    entry.ErrorCode,
			Error:        entry.Error,
			Runs:         entry.Runs,
			RanAt:        entry.RanAt,
		})
	}
	return records
}
