package domain

import "testing"

func TestPatternIsTheReadersOwn(t *testing.T) {
	cases := []struct {
		mode SearchMode
		text string
		want string
	}{
		{MatchContains, "khanh", "%khanh%"},
		{MatchPrefix, "khanh", "khanh%"},
		// A wildcard the reader typed is theirs, and `\` is what makes one a literal.
		{MatchContains, `10\%`, `%10\%%`},
		// The two modes a pattern cannot say have none.
		{MatchWord, "khanh", ""},
		{MatchRegex, "^kh", ""},
	}

	for _, test := range cases {
		match := TextMatch{Text: test.text, Mode: test.mode}
		if got := match.Pattern(); got != test.want {
			t.Errorf("Pattern(%q, %q) = %q, want %q", test.mode, test.text, got, test.want)
		}
	}
}

func TestExpressionSpellsEveryMode(t *testing.T) {
	cases := []struct {
		mode SearchMode
		text string
		want string
	}{
		// A pattern read as an expression: `%` is any run, `_` is one, everything else is literal.
		{MatchContains, "a.b", `a\.b`},
		{MatchContains, "a%b", "a.*b"},
		{MatchContains, "a_b", "a.b"},
		{MatchContains, `a\%b`, `a%b`},
		{MatchPrefix, "a.b", `^a\.b`},
		// A whole word is the text between two edges, taken literally.
		{MatchWord, "a.b", `\ba\.b\b`},
		// The reader's own expression travels verbatim.
		{MatchRegex, `a.b\b`, `a.b\b`},
	}

	for _, test := range cases {
		match := TextMatch{Text: test.text, Mode: test.mode}
		if got := match.Expression(`\b`); got != test.want {
			t.Errorf("Expression(%q, %q) = %q, want %q", test.mode, test.text, got, test.want)
		}
	}
}

// The engine's own word edge is the one part of the syntax that is not shared, so it arrives as an
// argument rather than being spelled here.
func TestExpressionTakesTheEnginesWordBoundary(t *testing.T) {
	match := TextMatch{Text: "khanh", Mode: MatchWord}
	if got := match.Expression(`\y`); got != `\ykhanh\y` {
		t.Errorf("Expression() = %q, want PostgreSQL's own boundary", got)
	}
}

func TestMatcherAnswersForOneName(t *testing.T) {
	cases := []struct {
		name  string
		match TextMatch
		value string
		want  bool
	}{
		{"contains finds it anywhere", TextMatch{Text: "hanh", Mode: MatchContains}, "khanh_id", true},
		{"contains insists on case", TextMatch{Text: "Hanh", Mode: MatchContains}, "khanh_id", false},
		{
			"the box the reader ticked is what ignores it",
			TextMatch{Text: "Hanh", Mode: MatchContains, CaseInsensitive: true},
			"khanh_id",
			true,
		},
		{"a prefix is anchored", TextMatch{Text: "khanh", Mode: MatchPrefix}, "khanh_id", true},
		{"and only at the start", TextMatch{Text: "hanh", Mode: MatchPrefix}, "khanh_id", false},
		{"a word is bounded", TextMatch{Text: "khanh", Mode: MatchWord}, "khanh id", true},
		{"so half a word is not one", TextMatch{Text: "khan", Mode: MatchWord}, "khanh", false},
		{"an expression is the reader's", TextMatch{Text: "^kh.*d$", Mode: MatchRegex}, "khanh_id", true},
		// A pattern's metacharacters are the reader's text and nothing more.
		{"a dot is a dot in a pattern", TextMatch{Text: "a.b", Mode: MatchContains}, "axb", false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			matcher, err := test.match.Matcher()
			if err != nil {
				t.Fatalf("Matcher() error = %v", err)
			}
			if got := matcher.Matches(test.value); got != test.want {
				t.Errorf("Matches(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestMatcherRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name  string
		match TextMatch
	}{
		{"nothing to look for", TextMatch{Mode: MatchContains}},
		{"a mode this app does not know", TextMatch{Text: "khanh", Mode: "fuzzy"}},
		{"an expression that is not one", TextMatch{Text: "kh(anh", Mode: MatchRegex}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.match.Matcher(); err == nil {
				t.Fatal("Matcher() error = nil, want a rejection")
			}
		})
	}
}
