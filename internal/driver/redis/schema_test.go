package redis

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func rowOf(values ...string) []*string {
	row := make([]*string, 0, len(values))
	for _, value := range values {
		row = append(row, cell(value))
	}
	return row
}

func TestRowStatementsSpellsARowAsTheCommandsThatWriteIt(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		row     []*string
		mode    driver.RowWriteMode
		want    []string
	}{
		{
			"a string",
			[]string{columnKey, columnType, columnValue},
			rowOf("user:1", typeString, "hello"),
			driver.WriteAppend,
			[]string{"SET user:1 hello"},
		},
		{
			"a string with a clock",
			[]string{columnKey, columnType, columnValue, columnTTL},
			rowOf("user:1", typeString, "hello", "60"),
			driver.WriteAppend,
			[]string{"SET user:1 hello EX 60"},
		},
		{
			// `SET` is the one command with a word for it, so a string can be added-only exactly.
			"a string that must not already be there",
			[]string{columnKey, columnType, columnValue},
			rowOf("user:1", typeString, "hello"),
			driver.WriteSkip,
			[]string{"SET user:1 hello NX"},
		},
		{
			"a string that must already be there",
			[]string{columnKey, columnType, columnValue},
			rowOf("user:1", typeString, "hello"),
			driver.WriteUpdate,
			[]string{"SET user:1 hello XX"},
		},
		{
			"a hash",
			[]string{columnKey, columnType, columnValue},
			rowOf("user:1", typeHash, `{"name":"bob","age":"7"}`),
			driver.WriteUpsert,
			[]string{"UNLINK user:1", "HSET user:1 name bob age 7"},
		},
		{
			"a list, in its order",
			[]string{columnKey, columnType, columnValue},
			rowOf("queue", typeList, `["a","b","c"]`),
			driver.WriteUpsert,
			[]string{"UNLINK queue", "RPUSH queue a b c"},
		},
		{
			// ZADD takes the score first, which is the one place a pair is written the other way
			// round from how it is read.
			"a sorted set",
			[]string{columnKey, columnType, columnValue},
			rowOf("board", typeZSet, `{"bob":"10","ann":"20"}`),
			driver.WriteUpsert,
			[]string{"UNLINK board", "ZADD board 10 bob 20 ann"},
		},
		{
			"a key being removed",
			[]string{columnKey},
			rowOf("user:1"),
			driver.WriteDelete,
			[]string{"UNLINK user:1"},
		},
		{
			// A container has no word for "only if it is not there", so nothing is spelled rather
			// than a rebuild that would overwrite what the reader asked to leave alone.
			"a hash that must not already be there",
			[]string{columnKey, columnType, columnValue},
			rowOf("user:1", typeHash, `{"name":"bob"}`),
			driver.WriteSkip,
			nil,
		},
		{
			"a value with a space in it",
			[]string{columnKey, columnValue},
			rowOf("greeting", "hello world"),
			driver.WriteAppend,
			[]string{`SET greeting "hello world"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&session{}).RowStatements(driver.Namespace{}, keySpace, driver.RowWrite{
				Mode:    tt.mode,
				Columns: tt.columns,
				Rows:    [][]*string{tt.row},
			})

			if len(got) != len(tt.want) {
				t.Fatalf("RowStatements() = %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("statement %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// A row with nothing to find it by matches nothing — and on this engine it is stronger than that:
// without a key there is nowhere for a value to be at all.
func TestRowStatementsWritesNothingWithoutAKey(t *testing.T) {
	got := (&session{}).RowStatements(driver.Namespace{}, keySpace, driver.RowWrite{
		Mode:    driver.WriteAppend,
		Columns: []string{columnValue},
		Rows:    [][]*string{rowOf("hello")},
	})
	if len(got) != 0 {
		t.Errorf("RowStatements() = %q, want none", got)
	}
}

func TestGlobPatternSpellsASearchExactly(t *testing.T) {
	tests := []struct {
		name  string
		match domain.TextMatch
		want  string
	}{
		{
			"contains",
			domain.TextMatch{Text: "session", Mode: domain.MatchContains},
			"*session*",
		},
		{
			"a prefix",
			domain.TextMatch{Text: "user:", Mode: domain.MatchPrefix},
			"user:*",
		},
		{
			// Redis has no case-insensitive matching, so the pattern carries the folding itself.
			"either case",
			domain.TextMatch{Text: "Us1", Mode: domain.MatchPrefix, CaseInsensitive: true},
			"[uU][sS]1*",
		},
		{
			"the reader's own wildcards",
			domain.TextMatch{Text: "a%b_c", Mode: domain.MatchPrefix},
			"a*b?c*",
		},
		{
			"a glob character in the text",
			domain.TextMatch{Text: "a*b", Mode: domain.MatchPrefix},
			`a\*b*`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := globPattern(tt.match); got != tt.want {
				t.Errorf("globPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateStatementsSaysAKeySpaceIsNotDeclared(t *testing.T) {
	statements, err := (&session{}).CreateStatements(t.Context(), driver.Namespace{}, keySpace)
	if err != nil {
		t.Fatalf("CreateStatements() error = %v", err)
	}
	if len(statements) != 1 || !strings.HasPrefix(statements[0], "#") {
		t.Errorf("CreateStatements() = %q, want one comment", statements)
	}
}
