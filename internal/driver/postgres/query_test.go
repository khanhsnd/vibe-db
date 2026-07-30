package postgres

import "testing"

func TestReturnsRows(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"select", "SELECT 1", true},
		{"cte", "WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"show", "SHOW search_path", true},
		{"values", "VALUES (1), (2)", true},
		{"insert returning", "INSERT INTO t (a) VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a = 1 RETURNING *", true},
		{"delete returning lower cased", "delete from t where a = 1 returning id", true},

		{"insert", "INSERT INTO t (a) VALUES (1)", false},
		{"update", "UPDATE t SET a = 1", false},
		{"delete", "DELETE FROM t", false},
		{"set", "SET search_path TO public", false},
		{"create", "CREATE TABLE t (a int)", false},
		{"returning inside a string is not a clause", "UPDATE t SET a = 'returning'", false},
		{"returning inside a subquery is not a clause",
			"UPDATE t SET a = (SELECT id FROM u RETURNING id)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := returnsRows(tt.in); got != tt.want {
				t.Errorf("returnsRows(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
