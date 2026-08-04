package sqlkit

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// comparisons is every operator that takes a value and compares it, spelled for both engines.
// The three that do not — IS NULL, IS NOT NULL and the text match — are cases of their own in
// Where, because none of them is `<column> <operator> <value>`.
var comparisons = map[driver.TableFilterOp]string{
	driver.FilterEqual:        "=",
	driver.FilterNotEqual:     "<>",
	driver.FilterGreater:      ">",
	driver.FilterGreaterEqual: ">=",
	driver.FilterLess:         "<",
	driver.FilterLessEqual:    "<=",
}

// Where spells the conditions a table read is narrowed by. The clause leads with a space and is
// empty for a read that filters nothing, so a caller concatenates it exactly as it does OrderBy;
// every filter holds at once, which is what AND is doing there.
//
// A value is written in as a string literal rather than bound, for the reason a limit is: one
// string is then both what runs and what the reader is shown, and a statement carrying `?` is
// not one they could copy into an editor. What stands between a reader's text and the statement
// is therefore QuoteString, and the column and the operator are checked rather than quoted — a
// name the table does not have, or an operator this side does not know, is refused here instead
// of reaching the server as SQL nobody meant. Both engines read a string literal back into a
// column of any type, so one spelling serves a name, a number and a date alike.
func (d Dialect) Where(filters []driver.TableFilter, columns []driver.Column) (string, error) {
	conditions := make([]string, 0, len(filters))

	for _, filter := range filters {
		if !named(columns, filter.Column) {
			return "", domain.ValidationError{
				Field:   "filter",
				Message: "names no column of this table",
			}
		}
		column := d.QuoteIdent(filter.Column)

		switch filter.Op {
		case driver.FilterNull:
			conditions = append(conditions, column+" IS NULL")
		case driver.FilterNotNull:
			conditions = append(conditions, column+" IS NOT NULL")
		case driver.FilterContains:
			// Read as text, so a substring means the same thing in a number as in a name. A
			// wildcard the reader typed is their own: this is the one condition that takes them.
			conditions = append(conditions,
				"CAST("+column+" AS "+d.TextType+") LIKE "+d.QuoteString("%"+filter.Value+"%"))
		default:
			operator, ok := comparisons[filter.Op]
			if !ok {
				return "", domain.ValidationError{
					Field:   "filter",
					Message: "is not a comparison this side knows",
				}
			}
			conditions = append(conditions, column+" "+operator+" "+d.QuoteString(filter.Value))
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), nil
}
