package redis

import (
	"strings"
	"testing"
)

func TestArgumentsReadsACommandAsArgv(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"a plain command", `GET user`, []string{"GET", "user"}},
		{
			// The one that matters: a colon is not a word character to the lexer, so a key is
			// several tokens with nothing between them.
			"a namespaced key",
			`GET biz-dev:session:4012`,
			[]string{"GET", "biz-dev:session:4012"},
		},
		{"a quoted value", `SET greeting "hello world"`, []string{"SET", "greeting", "hello world"}},
		{"a single-quoted value", `SET a 'b c'`, []string{"SET", "a", "b c"}},
		{"an escape", `SET a "line\none"`, []string{"SET", "a", "line\none"}},
		{"a hex escape", `SET a "\x41"`, []string{"SET", "a", "A"}},
		{"a value beside a quote", `SET a x"y"`, []string{"SET", "a", `xy`}},
		// A quote anywhere in an argument opens a quoted run, which is what `redis-cli` does with
		// the same line: JSON typed bare loses its quotes, and JSON quoted whole does not.
		{"bare JSON", `SET a {"n":1}`, []string{"SET", "a", `{n:1}`}},
		{"quoted JSON", `SET a '{"n":1}'`, []string{"SET", "a", `{"n":1}`}},
		{"an empty value", `SET a ""`, []string{"SET", "a", ""}},
		{"a comment", `GET a # the note`, []string{"GET", "a"}},
		{"a terminator", `GET a;`, []string{"GET", "a"}},
		{"a negative number", `INCRBY n -5`, []string{"INCRBY", "n", "-5"}},
		{"a glob", `SCAN 0 MATCH user:*`, []string{"SCAN", "0", "MATCH", "user:*"}},
		{"nothing", `   `, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := arguments(tt.command)
			if err != nil {
				t.Fatalf("arguments() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("arguments() = %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("argument %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestArgumentsRefusesACommandEndingInsideAQuote(t *testing.T) {
	if _, err := arguments(`SET a "unfinished`); err == nil {
		t.Fatal("arguments() error = nil, want a refusal")
	}
}

// Everything this adapter spells has to be something a reader could paste back, which is only true
// if what it writes reads back as what it wrote.
func TestQuotedArgumentsSurviveBeingReadBack(t *testing.T) {
	values := []string{
		"plain",
		"user:1",
		"with a space",
		`with "quotes"`,
		`with \ backslash`,
		"with\nnewline",
		"with;semicolon",
		"with#hash",
		"",
		`{"n":1,"s":"x y"}`,
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			line := command("SET", "key", value)

			got, err := arguments(line)
			if err != nil {
				t.Fatalf("arguments(%q) error = %v", line, err)
			}
			if len(got) != 3 {
				t.Fatalf("arguments(%q) = %q, want 3 arguments", line, got)
			}
			if got[2] != value {
				t.Errorf("arguments(%q)[2] = %q, want %q", line, got[2], value)
			}
		})
	}
}

func TestPlainWordsAreWrittenWithoutQuotes(t *testing.T) {
	if got := command("SET", "user:1", "hello"); got != "SET user:1 hello" {
		t.Errorf("command() = %q, want %q", got, "SET user:1 hello")
	}
}

func TestVerbReadsAContainerCommandsSecondWord(t *testing.T) {
	name, argument := verb([]string{"CONFIG", "SET", "maxmemory", "0"})
	if name != "CONFIG" || argument != "SET" {
		t.Errorf("verb() = %q %q, want CONFIG SET", name, argument)
	}

	if name, argument = verb([]string{"PING"}); name != "PING" || argument != "" {
		t.Errorf("verb() = %q %q, want PING and nothing", name, argument)
	}
}

func TestUnquoteLeavesTextItDoesNotUnderstandAlone(t *testing.T) {
	if got := unquote(`"a\qb"`); got != "aqb" {
		t.Errorf("unquote() = %q, want %q", got, "aqb")
	}
	if got := unquote(`""`); got != "" {
		t.Errorf("unquote() = %q, want empty", got)
	}
	if got := unquote(`"` + strings.Repeat("x", 3) + `"`); got != "xxx" {
		t.Errorf("unquote() = %q, want xxx", got)
	}
}
