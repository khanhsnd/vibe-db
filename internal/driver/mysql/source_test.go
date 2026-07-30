package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestParseSource(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		database string
		table    string
		ok       bool
	}{
		{name: "bare select", in: "SELECT * FROM users", table: "users", ok: true},
		{name: "column list", in: "SELECT id, name FROM users", table: "users", ok: true},
		{name: "where clause", in: "SELECT * FROM users WHERE id = 1", table: "users", ok: true},
		{name: "order and limit", in: "SELECT * FROM users ORDER BY id LIMIT 10", table: "users", ok: true},
		{name: "bare alias", in: "SELECT u.id FROM users u WHERE u.id = 1", table: "users", ok: true},
		{name: "as alias", in: "SELECT * FROM users AS u", table: "users", ok: true},
		{name: "qualified", in: "SELECT * FROM shop.users", database: "shop", table: "users", ok: true},
		{name: "quoted", in: "SELECT * FROM `shop`.`users`", database: "shop", table: "users", ok: true},
		{name: "function in projection", in: "SELECT UPPER(name), id FROM users", table: "users", ok: true},
		{name: "string holding a keyword", in: "SELECT 'from x join y' AS a, id FROM users", table: "users", ok: true},
		{name: "leading comment", in: "-- pick\nSELECT * FROM users", table: "users", ok: true},
		{name: "lower cased", in: "select * from users where id = 1", table: "users", ok: true},

		{name: "join", in: "SELECT * FROM users u JOIN orders o ON o.user_id = u.id"},
		{name: "implicit join", in: "SELECT * FROM users, orders"},
		{name: "union", in: "SELECT * FROM users UNION SELECT * FROM staff"},
		{name: "group by", in: "SELECT country, COUNT(*) FROM users GROUP BY country"},
		{name: "distinct", in: "SELECT DISTINCT country FROM users"},
		{name: "derived table", in: "SELECT * FROM (SELECT * FROM users) t"},
		{name: "subquery in projection wins no from", in: "SELECT (SELECT 1 FROM users)"},
		{name: "explain", in: "EXPLAIN SELECT * FROM users"},
		{name: "show", in: "SHOW TABLES"},
		{name: "no from", in: "SELECT 1"},
		{name: "insert", in: "INSERT INTO users VALUES (1)"},
		{name: "empty", in: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSource(tt.in)

			if ok != tt.ok {
				t.Fatalf("parseSource(%q) ok = %v, want %v (got %+v)", tt.in, ok, tt.ok, got)
			}
			if !tt.ok {
				return
			}
			if got.database != tt.database || got.table != tt.table {
				t.Errorf("parseSource(%q) = %+v, want {%q %q}", tt.in, got, tt.database, tt.table)
			}
		})
	}
}

func TestMergeSchemaNeedsEveryKeyColumnExactlyOnce(t *testing.T) {
	stored := []driver.Column{
		{Name: "id", FullType: "int unsigned", PrimaryKey: true},
		{Name: "name", FullType: "varchar(80)", Nullable: true},
	}

	tests := []struct {
		name   string
		result []string
		want   bool
	}{
		{name: "every column", result: []string{"id", "name"}, want: true},
		{name: "key alone", result: []string{"id"}, want: true},
		{name: "case insensitive", result: []string{"ID"}, want: true},
		{name: "key aliased away", result: []string{"name"}},
		{name: "key repeated", result: []string{"id", "id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]driver.Column, 0, len(tt.result))
			for _, name := range tt.result {
				result = append(result, driver.Column{Name: name, Type: "INT"})
			}

			if got := mergeSchema(result, stored, []string{"id"}); got != tt.want {
				t.Fatalf("mergeSchema = %v, want %v", got, tt.want)
			}
			if !tt.want {
				return
			}
			if !result[0].PrimaryKey || result[0].FullType != "int unsigned" {
				t.Errorf("stored facts not copied onto %+v", result[0])
			}
		})
	}
}
