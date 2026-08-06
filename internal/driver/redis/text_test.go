package redis

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestSplitEndsACommandAtANewline(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{"one command", "GET a", []string{"GET a"}},
		{"two lines", "GET a\nGET b", []string{"GET a", "GET b"}},
		{"blank lines between", "GET a\n\n\nGET b", []string{"GET a", "GET b"}},
		{"a terminator anyway", "GET a;\nGET b;", []string{"GET a", "GET b"}},
		{"two on one line", "GET a; GET b", []string{"GET a", "GET b"}},
		{"a comment of its own", "# note\nGET a", []string{"GET a"}},
		{"a comment after a command", "GET a # note\nGET b", []string{"GET a", "GET b"}},
		{"nothing", "\n\n # only a note \n", nil},

		// A newline inside a quote ends nothing: the value is one argument that happens to have a
		// line break in it.
		{"a newline in a value", "SET a \"one\ntwo\"", []string{"SET a \"one\ntwo\""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := text{}.Split(tt.script)
			if len(got) != len(tt.want) {
				t.Fatalf("Split() = %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("command %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReviewReadsWhatEachCommandWillDo(t *testing.T) {
	tests := []struct {
		command string
		want    domain.Hazard
	}{
		{`GET user:1`, domain.HazardNone},
		{`SCAN 0 MATCH * COUNT 10`, domain.HazardNone},
		{`KEYS *`, domain.HazardNone},
		{`INFO keyspace`, domain.HazardNone},
		{`TTL user:1`, domain.HazardNone},
		{`CONFIG GET maxmemory`, domain.HazardNone},
		{`CLIENT LIST`, domain.HazardNone},

		{`SET user:1 hello`, domain.HazardWrite},
		{`HSET user:1 name bob`, domain.HazardWrite},
		{`DEL user:1`, domain.HazardWrite},
		{`UNLINK user:1`, domain.HazardWrite},
		{`EXPIRE user:1 60`, domain.HazardWrite},
		{`EVAL "return 1" 0`, domain.HazardWrite},

		{`RENAME a b`, domain.HazardDestructive},
		{`CONFIG SET maxmemory 0`, domain.HazardDestructive},
		{`SHUTDOWN NOSAVE`, domain.HazardDestructive},

		// The two that name nothing and therefore take everything. They are answered for on a laptop
		// as well as on production, which is the whole reason they are not merely destructive.
		{`FLUSHDB`, domain.HazardUnfiltered},
		{`FLUSHALL`, domain.HazardUnfiltered},
		{`SCRIPT FLUSH`, domain.HazardUnfiltered},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			review := text{}.Review(tt.command)
			if len(review.Statements) != 1 {
				t.Fatalf("Review() = %d statements, want 1", len(review.Statements))
			}
			if got := review.Statements[0].Hazard; got != tt.want {
				t.Errorf("Hazard = %q, want %q", got, tt.want)
			}
			if len(review.Problems) != 0 {
				t.Errorf("Problems = %v, want none", review.Problems)
			}
		})
	}
}

func TestReviewSaysWhereAQuoteIsNeverClosed(t *testing.T) {
	review := text{}.Review(`SET a "unfinished`)
	if len(review.Problems) != 1 {
		t.Fatalf("Problems = %v, want one", review.Problems)
	}
	if !strings.Contains(review.Problems[0].Message, "quote") {
		t.Errorf("Message = %q, want it to mention the quote", review.Problems[0].Message)
	}
}

// A reviewed command points back at the text the reader is looking at, which is what lets a warning
// sit on the line that earned it.
func TestReviewPointsAtTheCommandItRead(t *testing.T) {
	script := "GET a\nFLUSHDB"

	review := text{}.Review(script)
	if len(review.Statements) != 2 {
		t.Fatalf("Review() = %d statements, want 2", len(review.Statements))
	}

	second := review.Statements[1]
	if script[second.Start:second.End] != "FLUSHDB" {
		t.Errorf("statement points at %q, want FLUSHDB", script[second.Start:second.End])
	}
	if second.Keyword != "FLUSHDB" {
		t.Errorf("Keyword = %q, want FLUSHDB", second.Keyword)
	}
}

func TestFormatUppercasesTheVerbAndNothingElse(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"a command", "get   user:1", "GET user:1"},
		{"a container command", "config get  maxmemory", "CONFIG GET maxmemory"},
		{"a value is data", `set Greeting "Hello World"`, `SET Greeting "Hello World"`},
		{"two commands", "get a\n\nget b", "GET a\nGET b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (text{}).Format(tt.script); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A script the lexer cannot make sense of is handed back untouched, for the reason `sqlkit.Format`
// does the same: the one thing worse than an unformatted script is a formatted one that no longer
// runs.
func TestFormatLeavesAnUnfinishedQuoteAlone(t *testing.T) {
	script := `set a "unfinished`
	if got := (text{}).Format(script); got != script {
		t.Errorf("Format() = %q, want it unchanged", got)
	}
}
