package domain

import (
	"strconv"
	"testing"
)

func entry(profileID, database, sql string) HistoryEntry {
	return HistoryEntry{ProfileID: profileID, Database: database, SQL: sql}
}

func TestRememberPutsTheNewestFirst(t *testing.T) {
	log := Remember(nil, entry("a", "shop", "SELECT 1"))
	log = Remember(log, entry("a", "shop", "SELECT 2"))

	if len(log) != 2 {
		t.Fatalf("log = %d entries, want 2", len(log))
	}
	if log[0].SQL != "SELECT 2" {
		t.Errorf("head = %q, want the statement just run", log[0].SQL)
	}
}

func TestRememberMovesARepeatedStatementAndCountsIt(t *testing.T) {
	log := Remember(nil, entry("a", "shop", "SELECT 1"))
	log = Remember(log, entry("a", "shop", "SELECT 2"))
	log = Remember(log, entry("a", "shop", "SELECT 1"))

	if len(log) != 2 {
		t.Fatalf("log = %d entries, want the repeat moved rather than added", len(log))
	}
	if log[0].SQL != "SELECT 1" || log[0].Runs != 2 {
		t.Errorf("head = %q run %d times, want SELECT 1 run twice", log[0].SQL, log[0].Runs)
	}
}

func TestRememberTellsNamespacesApart(t *testing.T) {
	log := Remember(nil, entry("a", "shop", "SELECT 1"))
	log = Remember(log, entry("a", "warehouse", "SELECT 1"))
	log = Remember(log, entry("b", "shop", "SELECT 1"))

	if len(log) != 3 {
		t.Errorf("log = %d entries, want one per connection and database", len(log))
	}
}

func TestRememberTrimsTheStatement(t *testing.T) {
	log := Remember(nil, entry("a", "shop", "\n  SELECT 1  \n"))
	log = Remember(log, entry("a", "shop", "SELECT 1"))

	if len(log) != 1 {
		t.Fatalf("log = %d entries, want the same statement recognised once", len(log))
	}
	if log[0].SQL != "SELECT 1" {
		t.Errorf("SQL = %q, want it trimmed", log[0].SQL)
	}
}

func TestRememberDropsTheOldestPastTheLimit(t *testing.T) {
	var log []HistoryEntry
	for at := 0; at <= HistoryLimit; at++ {
		log = Remember(log, entry("a", "shop", "SELECT "+strconv.Itoa(at)))
	}

	if len(log) != HistoryLimit {
		t.Errorf("log = %d entries, want it capped at %d", len(log), HistoryLimit)
	}
}
