package sqlkit

import (
	"strings"
	"unicode"
)

// LeadingKeyword returns the first word of a statement in lower case, looking past leading
// comments and the parentheses of a wrapped SELECT. `#` counts as a comment for every
// engine, because no statement can begin with it whatever else the character means.
func LeadingKeyword(statement string) string {
	rest := statement
	for {
		rest = strings.TrimLeft(rest, " \t\r\n(")

		switch {
		case strings.HasPrefix(rest, "--"), strings.HasPrefix(rest, "#"):
			end := strings.IndexAny(rest, "\r\n")
			if end < 0 {
				return ""
			}
			rest = rest[end+1:]
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return ""
			}
			rest = rest[end+4:]
		default:
			end := strings.IndexFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
			if end < 0 {
				end = len(rest)
			}
			return strings.ToLower(rest[:end])
		}
	}
}

// TrimStatement strips the whitespace and trailing semicolons around a statement, so an
// empty one can be told apart from one worth sending.
func TrimStatement(sql string) string {
	return strings.TrimRight(strings.TrimSpace(sql), "; \t\r\n")
}
