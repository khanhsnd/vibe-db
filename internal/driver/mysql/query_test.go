package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

func TestRowReturningCoversReadStatements(t *testing.T) {
	rowReturners := []string{"SELECT 1", "SHOW TABLES", "EXPLAIN SELECT 1", "DESCRIBE t"}
	commands := []string{"INSERT INTO t VALUES (1)", "UPDATE t SET a = 1", "DELETE FROM t", "SET @a = 1"}

	for _, statement := range rowReturners {
		if !rowReturning[sqlkit.LeadingKeyword(statement)] {
			t.Errorf("%q should be read through Query", statement)
		}
	}
	for _, statement := range commands {
		if rowReturning[sqlkit.LeadingKeyword(statement)] {
			t.Errorf("%q should be run through Exec so RowsAffected survives", statement)
		}
	}
}
