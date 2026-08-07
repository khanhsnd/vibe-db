package redis

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// A glob is the only narrowing a Redis server does, and this file is the whole of one: how a
// reader's text is spelled as a glob, and how a glob is answered *here*.
//
// Both are needed, and by the same rule the rest of this package's filtering follows. A glob taken
// from the top level of an AND narrows the scan, and the whole tree is then applied to the keys that
// came back — so `Key matches user:*` has to mean the same thing in both places, or a read would
// drop rows the server had already admitted.

// escapeGlob makes a reader's text stand for itself. A key holding `user[1]` is a key a reader can
// filter for, which it would not be if the brackets were left to the server to read as a class.
func escapeGlob(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		switch character := text[i]; character {
		case '*', '?', '[', ']', '\\', '^', '-':
			out.WriteByte('\\')
			out.WriteByte(character)
		default:
			out.WriteByte(character)
		}
	}
	return out.String()
}

// foldGlob is a reader's text as a glob that stands for it **in either case**, each letter being the
// class of its two: `[uU][sS][eE][rR]`. Redis has no case-insensitive matching, so an insensitive
// search is spelled into the pattern itself rather than approximated — which is what lets the server
// do the narrowing for a search that ignores case, exactly as it does for one that does not.
func foldGlob(text string) string {
	var out strings.Builder
	for _, character := range text {
		out.WriteString(letter(character, true))
	}
	return out.String()
}

// globPattern spells a `domain.TextMatch` as the one thing a Redis server understands. A LIKE
// pattern and a glob are near neighbours — `%` is `*` and `_` is `?` — and the reader's own text is
// escaped so that a key holding `[` is a key they can search for.
//
// **Case is folded into the pattern itself**, the way `foldGlob` does it. It is exact rather than
// approximate, which is what lets the count and the read stay one clause.
func globPattern(match domain.TextMatch) string {
	pattern := match.Pattern()

	var out strings.Builder
	escaped := false

	for _, character := range pattern {
		switch {
		case escaped:
			out.WriteString(letter(character, match.CaseInsensitive))
			escaped = false
		case character == '\\':
			escaped = true
		case character == '%':
			out.WriteByte('*')
		case character == '_':
			out.WriteByte('?')
		default:
			out.WriteString(letter(character, match.CaseInsensitive))
		}
	}

	// A pattern ending in a lone backslash escapes nothing, so it is the character itself.
	if escaped {
		out.WriteString(escapeGlob(`\`))
	}
	return out.String()
}

// letter is one character of a pattern, standing for itself — and for its other case as well where
// the search asked to ignore case.
func letter(character rune, insensitive bool) string {
	lower, upper := strings.ToLower(string(character)), strings.ToUpper(string(character))
	if !insensitive || lower == upper {
		return escapeGlob(string(character))
	}
	return "[" + escapeGlob(lower) + escapeGlob(upper) + "]"
}

// globMatch answers a glob here, over a key the scan has already handed back: `*` is any run of
// characters, `?` is exactly one, `[…]` is a class — `^` negating it and `a-z` a range — and `\`
// makes the next character itself.
//
// It is Redis's own `stringmatchlen` rather than a regular expression, for two reasons. It gives the
// same answer as the server, which is the point of the file; and it compiles nothing, where a
// pattern read into a regular expression would be read again for every key of a page.
//
// It matches **bytes**, because that is what a Redis key is.
func globMatch(pattern, value string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// A run of stars says no more than one does, and a star at the end of a pattern says
			// whatever is left of the key is admitted.
			for len(pattern) > 1 && pattern[1] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 1 {
				return true
			}
			for at := 0; at <= len(value); at++ {
				if globMatch(pattern[1:], value[at:]) {
					return true
				}
			}
			return false

		case '?':
			if len(value) == 0 {
				return false
			}
			value = value[1:]

		case '[':
			if len(value) == 0 {
				return false
			}
			beyond, ok := matchClass(pattern, value[0])
			if !ok {
				return false
			}
			pattern, value = beyond, value[1:]
			continue

		case '\\':
			// A backslash at the very end escapes nothing, so it is the character itself.
			if len(pattern) > 1 {
				pattern = pattern[1:]
			}
			fallthrough

		default:
			if len(value) == 0 || value[0] != pattern[0] {
				return false
			}
			value = value[1:]
		}
		pattern = pattern[1:]
	}

	return len(value) == 0
}

// matchClass answers one `[…]` against one character, and hands back the pattern beyond the class.
// A class nobody closed runs to the end of the pattern, which is what the server does with one.
func matchClass(pattern string, character byte) (string, bool) {
	pattern = pattern[1:]

	negated := len(pattern) > 0 && pattern[0] == '^'
	if negated {
		pattern = pattern[1:]
	}

	found := false
	for len(pattern) > 0 && pattern[0] != ']' {
		switch {
		case pattern[0] == '\\' && len(pattern) > 1:
			pattern = pattern[1:]
			found = found || pattern[0] == character
		case len(pattern) > 2 && pattern[1] == '-':
			low, high := pattern[0], pattern[2]
			if low > high {
				low, high = high, low
			}
			found = found || (character >= low && character <= high)
			pattern = pattern[2:]
		default:
			found = found || pattern[0] == character
		}
		pattern = pattern[1:]
	}

	if len(pattern) > 0 {
		pattern = pattern[1:]
	}
	return pattern, found != negated
}
