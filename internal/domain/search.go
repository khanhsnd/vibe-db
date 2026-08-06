package domain

import (
	"regexp"
	"strings"
)

// SearchScope is which half of an object a Find in Schema reads. They are two different *kinds*
// of question and are answered in two different places: which rows hold the text is a statement,
// and so the driver's, while which *names* do is nobody's statement at all — a field, an index and
// a constraint are names the app has already read.
type SearchScope string

const (
	SearchData      SearchScope = "data"
	SearchStructure SearchScope = "structure"
)

// Known says this is a half of an object the app searches, and is what refuses one it does not
// know rather than defaulting to either: a search that read the wrong half would look like an
// answer.
func (s SearchScope) Known() bool {
	return s == SearchData || s == SearchStructure
}

// SearchMode is how the text a reader typed is matched, named by a fixed token rather than spelled
// as the pattern itself — the same rule a filter's operator follows, and for the same reason: a
// mode this side does not know is refused instead of reaching a server as something nobody meant.
type SearchMode string

const (
	// MatchContains and MatchPrefix are *patterns*: `%` stands for any run of characters, `_` for
	// one, and `\` makes either of them itself. That is what the note beside the box says.
	MatchContains SearchMode = "contains"
	MatchPrefix   SearchMode = "prefix"
	// MatchWord is the text between two word edges, taken literally — `a.b` matches `a.b` here.
	MatchWord SearchMode = "word"
	// MatchRegex is the reader's own expression, in the engine's own flavour of one.
	MatchRegex SearchMode = "regex"
)

// Known says this is a way of matching the app has, and is what refuses one it has not.
func (m SearchMode) Known() bool {
	switch m {
	case MatchContains, MatchPrefix, MatchWord, MatchRegex:
		return true
	default:
		return false
	}
}

// TextMatch is the text a reader typed and how it is to be matched. It is one rule with two
// spellings — a pattern for an engine that has LIKE, an expression for everything else — which is
// what stops a data search and a structure search meaning different things by `Contains`.
type TextMatch struct {
	Text            string
	Mode            SearchMode
	CaseInsensitive bool
}

func (m TextMatch) Validate() error {
	if m.Text == "" {
		return ValidationError{Field: "text", Message: "is required"}
	}
	if !m.Mode.Known() {
		return ValidationError{Field: "mode", Message: "is not a way of matching this app knows"}
	}
	return nil
}

// Pattern is the mode spelled as a LIKE pattern, and empty for the two modes a pattern cannot say.
// The reader's text goes in verbatim, so a `%` they typed is a wildcard and `\%` is a per cent
// sign — which is LIKE's own escape and therefore the one this app promises.
func (m TextMatch) Pattern() string {
	switch m.Mode {
	case MatchContains:
		return "%" + m.Text + "%"
	case MatchPrefix:
		return m.Text + "%"
	default:
		return ""
	}
}

// IsPattern says the match is spelled as a pattern rather than as an expression, which is what a
// caller switches on to know which of the two to ask for.
func (m TextMatch) IsPattern() bool {
	return m.Mode == MatchContains || m.Mode == MatchPrefix
}

// Expression is the same rule spelled as a regular expression, whatever the mode: what an engine
// with no LIKE reads, and what a name is matched by on this side. `boundary` is the engine's own
// token for the edge of a word, which is the one piece of the syntax that is not shared — `\b` on
// MySQL's ICU and in Go, `\y` on PostgreSQL, where `\b` is a backspace.
//
// It carries no flags: whether the match ignores case, and whether a dot reaches a newline, are the
// caller's to say in the way its own engine says them.
func (m TextMatch) Expression(boundary string) string {
	switch m.Mode {
	case MatchRegex:
		return m.Text
	case MatchWord:
		return boundary + regexp.QuoteMeta(m.Text) + boundary
	case MatchPrefix:
		return "^" + patternExpression(m.Text)
	default:
		return patternExpression(m.Text)
	}
}

// Matcher compiles the rule for matching one name here, which is the whole of a structure search:
// a field, an index and a constraint are names the app holds already, so nothing about them is
// asked of a server.
func (m TextMatch) Matcher() (Matcher, error) {
	if err := m.Validate(); err != nil {
		return Matcher{}, err
	}

	// `s` so a dot reaches a newline: a value or a name holding one is still one string, and a
	// pattern that stopped at it would answer differently from the server's own LIKE.
	flags := "(?s)"
	if m.CaseInsensitive {
		flags = "(?is)"
	}

	compiled, err := regexp.Compile(flags + m.Expression(goBoundary))
	if err != nil {
		return Matcher{}, ValidationError{
			Field:   "text",
			Message: "is not a regular expression this app can read",
		}
	}
	return Matcher{expression: compiled}, nil
}

// goBoundary is the word edge in the flavour of regular expression this side compiles.
const goBoundary = `\b`

// Matcher is a compiled TextMatch, so a search over a hundred tables reads the reader's text once.
type Matcher struct {
	expression *regexp.Regexp
}

func (m Matcher) Matches(value string) bool {
	return m.expression != nil && m.expression.MatchString(value)
}

// patternExpression reads a pattern as a regular expression: `%` is any run of characters, `_` is
// one, `\` makes the next character itself, and **everything else is literal** — which is what
// keeps `a.b` the three characters a reader typed in every mode but Regular Expression.
func patternExpression(text string) string {
	var out strings.Builder
	escaped := false

	for _, character := range text {
		switch {
		case escaped:
			out.WriteString(regexp.QuoteMeta(string(character)))
			escaped = false
		case character == '\\':
			escaped = true
		case character == '%':
			out.WriteString(".*")
		case character == '_':
			out.WriteString(".")
		default:
			out.WriteString(regexp.QuoteMeta(string(character)))
		}
	}

	// A pattern ending in a lone `\` escapes nothing, so it is the character itself.
	if escaped {
		out.WriteString(regexp.QuoteMeta(`\`))
	}
	return out.String()
}
