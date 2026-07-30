package mysql

import "testing"

func TestLeadingKeyword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain select", "SELECT 1", "select"},
		{"lower cased", "select 1", "select"},
		{"leading whitespace", "\n\t  SELECT 1", "select"},
		{"parenthesised select", "(SELECT 1) UNION (SELECT 2)", "select"},
		{"line comment first", "-- pick one\nSELECT 1", "select"},
		{"hash comment first", "# pick one\nSELECT 1", "select"},
		{"block comment first", "/* pick one */ SELECT 1", "select"},
		{"stacked comments", "-- a\n/* b */\nUPDATE t SET a = 1", "update"},
		{"insert", "INSERT INTO t VALUES (1)", "insert"},
		{"cte", "WITH x AS (SELECT 1) SELECT * FROM x", "with"},
		{"empty", "   ", ""},
		{"unterminated block comment", "/* never closed", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := leadingKeyword(tt.in); got != tt.want {
				t.Errorf("leadingKeyword(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRowReturningCoversReadStatements(t *testing.T) {
	rowReturners := []string{"SELECT 1", "SHOW TABLES", "EXPLAIN SELECT 1", "DESCRIBE t"}
	commands := []string{"INSERT INTO t VALUES (1)", "UPDATE t SET a = 1", "DELETE FROM t", "SET @a = 1"}

	for _, statement := range rowReturners {
		if !rowReturning[leadingKeyword(statement)] {
			t.Errorf("%q should be read through Query", statement)
		}
	}
	for _, statement := range commands {
		if rowReturning[leadingKeyword(statement)] {
			t.Errorf("%q should be run through Exec so RowsAffected survives", statement)
		}
	}
}
