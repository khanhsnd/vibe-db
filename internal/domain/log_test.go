package domain

import "testing"

func noted(sql string) LogEntry {
	return LogEntry{Connection: "Local MySQL", SQL: sql}
}

func TestNotePutsTheNewestLastAndNumbersIt(t *testing.T) {
	log := Note(nil, noted("SELECT 1"))
	log = Note(log, noted("SELECT 2"))

	if len(log) != 2 {
		t.Fatalf("want 2 entries, got %d", len(log))
	}
	if log[0].SQL != "SELECT 1" || log[1].SQL != "SELECT 2" {
		t.Fatalf("want the newest last, got %q then %q", log[0].SQL, log[1].SQL)
	}
	if log[0].Seq != 1 || log[1].Seq != 2 {
		t.Fatalf("want 1 then 2, got %d then %d", log[0].Seq, log[1].Seq)
	}
}

func TestNoteKeepsARepeatedStatementTwice(t *testing.T) {
	log := Note(nil, noted("SELECT 1"))
	log = Note(log, noted("SELECT 1"))

	if len(log) != 2 {
		t.Fatalf("want the same statement kept twice, got %d entries", len(log))
	}
}

func TestNoteTrimsTheStatement(t *testing.T) {
	log := Note(nil, noted("\n  SELECT 1  \n"))

	if log[0].SQL != "SELECT 1" {
		t.Fatalf("want the statement trimmed, got %q", log[0].SQL)
	}
}

func TestNoteDropsTheOldestPastTheLimitAndKeepsCounting(t *testing.T) {
	var log []LogEntry
	for range LogLimit + 5 {
		log = Note(log, noted("SELECT 1"))
	}

	if len(log) != LogLimit {
		t.Fatalf("want %d entries, got %d", LogLimit, len(log))
	}
	if log[0].Seq != 6 {
		t.Fatalf("want the oldest kept to be 6, got %d", log[0].Seq)
	}
	if log[len(log)-1].Seq != int64(LogLimit+5) {
		t.Fatalf("want the newest to be %d, got %d", LogLimit+5, log[len(log)-1].Seq)
	}
}
