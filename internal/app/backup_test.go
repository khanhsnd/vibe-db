package app

import (
	"bufio"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/backup"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// The whole of what a narrowed restore is: a backup's own text, cut to the tables the reader
// picked, split by the engine's own reader. It is written here rather than in `internal/backup`
// because the halves belong to different packages and only their meeting is worth proving — the
// marker is only a good idea if the thing that reads the file cannot see it.
func TestANarrowedRestoreRunsTheChosenTablesStatementsAlone(t *testing.T) {
	out := &strings.Builder{}
	writer := bufio.NewWriter(out)

	// Written exactly as `writeBackup` writes it: a line naming the table, then its statements.
	for _, table := range []string{"users", "orders"} {
		if err := backup.MarkTable(writer, table); err != nil {
			t.Fatalf("mark: %v", err)
		}
		if err := writeStatements(writer, []string{
			"CREATE TABLE " + table + " (\n  id int\n)",
			"INSERT INTO " + table + " VALUES (1)",
		}); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := endTable(writer); err != nil {
			t.Fatalf("end: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	text := "-- vibe-db-backup {\"comment\":\"x\"}\n\n" + out.String()

	if whole := sqlkit.Postgres.Split(text); len(whole) != 4 {
		t.Fatalf("the whole file split into %d statements, want 4: %q", len(whole), whole)
	}

	narrowed := sqlkit.Postgres.Split(backup.Only(text, []string{"orders"}))
	if len(narrowed) != 2 {
		t.Fatalf("the narrowed file split into %d statements, want 2: %q", len(narrowed), narrowed)
	}
	for _, statement := range narrowed {
		if !strings.Contains(statement, "orders") {
			t.Fatalf("a statement of no chosen table survived: %q", statement)
		}
		if strings.Contains(statement, "INSERT INTO users") ||
			strings.Contains(statement, "CREATE TABLE users") {
			t.Fatalf("an unchosen table's statement survived: %q", statement)
		}
	}

	// A marker rides along as the leading comment of the statement under it, which is what the
	// header line has always done and what makes both of them free: the server is handed a comment
	// and a statement, and reads the comment as a comment.
	if !strings.HasPrefix(narrowed[0], "-- vibe-db-backup") {
		t.Fatalf("the first statement lost the lines above it: %q", narrowed[0])
	}
}
