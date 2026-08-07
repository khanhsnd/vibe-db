package app

import (
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/history"
	"github.com/khanhsnd/vibe-db/internal/session"
)

// noting is a recorder pointed at no session at all, which is the case the log has to survive:
// a connection closed under a run still leaves a line saying what ran.
func noting(log *history.Log) statementLog {
	sessions := session.NewManager(engines())
	return newStatementLog(log, sessions, "gone", Namespace{Database: "shop", Schema: "public"})
}

func TestStatementLogKeepsWhereAStatementRan(t *testing.T) {
	log := history.NewLog()
	noting(log).note("SELECT 1", 4*time.Millisecond, nil)

	entries := log.Since(0)
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.SQL != "SELECT 1" || entry.Database != "shop" || entry.Schema != "public" {
		t.Fatalf("want the statement and its namespace, got %+v", entry)
	}
	if entry.ElapsedMs != 4 || entry.Error != "" {
		t.Fatalf("want 4 ms and no error, got %d ms and %q", entry.ElapsedMs, entry.Error)
	}
}

func TestStatementLogKeepsTheServerWording(t *testing.T) {
	log := history.NewLog()
	refused := domain.StatementError{Code: "42P01", Message: `relation "nope" does not exist`}
	noting(log).note("SELECT * FROM nope", time.Millisecond, refused)

	entries := log.Since(0)
	if entries[0].Error != `relation "nope" does not exist` {
		t.Fatalf("want the server's own wording, got %q", entries[0].Error)
	}
}

func TestListLogAnswersOnlyWithWhatTheViewerHasNotRead(t *testing.T) {
	log := history.NewLog()
	recorder := noting(log)
	recorder.note("SELECT 1", 0, nil)
	recorder.note("SELECT 2", 0, nil)

	api := NewQueryAPI(session.NewManager(engines()), nil, log)
	if got := api.ListLog(0); len(got) != 2 {
		t.Fatalf("want both entries, got %d", len(got))
	}

	fresh := api.ListLog(1)
	if len(fresh) != 1 || fresh[0].SQL != "SELECT 2" {
		t.Fatalf("want only the second entry, got %+v", fresh)
	}

	api.ClearLog()
	if got := api.ListLog(0); len(got) != 0 {
		t.Fatalf("want an empty log, got %d entries", len(got))
	}
}
