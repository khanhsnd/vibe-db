package sqlkit

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestReviewReadsWhatEachStatementWillDo(t *testing.T) {
	tests := []struct {
		sql  string
		want domain.Hazard
	}{
		{"SELECT * FROM users", domain.HazardNone},
		{"EXPLAIN SELECT * FROM users", domain.HazardNone},
		{"INSERT INTO users (id) VALUES (1)", domain.HazardWrite},
		{"UPDATE users SET name = 'a' WHERE id = 1", domain.HazardWrite},
		{"DELETE FROM users WHERE id = 1", domain.HazardWrite},
		{"UPDATE users SET name = 'a'", domain.HazardUnfiltered},
		{"DELETE FROM users", domain.HazardUnfiltered},
		{"DROP TABLE users", domain.HazardDestructive},
		{"TRUNCATE TABLE users", domain.HazardDestructive},
		{"ALTER TABLE users DROP COLUMN name", domain.HazardDestructive},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			review := Postgres.Review(tt.sql)
			if len(review.Statements) != 1 {
				t.Fatalf("Review() = %d statements, want 1", len(review.Statements))
			}
			if got := review.Statements[0].Hazard; got != tt.want {
				t.Errorf("Hazard = %q, want %q", got, tt.want)
			}
		})
	}
}

// A WHERE inside a subquery narrows the subquery and not the delete around it, so it must not be
// what makes an unfiltered DELETE look filtered.
func TestReviewDoesNotReadASubquerysWhereAsTheStatements(t *testing.T) {
	review := Postgres.Review("UPDATE users SET tier = (SELECT t FROM plans WHERE id = 1)")

	if got := review.Statements[0].Hazard; got != domain.HazardUnfiltered {
		t.Errorf("Hazard = %q, want %q", got, domain.HazardUnfiltered)
	}
}

// Every statement is placed in the script it came from, which is what lets a warning point at it.
func TestReviewSaysWhereEachStatementSits(t *testing.T) {
	const script = "SELECT 1;\n\nDELETE FROM users;\n"
	review := Postgres.Review(script)

	if len(review.Statements) != 2 {
		t.Fatalf("Review() = %d statements, want 2", len(review.Statements))
	}
	for _, statement := range review.Statements {
		if script[statement.Start:statement.End] != statement.SQL {
			t.Errorf("statement %q does not sit at %d..%d", statement.SQL, statement.Start, statement.End)
		}
	}
	if review.Statements[1].Keyword != "delete" {
		t.Errorf("Keyword = %q, want delete", review.Statements[1].Keyword)
	}
}

func TestReviewReportsWhatTheLexerCanProve(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"an unclosed string", "SELECT 'a FROM t", "string is never closed"},
		{"an unclosed block comment", "SELECT 1 /* note", "block comment is never closed"},
		{"an unclosed name", `SELECT "a FROM t`, "quoted name is never closed"},
		{"an unclosed body", "CREATE FUNCTION f() AS $$ BEGIN", "$tag$ body is never closed"},
		{"an unclosed bracket", "SELECT (1, 2 FROM t", "bracket is never closed"},
		{"a stray bracket", "SELECT 1) FROM t", "nothing to close"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review := Postgres.Review(tt.sql)
			if len(review.Problems) == 0 {
				t.Fatalf("Review(%q) reported nothing, want %q", tt.sql, tt.want)
			}
			if !strings.Contains(review.Problems[0].Message, tt.want) {
				t.Errorf("message = %q, want it to mention %q", review.Problems[0].Message, tt.want)
			}
			at := review.Problems[0]
			if at.Start < 0 || at.End > len(tt.sql) || at.Start >= at.End {
				t.Errorf("problem sits at %d..%d, which is not inside a %d-byte script",
					at.Start, at.End, len(tt.sql))
			}
		})
	}
}

// Nothing is reported for a script that is simply correct, and nothing is reported for a `$1`
// placeholder either — it opens no body, however much it looks like one.
func TestReviewIsQuietWhenThereIsNothingToSay(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM users WHERE id = 1",
		"SELECT $1, $2 FROM t",
		"SELECT 'a;b', \"Mixed\" FROM t -- a note",
		"CREATE FUNCTION f() RETURNS int AS $$ SELECT 1; $$ LANGUAGE sql",
	} {
		if problems := Postgres.Review(sql).Problems; len(problems) != 0 {
			t.Errorf("Review(%q) = %+v, want nothing", sql, problems)
		}
	}
}

// A backslash is an escape on MySQL and not on PostgreSQL, so the same text is a runaway string
// on one engine and a whole one on the other. The review has to say what *this* engine will read.
func TestReviewReadsAnEscapeByTheEnginesRule(t *testing.T) {
	const sql = `SELECT 'C:\' FROM t`

	if problems := Postgres.Review(sql).Problems; len(problems) != 0 {
		t.Errorf("Postgres.Review() = %+v, want nothing: the string closes here", problems)
	}
	if problems := MySQL.Review(sql).Problems; len(problems) == 0 {
		t.Error("MySQL.Review() reported nothing, want an unclosed string")
	}
}
