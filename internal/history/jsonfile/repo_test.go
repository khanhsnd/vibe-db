package jsonfile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "nested", "history.json"))
}

func ran(sql string, at time.Time) domain.HistoryEntry {
	return domain.HistoryEntry{ProfileID: "a", Database: "shop", SQL: sql, RanAt: at}
}

func TestListOnMissingFileReturnsEmpty(t *testing.T) {
	entries, err := newRepo(t).List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %d entries, want 0", len(entries))
	}
}

func TestAppendKeepsTheNewestFirstAcrossReads(t *testing.T) {
	repo := newRepo(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := repo.Append(ran("SELECT 1", now)); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := repo.Append(ran("SELECT 2", now.Add(time.Minute))); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	entries, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List() = %d entries, want 2", len(entries))
	}
	if entries[0].SQL != "SELECT 2" {
		t.Errorf("head = %q, want the statement run last", entries[0].SQL)
	}
	if !entries[0].RanAt.Equal(now.Add(time.Minute)) {
		t.Errorf("RanAt = %v, want the time it ran to survive the file", entries[0].RanAt)
	}
}

func TestClearEmptiesTheLog(t *testing.T) {
	repo := newRepo(t)

	if err := repo.Append(ran("SELECT 1", time.Now())); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := repo.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	entries, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %d entries after Clear, want 0", len(entries))
	}
}
