package sqlkit

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// MatchAny spells the condition a row satisfies when **any one of its columns** holds the reader's
// text — the whole of a Find in Schema's data half, whose answer is a count through this clause.
// The clause leads with a space and is used exactly as Where's is.
//
// Every column is read as text, so one condition serves a name, a number and a date alike, which is
// the rule the grid's own `contains` filter already follows. Nothing the reader typed reaches the
// statement as SQL: it is a string literal in every mode, and the mode itself is a token from a
// closed list — an unknown one is refused here rather than pasted in.
func (d Dialect) MatchAny(match domain.TextMatch, columns []driver.Column) (string, error) {
	if err := match.Validate(); err != nil {
		return "", err
	}
	if len(columns) == 0 {
		// A table with no columns holds no text, and a condition nothing satisfies says exactly
		// that — the same answer a row with nothing to find it by gets.
		return " WHERE 1 = 0", nil
	}

	conditions := make([]string, 0, len(columns))
	for _, column := range columns {
		text := "CAST(" + d.QuoteIdent(column.Name) + " AS " + d.TextType + ")"
		conditions = append(conditions, d.matches(text, match))
	}
	return " WHERE (" + strings.Join(conditions, " OR ") + ")", nil
}

// SelectMatching is the read the count was made through, as the reader would run it: the same
// clause, in front of it the rows themselves instead of a count of them. The table arrives already
// qualified, because which half of a Namespace qualifies one is the engine's own business.
//
// It is here rather than in either adapter so that the number a search reports and the statement a
// reader opens are spelled in one place from one clause.
func (d Dialect) SelectMatching(table, where string) string {
	return "SELECT * FROM " + table + where
}

// matches is one column against the reader's text: a LIKE for the two modes a pattern can say, and
// a regular expression for the two it cannot — or for all four on an engine whose LIKE cannot
// answer the case box at all.
func (d Dialect) matches(text string, match domain.TextMatch) string {
	if match.IsPattern() && !d.MatchesByRegex {
		return d.like(text, match)
	}
	return d.regex(text, match)
}

// like spells a pattern match, and is where the reader's Case Insensitive box becomes SQL. Folding
// both sides is one spelling that serves every engine; the *sensitive* match is the one that
// differs, an engine whose collation ignores case having to say so — MySQL by comparing the pattern
// as bytes, SQL Server by naming a collation that tells them apart.
func (d Dialect) like(text string, match domain.TextMatch) string {
	pattern := d.QuoteString(match.Pattern())

	if match.CaseInsensitive {
		return "LOWER(" + text + ") LIKE LOWER(" + pattern + ")"
	}
	if d.SensitiveCast != "" {
		return text + " LIKE CAST(" + pattern + " AS " + d.SensitiveCast + ")"
	}
	if d.SensitiveCollation != "" {
		return text + " COLLATE " + d.SensitiveCollation + " LIKE " + pattern
	}
	return text + " LIKE " + pattern
}

// regex spells an expression match. The expression itself is domain's — one rule, so Whole Word
// means the same thing here as it does of a name — and only the *engine's* three words for it are
// here: a function whose third argument is the sensitivity (MySQL, Oracle, SQL Server), an operator
// per answer (PostgreSQL), and one operator whose pattern carries the answer itself (SQLite, whose
// `regexp` the adapter supplies and which is therefore Go's own).
func (d Dialect) regex(text string, match domain.TextMatch) string {
	expression := match.Expression(d.WordBoundary)

	if d.RegexFlagsInPattern {
		if match.CaseInsensitive {
			expression = "(?i)" + expression
		}
		return text + " REGEXP " + d.QuoteString(expression)
	}

	pattern := d.QuoteString(expression)
	if d.RegexIsFunction {
		return "REGEXP_LIKE(" + text + ", " + pattern + ", " + d.QuoteString(regexFlags(match)) + ")"
	}
	if match.CaseInsensitive {
		return text + " ~* " + pattern
	}
	return text + " ~ " + pattern
}

// regexFlags is the match type those engines take: `i` ignores case and `c` insists on it. Both are
// spelled out rather than one being left to the collation, so the box the reader ticked decides.
func regexFlags(match domain.TextMatch) string {
	if match.CaseInsensitive {
		return "i"
	}
	return "c"
}
