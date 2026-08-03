package sqlkit

import (
	"strings"
	"testing"
)

func joined(statements []string) string {
	return strings.Join(statements, " | ")
}

func TestSplitCutsOnTheSemicolonsThatEndAStatement(t *testing.T) {
	script := "SELECT 1;\nSELECT 2;\n"

	if got, want := joined(Split(script, Postgres)), "SELECT 1 | SELECT 2"; got != want {
		t.Errorf("Split = %s, want %s", got, want)
	}
}

// A semicolon inside a string, a comment or a name ends nothing.
func TestSplitIgnoresASemicolonThatIsInsideSomething(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		dialect Dialect
		want    string
	}{
		{
			name:    "string literal",
			script:  "SELECT 'a;b'; SELECT 2",
			dialect: Postgres,
			want:    "SELECT 'a;b' | SELECT 2",
		},
		{
			name:    "line comment",
			script:  "SELECT 1 -- ; not this\n; SELECT 2",
			dialect: Postgres,
			want:    "SELECT 1 -- ; not this | SELECT 2",
		},
		{
			name:    "block comment",
			script:  "SELECT 1 /* ; */; SELECT 2",
			dialect: Postgres,
			want:    "SELECT 1 /* ; */ | SELECT 2",
		},
		{
			name:    "quoted identifier",
			script:  "SELECT \"a;b\" FROM t; SELECT 2",
			dialect: Postgres,
			want:    "SELECT \"a;b\" FROM t | SELECT 2",
		},
		{
			name:    "backtick identifier",
			script:  "SELECT `a;b` FROM t; SELECT 2",
			dialect: MySQL,
			want:    "SELECT `a;b` FROM t | SELECT 2",
		},
		{
			name:    "hash comment",
			script:  "SELECT 1 # ; not this\n; SELECT 2",
			dialect: MySQL,
			want:    "SELECT 1 # ; not this | SELECT 2",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := joined(Split(test.script, test.dialect)); got != test.want {
				t.Errorf("Split = %s, want %s", got, test.want)
			}
		})
	}
}

// A function body is where a PostgreSQL statement carries semicolons of its own.
func TestSplitKeepsADollarQuotedBodyWhole(t *testing.T) {
	script := "CREATE FUNCTION f() RETURNS int AS $$ BEGIN; SELECT 1; END; $$ LANGUAGE plpgsql;\nSELECT 2"

	got := Split(script, Postgres)
	if len(got) != 2 {
		t.Fatalf("got %d statements, want 2:\n%s", len(got), joined(got))
	}
	if !strings.HasPrefix(got[0], "CREATE FUNCTION") || !strings.HasSuffix(got[0], "plpgsql") {
		t.Errorf("the body did not survive: %s", got[0])
	}
}

// PostgreSQL reads no escape into a backslash, so a path that ends in one closes its string.
func TestSplitReadsABackslashByTheEnginesRule(t *testing.T) {
	script := `SELECT 'C:\'; SELECT 2`

	if got, want := len(Split(script, Postgres)), 2; got != want {
		t.Errorf("Postgres: got %d statements, want %d", got, want)
	}
	// MySQL escapes the closing quote instead, so the whole of it is one runaway statement.
	if got, want := len(Split(script, MySQL)), 1; got != want {
		t.Errorf("MySQL: got %d statements, want %d", got, want)
	}
}

// A `$1` is a placeholder and not a quote, so what follows it still ends where it should.
func TestSplitLeavesAPlaceholderAlone(t *testing.T) {
	script := "SELECT $1, $2; SELECT 3"

	if got, want := joined(Split(script, Postgres)), "SELECT $1, $2 | SELECT 3"; got != want {
		t.Errorf("Split = %s, want %s", got, want)
	}
}

func TestSplitDropsWhatIsNotAStatement(t *testing.T) {
	cases := map[string]string{
		"empty":              "",
		"whitespace":         "  \n\t ",
		"semicolons":         ";;;",
		"a trailing comment": "SELECT 1;\n-- done\n",
	}

	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			got := Split(script, Postgres)
			if name == "a trailing comment" {
				if len(got) != 1 || got[0] != "SELECT 1" {
					t.Errorf("Split = %v, want [SELECT 1]", got)
				}
				return
			}
			if len(got) != 0 {
				t.Errorf("Split = %v, want nothing", got)
			}
		})
	}
}
