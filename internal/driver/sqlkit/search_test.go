package sqlkit

import (
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

var searchColumns = []driver.Column{{Name: "id"}, {Name: "name"}}

func match(mode domain.SearchMode, text string, caseless bool) domain.TextMatch {
	return domain.TextMatch{Text: text, Mode: mode, CaseInsensitive: caseless}
}

func TestMatchAnyReadsEveryColumnAsText(t *testing.T) {
	clause, err := MySQL.MatchAny(match(domain.MatchContains, "khanh", false), searchColumns)
	if err != nil {
		t.Fatalf("MatchAny() error = %v", err)
	}

	want := " WHERE (CAST(`id` AS CHAR) LIKE CAST('%khanh%' AS BINARY)" +
		" OR CAST(`name` AS CHAR) LIKE CAST('%khanh%' AS BINARY))"
	if clause != want {
		t.Errorf("MatchAny() = %q, want %q", clause, want)
	}
}

// The read a reader opens is the count's own clause with the rows in front of it instead of a count
// of them, which is what stops the number and the tab from being about different rows.
func TestSelectMatchingIsTheCountsOwnClause(t *testing.T) {
	where, err := Postgres.MatchAny(match(domain.MatchContains, "khanh", false), searchColumns[1:])
	if err != nil {
		t.Fatalf("MatchAny() error = %v", err)
	}

	got := Postgres.SelectMatching(Postgres.Qualify("public", "people"), where)
	want := `SELECT * FROM "public"."people" WHERE (CAST("name" AS TEXT) LIKE '%khanh%')`
	if got != want {
		t.Errorf("SelectMatching() = %q, want %q", got, want)
	}
}

func TestMatchAnySpellsEveryMode(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		match   domain.TextMatch
		want    string
	}{
		{
			"mysql insists on case by comparing bytes",
			MySQL,
			match(domain.MatchContains, "Khanh", false),
			" WHERE (CAST(`name` AS CHAR) LIKE CAST('%Khanh%' AS BINARY))",
		},
		{
			"a caseless pattern folds both sides on either engine",
			MySQL,
			match(domain.MatchContains, "Khanh", true),
			" WHERE (LOWER(CAST(`name` AS CHAR)) LIKE LOWER('%Khanh%'))",
		},
		{
			"postgres needs nothing to insist on case",
			Postgres,
			match(domain.MatchPrefix, "kh", false),
			` WHERE (CAST("name" AS TEXT) LIKE 'kh%')`,
		},
		{
			"postgres folds a caseless pattern the same way",
			Postgres,
			match(domain.MatchPrefix, "kh", true),
			` WHERE (LOWER(CAST("name" AS TEXT)) LIKE LOWER('kh%'))`,
		},
		{
			"a whole word is an expression, bounded the engine's own way",
			MySQL,
			match(domain.MatchWord, "khanh", true),
			" WHERE (REGEXP_LIKE(CAST(`name` AS CHAR), '\\\\bkhanh\\\\b', 'i'))",
		},
		{
			"postgres bounds a word with its own token",
			Postgres,
			match(domain.MatchWord, "khanh", false),
			` WHERE (CAST("name" AS TEXT) ~ '\ykhanh\y')`,
		},
		{
			"an expression the reader wrote travels verbatim",
			MySQL,
			match(domain.MatchRegex, "^kh.*h$", false),
			" WHERE (REGEXP_LIKE(CAST(`name` AS CHAR), '^kh.*h$', 'c'))",
		},
		{
			"postgres carries the sensitivity in the operator",
			Postgres,
			match(domain.MatchRegex, "^kh", true),
			` WHERE (CAST("name" AS TEXT) ~* '^kh')`,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			clause, err := test.dialect.MatchAny(test.match, []driver.Column{{Name: "name"}})
			if err != nil {
				t.Fatalf("MatchAny() error = %v", err)
			}
			if clause != test.want {
				t.Errorf("MatchAny() = %q, want %q", clause, test.want)
			}
		})
	}
}

// A pattern is the reader's own: `%` is theirs to type, and `\` is what makes one a per cent sign —
// which is LIKE's own escape and therefore the one the box under the field promises.
func TestMatchAnyLeavesAPatternTheReadersOwn(t *testing.T) {
	clause, err := Postgres.MatchAny(match(domain.MatchContains, `10\%`, false), searchColumns[1:])
	if err != nil {
		t.Fatalf("MatchAny() error = %v", err)
	}

	want := ` WHERE (CAST("name" AS TEXT) LIKE '%10\%%')`
	if clause != want {
		t.Errorf("MatchAny() = %q, want %q", clause, want)
	}
}

func TestMatchAnyRefusesWhatItCannotSpell(t *testing.T) {
	cases := []struct {
		name  string
		match domain.TextMatch
	}{
		{"nothing to look for", match(domain.MatchContains, "", false)},
		{"a mode this side does not know", match("fuzzy", "khanh", false)},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := MySQL.MatchAny(test.match, searchColumns); err == nil {
				t.Fatal("MatchAny() error = nil, want a rejection")
			} else if !errors.Is(err, domain.ErrInvalidProfile) {
				t.Errorf("MatchAny() error = %v, want a validation error", err)
			}
		})
	}
}

// A table with no columns holds no text, and the clause says so rather than being empty — an empty
// one would count every row of the table as a match.
func TestMatchAnyWithoutColumnsMatchesNothing(t *testing.T) {
	clause, err := MySQL.MatchAny(match(domain.MatchContains, "khanh", false), nil)
	if err != nil {
		t.Fatalf("MatchAny() error = %v", err)
	}
	if clause != " WHERE 1 = 0" {
		t.Errorf("MatchAny() = %q, want a clause nothing satisfies", clause)
	}
}
