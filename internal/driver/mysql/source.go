package mysql

import (
	"strings"
	"unicode"
)

// source is the table a result set can be written back to.
type source struct {
	database string
	table    string
}

// tailClauses are the only clauses allowed after the table of a writable SELECT. A JOIN,
// a UNION, a second table or a GROUP BY breaks the one-result-row-per-stored-row mapping
// that addressing a row by its primary key depends on, so anything else disqualifies it.
var tailClauses = map[string]bool{
	"where": true,
	"order": true,
	"limit": true,
	"for":   true,
	"lock":  true,
}

// parseSource reads the single table a plain SELECT draws from. It reports false for every
// statement whose rows cannot be traced back to exactly one stored row.
func parseSource(statement string) (source, bool) {
	tokens := scanTokens(statement)
	if len(tokens) < 4 || !strings.EqualFold(tokens[0], "select") {
		return source{}, false
	}
	if strings.EqualFold(tokens[1], "distinct") || strings.EqualFold(tokens[1], "distinctrow") {
		return source{}, false
	}

	from := indexFold(tokens, "from")
	if from < 0 || from+1 >= len(tokens) {
		return source{}, false
	}

	found, ok := splitQualified(tokens[from+1])
	if !ok {
		return source{}, false
	}

	rest := tokens[from+2:]
	if len(rest) > 0 && strings.EqualFold(rest[0], "as") {
		rest = rest[1:] // AS
	}
	if len(rest) > 0 && !tailClauses[strings.ToLower(rest[0])] {
		rest = rest[1:] // the alias
	}
	if len(rest) > 0 && !tailClauses[strings.ToLower(rest[0])] {
		return source{}, false
	}

	return found, true
}

// scanTokens splits a statement into its words at parenthesis depth zero. Comments are
// dropped and quoted text collapses to a placeholder, so only structure survives — a
// subquery, a function call or a string can no longer be mistaken for a clause.
func scanTokens(statement string) []string {
	tokens := []string{}
	word := strings.Builder{}
	depth := 0

	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	emit := func(token string) {
		flush()
		if depth == 0 {
			tokens = append(tokens, token)
		}
	}

	for i := 0; i < len(statement); i++ {
		switch c := statement[i]; {
		case c == '\'' || c == '"':
			emit("?")
			i = skipDelimited(statement, i, c)

		case c == '`':
			end := skipDelimited(statement, i, '`')
			if depth == 0 {
				word.WriteString(statement[i:min(end+1, len(statement))])
			}
			i = end

		case c == '#', c == '-' && peek(statement, i) == '-':
			flush()
			i = skipLine(statement, i)

		case c == '/' && peek(statement, i) == '*':
			flush()
			i = skipBlock(statement, i)

		case c == '(':
			emit("(")
			depth++

		case c == ')':
			flush()
			depth--
			if depth == 0 {
				tokens = append(tokens, ")")
			}

		case c == ',':
			emit(",")

		case c == ' ', c == '\t', c == '\r', c == '\n':
			flush()

		default:
			if depth == 0 {
				word.WriteByte(c)
			}
		}
	}

	flush()
	return tokens
}

// splitQualified reads `db`.`table`, db.table or a bare table name.
func splitQualified(token string) (source, bool) {
	parts := []string{}
	part := strings.Builder{}
	quoted := false

	for i := 0; i < len(token); i++ {
		switch c := token[i]; {
		case c == '`':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, part.String())
			part.Reset()
		default:
			part.WriteByte(c)
		}
	}
	parts = append(parts, part.String())

	switch len(parts) {
	case 1:
		return source{table: parts[0]}, isIdentifier(parts[0])
	case 2:
		return source{database: parts[0], table: parts[1]},
			isIdentifier(parts[0]) && isIdentifier(parts[1])
	default:
		return source{}, false
	}
}

func isIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' && r < 0x80 {
			return false
		}
	}
	return true
}

func indexFold(tokens []string, want string) int {
	for i, token := range tokens {
		if strings.EqualFold(token, want) {
			return i
		}
	}
	return -1
}

// skipDelimited returns the index of the closing delimiter, or the last byte when the
// statement ends first.
func skipDelimited(s string, start int, delimiter byte) int {
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case delimiter:
			if peek(s, i) == delimiter {
				i++
				continue
			}
			return i
		}
	}
	return len(s) - 1
}

func skipLine(s string, start int) int {
	if end := strings.IndexAny(s[start:], "\r\n"); end >= 0 {
		return start + end
	}
	return len(s) - 1
}

func skipBlock(s string, start int) int {
	if end := strings.Index(s[start+2:], "*/"); end >= 0 {
		return start + end + 3
	}
	return len(s) - 1
}

func peek(s string, i int) byte {
	if i+1 >= len(s) {
		return 0
	}
	return s[i+1]
}
