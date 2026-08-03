package sqlkit

import (
	"strings"
	"unicode"
)

// Split cuts a script into the statements it holds. What ends a statement is a semicolon that is
// not *inside* something, and what counts as inside is lexical and therefore the dialect's: a
// string literal, a quoted identifier, a line or block comment, and — where the engine has them —
// a dollar-quoted body, which is how a PostgreSQL function carries semicolons of its own.
//
// A chunk with no statement in it is dropped rather than sent: trailing whitespace, and the
// comment a script ends on, are not statements the server has anything to do with.
//
// Every skip here answers with the last byte it consumed, which is the convention `source.go`
// already uses — so the loop below is the one place that steps past it.
func Split(script string, d Dialect) []string {
	statements := []string{}
	start := 0

	for at := 0; at < len(script); at++ {
		rest := script[at:]

		switch {
		case script[at] == '\'' || script[at] == '"' || script[at] == d.IdentQuote:
			at = skipQuoted(script, at, script[at], d.BackslashEscapes)

		case strings.HasPrefix(rest, "--"), d.HashComments && script[at] == '#':
			at = skipLine(script, at)

		case strings.HasPrefix(rest, "/*"):
			at = skipBlock(script, at)

		case d.DollarQuotes && script[at] == '$':
			if end, ok := skipDollar(script, at); ok {
				at = end
			}

		case script[at] == ';':
			statements = appendStatement(statements, script[start:at])
			start = at + 1
		}
	}

	return appendStatement(statements, script[start:])
}

// appendStatement keeps only a chunk that has a statement in it, which is what a leading keyword
// found past the comments says. An unterminated quote or comment runs to the end of the script
// and is sent as it stands, so the server is the one that says what is wrong with it.
func appendStatement(statements []string, chunk string) []string {
	if LeadingKeyword(chunk) == "" {
		return statements
	}
	return append(statements, strings.TrimSpace(chunk))
}

// skipQuoted is skipDelimited told whether this engine reads a backslash as an escape.
// PostgreSQL does not, so `'C:\'` is a whole string there and a runaway one in MySQL.
func skipQuoted(text string, at int, quote byte, backslashEscapes bool) int {
	if backslashEscapes {
		return skipDelimited(text, at, quote)
	}

	for i := at + 1; i < len(text); i++ {
		if text[i] != quote {
			continue
		}
		if peek(text, i) == quote {
			i++
			continue
		}
		return i
	}
	return len(text) - 1
}

// skipDollar runs past a `$tag$ … $tag$` body and answers with its last byte. It answers false
// for a `$` that opens no such body — a placeholder, or a `$` inside a name — so that character
// is stepped over like any other.
func skipDollar(text string, at int) (int, bool) {
	end := strings.IndexByte(text[at+1:], '$')
	if end < 0 {
		return 0, false
	}

	tag := text[at : at+end+2]
	for _, letter := range tag[1 : len(tag)-1] {
		if letter != '_' && !unicode.IsLetter(letter) && !unicode.IsDigit(letter) {
			return 0, false
		}
	}

	body := at + len(tag)
	closing := strings.Index(text[body:], tag)
	if closing < 0 {
		return len(text) - 1, true
	}
	return body + closing + len(tag) - 1, true
}
