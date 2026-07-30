package sqlkit

import "testing"

func TestParseSource(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		namespace string
		table     string
		ok        bool
	}{
		{name: "bare select", in: "SELECT * FROM users", table: "users", ok: true},
		{name: "column list", in: "SELECT id, name FROM users", table: "users", ok: true},
		{name: "where clause", in: "SELECT * FROM users WHERE id = 1", table: "users", ok: true},
		{name: "order and limit", in: "SELECT * FROM users ORDER BY id LIMIT 10", table: "users", ok: true},
		{name: "bare alias", in: "SELECT u.id FROM users u WHERE u.id = 1", table: "users", ok: true},
		{name: "as alias", in: "SELECT * FROM users AS u", table: "users", ok: true},
		{name: "qualified", in: "SELECT * FROM shop.users", namespace: "shop", table: "users", ok: true},
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

	for _, dialect := range []struct {
		name string
		Dialect
	}{{"mysql", MySQL}, {"postgres", Postgres}} {
		for _, tt := range tests {
			t.Run(dialect.name+"/"+tt.name, func(t *testing.T) {
				got, ok := dialect.ParseSource(tt.in)

				if ok != tt.ok {
					t.Fatalf("ParseSource(%q) ok = %v, want %v (got %+v)", tt.in, ok, tt.ok, got)
				}
				if !tt.ok {
					return
				}
				if got.Namespace != tt.namespace || got.Table != tt.table {
					t.Errorf("ParseSource(%q) = %+v, want {%q %q}", tt.in, got, tt.namespace, tt.table)
				}
			})
		}
	}
}

// Each engine quotes an identifier its own way, and the other engine's quote is a string
// delimiter rather than a second spelling of the same thing.
func TestParseSourceReadsQuotedNamesPerDialect(t *testing.T) {
	tests := []struct {
		name      string
		dialect   Dialect
		in        string
		namespace string
		table     string
		ok        bool
	}{
		{
			name: "mysql backticks", dialect: MySQL, in: "SELECT * FROM `shop`.`users`",
			namespace: "shop", table: "users", ok: true,
		},
		{
			name: "postgres double quotes", dialect: Postgres, in: `SELECT * FROM "shop"."users"`,
			namespace: "shop", table: "users", ok: true,
		},
		{
			name: "postgres offset without limit", dialect: Postgres, in: "SELECT * FROM users OFFSET 10",
			table: "users", ok: true,
		},
		{name: "mysql cannot read a double-quoted name", dialect: MySQL, in: `SELECT * FROM "users"`},
		{name: "postgres cannot read a backquoted name", dialect: Postgres, in: "SELECT * FROM `users`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.dialect.ParseSource(tt.in)

			if ok != tt.ok {
				t.Fatalf("ParseSource(%q) ok = %v, want %v (got %+v)", tt.in, ok, tt.ok, got)
			}
			if tt.ok && (got.Namespace != tt.namespace || got.Table != tt.table) {
				t.Errorf("ParseSource(%q) = %+v, want {%q %q}", tt.in, got, tt.namespace, tt.table)
			}
		})
	}
}

func TestQuoteIdentEscapesTheDialectQuote(t *testing.T) {
	if got, want := MySQL.QuoteIdent("us`er"), "`us``er`"; got != want {
		t.Errorf("MySQL.QuoteIdent = %q, want %q", got, want)
	}
	if got, want := Postgres.QuoteIdent(`us"er`), `"us""er"`; got != want {
		t.Errorf("Postgres.QuoteIdent = %q, want %q", got, want)
	}
}

func TestQualifyNamesTheNamespaceThenTheTable(t *testing.T) {
	if got, want := Postgres.Qualify("public", "users"), `"public"."users"`; got != want {
		t.Errorf("Qualify = %q, want %q", got, want)
	}
}
