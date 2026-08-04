package sqlkit

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Review reads what a script is going to do, and what is already wrong with it, without sending
// any of it anywhere.
//
// **It is a lexer and not a parser, deliberately.** Everything it reports it can prove: a quote
// the script ends inside of, a parenthesis with no partner, a statement whose first word says it
// writes. Whether a column exists, whether a clause is in the right order, whether a type will
// take that value — those are the server's answers, and a client that guessed at them would be
// wrong often enough that a reader learned to ignore it, which costs more than the warning saved.
func (d Dialect) Review(script string) driver.Review {
	tokens := d.Lex(script)
	spans := d.Spans(script)

	statements := make([]driver.ReviewedStatement, 0, len(spans))
	for _, span := range spans {
		sql := script[span.Start:span.End]
		keyword := LeadingKeyword(sql)

		statements = append(statements, driver.ReviewedStatement{
			Start:   span.Start,
			End:     span.End,
			SQL:     sql,
			Keyword: keyword,
			Hazard:  domain.StatementHazard(keyword, d.HasClause(sql, "where")),
		})
	}

	return driver.Review{Statements: statements, Problems: Problems(tokens)}
}

// brackets is every pair a script may nest, and what each opener's partner is. All three are
// checked whatever the language: SQL writes `{` only inside a string, which is one token and not
// punctuation, so nothing is reported that was not already unbalanced.
var brackets = map[string]string{"(": ")", "[": "]", "{": "}"}

// Problems walks the tokens once for the two faults a lexer can be certain of: a quote or a
// comment the script ends inside of, and a bracket with no partner. It is exported because it is
// every language's answer and not only SQL's — the MongoDB adapter's Review reports the same two
// faults, read off the same tokens.
func Problems(tokens []Token) []driver.Problem {
	found := []driver.Problem{}
	open := []Token{}

	for _, one := range tokens {
		if one.Unterminated {
			found = append(found, driver.Problem{
				Start:   one.Start,
				End:     one.End,
				Message: unterminated(one),
			})
			continue
		}

		if one.Kind != TokenPunct {
			continue
		}
		if _, ok := brackets[one.Text]; ok {
			open = append(open, one)
			continue
		}
		if !closes(one.Text) {
			continue
		}

		// A closer is matched against the innermost opener, and a mismatched pair is reported as
		// the closer having nothing to close: `(a]` is one fault and not two.
		if len(open) == 0 || brackets[open[len(open)-1].Text] != one.Text {
			found = append(found, driver.Problem{
				Start:   one.Start,
				End:     one.End,
				Message: "This closing bracket has nothing to close.",
			})
			continue
		}
		open = open[:len(open)-1]
	}

	// Whatever is still open at the end never closed. They are reported oldest first, because the
	// outermost is nearly always the one the reader forgot.
	for _, one := range open {
		found = append(found, driver.Problem{
			Start:   one.Start,
			End:     one.End,
			Message: "This bracket is never closed.",
		})
	}

	return found
}

func closes(text string) bool {
	for _, closer := range brackets {
		if text == closer {
			return true
		}
	}
	return false
}

func unterminated(one Token) string {
	switch {
	case one.Kind == TokenComment:
		return "This block comment is never closed — everything after it is inside it."
	case one.Kind == TokenIdent:
		return "This quoted name is never closed — everything after it is part of the name."
	case strings.HasPrefix(one.Text, "$"):
		return "This $tag$ body is never closed — everything after it is inside it."
	default:
		return "This string is never closed — everything after it is inside it."
	}
}
