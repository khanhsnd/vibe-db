package sqlkit

import (
	"strings"
	"testing"
)

func TestFormatLaysAStatementOut(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a clause to a line, its items indented under it",
			in:   "select id, name from users where status='active' order by id desc limit 10",
			want: strings.Join([]string{
				"SELECT",
				"\tid,",
				"\tname",
				"FROM",
				"\tusers",
				"WHERE",
				"\tstatus = 'active'",
				"ORDER BY",
				"\tid DESC",
				"LIMIT 10",
			}, "\n"),
		},
		{
			name: "a conjunction hangs under the condition above it",
			in:   "SELECT 1 FROM t WHERE a = 1 AND b = 2 OR c = 3",
			want: strings.Join([]string{
				"SELECT",
				"\t1",
				"FROM",
				"\tt",
				"WHERE",
				"\ta = 1",
				"\tAND b = 2",
				"\tOR c = 3",
			}, "\n"),
		},
		{
			name: "a join hangs under the FROM above it",
			in:   "select * from a left join b on b.a_id = a.id",
			want: strings.Join([]string{
				"SELECT",
				"\t*",
				"FROM",
				"\ta",
				"\tLEFT JOIN b ON b.a_id = a.id",
			}, "\n"),
		},
		{
			name: "a short parenthesis is punctuation",
			in:   "select count(*), coalesce(a,b) from t where id in (1,2,3)",
			want: strings.Join([]string{
				"SELECT",
				"\tCOUNT(*),",
				"\tCOALESCE(a, b)",
				"FROM",
				"\tt",
				"WHERE",
				"\tid IN (1, 2, 3)",
			}, "\n"),
		},
		{
			name: "a subquery is a block whatever its length",
			in:   "select * from t where id in (select id from u)",
			want: strings.Join([]string{
				"SELECT",
				"\t*",
				"FROM",
				"\tt",
				"WHERE",
				"\tid IN (",
				"\t\tSELECT",
				"\t\t\tid",
				"\t\tFROM",
				"\t\t\tu",
				"\t)",
			}, "\n"),
		},
		{
			name: "an update keeps its target on one line",
			in:   "update shop.users set name='a', email='b' where id=1",
			want: strings.Join([]string{
				"UPDATE shop.users",
				"SET",
				"\tname = 'a',",
				"\temail = 'b'",
				"WHERE",
				"\tid = 1",
			}, "\n"),
		},
		{
			name: "an insert's column list and its values",
			in:   "insert into users (id,name) values (1,'a')",
			want: strings.Join([]string{
				"INSERT INTO users (id, name)",
				"VALUES",
				"\t(1, 'a')",
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Postgres.Format(tt.in); got != tt.want {
				t.Errorf("Format() =\n%s\n\nwant\n%s", got, tt.want)
			}
		})
	}
}

// A column list is long enough to be worth a line each, which is the whole of what the width
// rule is for: the same parentheses that stay closed up in `numeric(10,2)` open out here.
func TestFormatOpensOutALongList(t *testing.T) {
	got := Postgres.Format(
		"create table users (id integer not null, name varchar(255), created_at timestamp)",
	)
	want := strings.Join([]string{
		"CREATE TABLE users (",
		"\tid INTEGER NOT NULL,",
		"\tname VARCHAR(255),",
		"\tcreated_at TIMESTAMP",
		")",
	}, "\n")

	if got != want {
		t.Errorf("Format() =\n%s\n\nwant\n%s", got, want)
	}
}

func TestFormatSeparatesStatementsAndKeepsTheLastTerminator(t *testing.T) {
	got := Postgres.Format("select 1; select 2;")
	want := "SELECT\n\t1;\n\nSELECT\n\t2;"
	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}

	// One statement the reader never terminated does not gain a semicolon they did not type.
	if got := Postgres.Format("select 1"); got != "SELECT\n\t1" {
		t.Errorf("Format() = %q, want no terminator added", got)
	}
}

// Everything a server would notice has to survive byte for byte: a string, a quoted name and a
// comment are single tokens and are never re-cased, re-spaced or re-wrapped.
func TestFormatLeavesQuotedTextAndCommentsAlone(t *testing.T) {
	got := Postgres.Format(`select "Mixed Case", 'select from where' from "T"`)

	for _, want := range []string{`"Mixed Case"`, `'select from where'`, `"T"`} {
		if !strings.Contains(got, want) {
			t.Errorf("Format() = %q, want it to keep %s untouched", got, want)
		}
	}
}

func TestFormatKeepsACommentOnItsOwnLine(t *testing.T) {
	got := Postgres.Format("-- how many\nselect count(*) from t")
	want := "-- how many\nSELECT\n\tCOUNT(*)\nFROM\n\tt"

	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}
}

// A trailing line comment must never end up in front of the statement that followed it, which is
// the one way a formatter can silently comment out the reader's SQL.
func TestFormatNeverSwallowsCodeIntoALineComment(t *testing.T) {
	got := Postgres.Format("select 1 -- note\nfrom t")

	comment := strings.Index(got, "-- note")
	from := strings.Index(got, "FROM")
	if comment < 0 || from < 0 {
		t.Fatalf("Format() = %q, want both the comment and the FROM", got)
	}
	if !strings.Contains(got[comment:from], "\n") {
		t.Errorf("Format() = %q, want a line break between the comment and what follows", got)
	}
}

// A script the lexer cannot make sense of is handed back exactly as it was: the one thing worse
// than an unformatted script is a formatted one that no longer runs.
func TestFormatLeavesAnUnterminatedScriptAlone(t *testing.T) {
	for _, script := range []string{
		"select 'unclosed from t",
		"select 1 /* unclosed",
		`select "unclosed from t`,
	} {
		if got := Postgres.Format(script); got != script {
			t.Errorf("Format(%q) = %q, want it back unchanged", script, got)
		}
	}
}

func TestFormatOfNothingIsNothing(t *testing.T) {
	for _, script := range []string{"", "   \n\t"} {
		if got := Postgres.Format(script); got != "" {
			t.Errorf("Format(%q) = %q, want the empty string", script, got)
		}
	}
}

// A script that is only comments is still the reader's text, so it comes back rather than being
// dropped the way Split drops it.
func TestFormatKeepsAScriptThatIsOnlyComments(t *testing.T) {
	if got := Postgres.Format("-- nothing to run\n"); !strings.Contains(got, "-- nothing to run") {
		t.Errorf("Format() = %q, want the comment kept", got)
	}
}

// MySQL reads `#` as a comment and a backslash as an escape; PostgreSQL reads neither that way.
// The layout is the same on both — only what counts as one token differs.
func TestFormatReadsCommentsAndEscapesByTheEnginesRule(t *testing.T) {
	if got := MySQL.Format("select 1 # note\nfrom t"); !strings.Contains(got, "# note\nFROM") {
		t.Errorf("MySQL.Format() = %q, want `#` read as a comment", got)
	}
	// On MySQL the backslash escapes the closing quote, so the script never terminates and is
	// handed back whole rather than laid out around a string that is not one.
	const path = `select 'C:\' from t`
	if got := MySQL.Format(path); got != path {
		t.Errorf("MySQL.Format(%q) = %q, want it back unchanged", path, got)
	}
	if got := Postgres.Format(path); got == path {
		t.Errorf("Postgres.Format(%q) = %q, want it laid out: the string closes here", path, got)
	}
}

// Formatting is idempotent, which is what makes it safe to bind to a keystroke.
func TestFormatTwiceChangesNothing(t *testing.T) {
	scripts := []string{
		"select id, name from users where a = 1 and b = 2 order by id",
		"insert into t (a, b) values (1, 2); update t set a = 1 where b = 2;",
		"select * from t where id in (select id from u where x = 1)",
		"create table users (id integer not null, name varchar(255), created_at timestamp)",
	}

	for _, script := range scripts {
		once := Postgres.Format(script)
		if twice := Postgres.Format(once); twice != once {
			t.Errorf("Format() is not idempotent for %q:\n%s\n\nbecame\n%s", script, once, twice)
		}
	}
}

// The formatter may move whitespace and change the case of keywords, and nothing else. Comparing
// the token text with case and space taken out is what proves it.
func TestFormatChangesNothingAServerWouldNotice(t *testing.T) {
	scripts := []string{
		"select id, name from users where a = 1 and b = 2",
		`select "Mixed", 'a;b' from "T" join u on u.id = "T".id`,
		"update t set a = 1 where b = 2",
		"select count(*) from t group by a having count(*) > 1",
	}

	for _, script := range scripts {
		before := meaningful(Postgres.Lex(script))
		after := meaningful(Postgres.Lex(Postgres.Format(script)))

		if before != after {
			t.Errorf("Format(%q) changed the statement:\n%s\n\nbecame\n%s", script, before, after)
		}
	}
}

// meaningful renders the tokens the way a server reads them: an unquoted word folded to one case,
// and everything quoted left exactly as it is.
func meaningful(tokens []Token) string {
	parts := make([]string, 0, len(tokens))
	for _, one := range tokens {
		if one.Kind == TokenWord {
			parts = append(parts, strings.ToLower(one.Text))
			continue
		}
		parts = append(parts, one.Text)
	}
	return strings.Join(parts, "\x00")
}
