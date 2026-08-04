package sqlkit

import "strings"

// TokenKind is what one piece of a script *is*, which is as much as a lexer can honestly say.
// It never distinguishes a keyword from a name — whether `user` is a column or a reserved word is
// a question about the statement around it — so both are TokenWord and the keyword list above is
// what decides which of them gets laid out and upper-cased.
type TokenKind int

const (
	TokenWord TokenKind = iota
	TokenNumber
	// TokenString is a literal: `'…'` everywhere, `"…"` on an engine whose identifier quote is
	// something else, and a `$tag$ … $tag$` body where the engine has them.
	TokenString
	// TokenIdent is a name written in the engine's own identifier quote, and is therefore the one
	// kind of word never touched by anything downstream: its case and its spacing are the
	// reader's, and a server would read a change to either as a different name.
	TokenIdent
	TokenComment
	// TokenPunct is everything else — one of `(){},;.` or an operator, taken longest-first so
	// `<=` is one token and not two.
	TokenPunct
)

// Token is one piece of a script and where it sits in it. End is exclusive, so `script[t.Start:
// t.End]` is exactly the text, which is what lets a diagnostic point at the script the reader is
// looking at rather than at a copy of it.
type Token struct {
	Kind  TokenKind
	Start int
	End   int
	Text  string
	// Unterminated marks a quote, a comment or a dollar body that the script simply ends inside
	// of. The token still runs to the end of the text — there is nothing else it could be — and
	// this is what tells Review to say so rather than pretending the script was whole.
	Unterminated bool
}

// operators are read longest-first, so `->>` is one token before `->` can claim it and `->`
// before `-` can. Anything not here is punctuation one byte at a time.
var operators = []string{"->>", "#>>", "<=>", "->", "#>", "<=", ">=", "<>", "!=", "||", "::", ":="}

// Lex cuts a script into its tokens. It is the one place in this package that reads a script
// character by character: Spans, Format and Review are all written over the result, so what a
// quote or a comment *is* is settled once per engine instead of three times.
func (d Dialect) Lex(script string) []Token {
	tokens := []Token{}
	at := 0

	for at < len(script) {
		switch c := script[at]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			at++

		case c == '\'', c == '"' && d.IdentQuote != '"':
			tokens = append(tokens, d.lexQuoted(script, at, c, TokenString))
			at = tokens[len(tokens)-1].End

		case c == d.IdentQuote:
			tokens = append(tokens, d.lexQuoted(script, at, c, TokenIdent))
			at = tokens[len(tokens)-1].End

		case strings.HasPrefix(script[at:], "--"),
			d.HashComments && c == '#',
			d.SlashComments && strings.HasPrefix(script[at:], "//"):
			end := lineEnd(script, at)
			tokens = append(tokens, token(TokenComment, script, at, end, false))
			at = end

		case strings.HasPrefix(script[at:], "/*"):
			end, closed := blockEnd(script, at)
			tokens = append(tokens, token(TokenComment, script, at, end, !closed))
			at = end

		case d.DollarQuotes && c == '$':
			if end, closed, ok := dollarEnd(script, at); ok {
				tokens = append(tokens, token(TokenString, script, at, end, !closed))
				at = end
				continue
			}
			tokens = append(tokens, token(TokenPunct, script, at, at+1, false))
			at++

		case isDigit(c):
			end := numberEnd(script, at)
			tokens = append(tokens, token(TokenNumber, script, at, end, false))
			at = end

		case isWordByte(c):
			end := wordEnd(script, at)
			tokens = append(tokens, token(TokenWord, script, at, end, false))
			at = end

		default:
			end := at + 1
			for _, operator := range operators {
				if strings.HasPrefix(script[at:], operator) {
					end = at + len(operator)
					break
				}
			}
			tokens = append(tokens, token(TokenPunct, script, at, end, false))
			at = end
		}
	}

	return tokens
}

// lexQuoted reads to the closing quote. A doubled quote is one escaped character rather than the
// end, and a backslash is an escape only on an engine that reads it as one — which is why `'C:\'`
// is a whole string on PostgreSQL and a runaway one on MySQL.
func (d Dialect) lexQuoted(script string, at int, quote byte, kind TokenKind) Token {
	for i := at + 1; i < len(script); i++ {
		switch script[i] {
		case '\\':
			if d.BackslashEscapes {
				i++
			}
		case quote:
			if peek(script, i) == quote {
				i++
				continue
			}
			return token(kind, script, at, i+1, false)
		}
	}
	return token(kind, script, at, len(script), true)
}

func token(kind TokenKind, script string, start, end int, unterminated bool) Token {
	return Token{
		Kind:         kind,
		Start:        start,
		End:          end,
		Text:         script[start:end],
		Unterminated: unterminated,
	}
}

func lineEnd(script string, at int) int {
	if end := strings.IndexAny(script[at:], "\r\n"); end >= 0 {
		return at + end
	}
	return len(script)
}

func blockEnd(script string, at int) (int, bool) {
	if end := strings.Index(script[at+2:], "*/"); end >= 0 {
		return at + 2 + end + 2, true
	}
	return len(script), false
}

// dollarEnd reads a `$tag$ … $tag$` body. It answers false for a `$` that opens no such body —
// a `$1` placeholder, or a `$` inside a name — so that character is then punctuation like any
// other.
func dollarEnd(script string, at int) (end int, closed, ok bool) {
	next := strings.IndexByte(script[at+1:], '$')
	if next < 0 {
		return 0, false, false
	}

	tag := script[at : at+next+2]
	for i := 1; i < len(tag)-1; i++ {
		if !isWordByte(tag[i]) && !isDigit(tag[i]) {
			return 0, false, false
		}
	}

	body := at + len(tag)
	closing := strings.Index(script[body:], tag)
	if closing < 0 {
		return len(script), false, true
	}
	return body + closing + len(tag), true, true
}

func numberEnd(script string, at int) int {
	i := at
	for i < len(script) && (isDigit(script[i]) || script[i] == '.') {
		i++
	}
	// An exponent, and the sign that may follow it: `1e-9` is one number and not three tokens.
	if i < len(script) && (script[i] == 'e' || script[i] == 'E') {
		j := i + 1
		if j < len(script) && (script[j] == '+' || script[j] == '-') {
			j++
		}
		if j < len(script) && isDigit(script[j]) {
			for j < len(script) && isDigit(script[j]) {
				j++
			}
			return j
		}
	}
	return i
}

func wordEnd(script string, at int) int {
	i := at
	for i < len(script) && (isWordByte(script[i]) || isDigit(script[i])) {
		i++
	}
	return i
}

// isWordByte is deliberately generous about what starts a name: `@` and `:` open a variable, `$`
// is legal in a MySQL name, and every byte above ASCII is left to be part of one rather than
// broken out of it, since neither engine restricts a quoted-or-not name to ASCII.
func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c == '@' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
