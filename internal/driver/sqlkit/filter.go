package sqlkit

import (
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// conjunctions is how a group joins what is under it. A conjunction this side does not know is
// refused rather than defaulted to AND: a read narrowed the wrong way looks like data.
var conjunctions = map[driver.TableConjunction]string{
	driver.ConjunctionAnd: " AND ",
	driver.ConjunctionOr:  " OR ",
}

// comparisons is every operator that compares one value, spelled for both engines.
var comparisons = map[driver.TableFilterOp]string{
	driver.FilterEqual:        "=",
	driver.FilterNotEqual:     "<>",
	driver.FilterGreater:      ">",
	driver.FilterGreaterEqual: ">=",
	driver.FilterLess:         "<",
	driver.FilterLessEqual:    "<=",
}

// textMatches is every operator that matches text: the LIKE it needs, and where the wildcards go
// around the reader's own value. All six read the column as text, so a prefix or a substring
// means the same thing in a number as in a name, and a `%` the reader typed is their own.
var textMatches = map[driver.TableFilterOp]struct{ match, before, after string }{
	driver.FilterContains:    {"LIKE", "%", "%"},
	driver.FilterNotContains: {"NOT LIKE", "%", "%"},
	driver.FilterBegins:      {"LIKE", "", "%"},
	driver.FilterNotBegins:   {"NOT LIKE", "", "%"},
	driver.FilterEnds:        {"LIKE", "%", ""},
	driver.FilterNotEnds:     {"NOT LIKE", "%", ""},
}

// valueless is every operator that compares against nothing, so the whole condition is the
// clause after the column's own name.
var valueless = map[driver.TableFilterOp]string{
	driver.FilterNull:     "IS NULL",
	driver.FilterNotNull:  "IS NOT NULL",
	driver.FilterEmpty:    "= ''",
	driver.FilterNotEmpty: "<> ''",
}

// ranges takes two values, one on each side of its AND.
var ranges = map[driver.TableFilterOp]string{
	driver.FilterBetween:    "BETWEEN",
	driver.FilterNotBetween: "NOT BETWEEN",
}

// lists takes any number of values above none.
var lists = map[driver.TableFilterOp]string{
	driver.FilterInList:    "IN",
	driver.FilterNotInList: "NOT IN",
}

// Where spells the tree a table read is narrowed by. The clause leads with a space and is empty
// for a read that filters nothing, so a caller concatenates it exactly as it does OrderBy.
//
// A value is written in as a string literal rather than bound, for the reason a limit is: one
// string is then both what runs and what the reader is shown, and a statement carrying `?` is
// not one they could copy into an editor. What stands between a reader's text and the statement
// is therefore QuoteString, and the column, the operator and the conjunction are checked rather
// than quoted — a name the table does not have, or a token this side does not know, is refused
// here instead of reaching the server as SQL nobody meant. Both engines read a string literal
// back into a column of any type, so one spelling serves a name, a number and a date alike.
func (d Dialect) Where(filter driver.TableGroup, columns []driver.Column) (string, error) {
	// The root is the whole clause, so it needs no parentheses of its own: only a group nested
	// inside another does, and that is where criterion puts them.
	clause, _, err := d.group(filter, columns)
	if err != nil || clause == "" {
		return "", err
	}
	return " WHERE " + clause, nil
}

// group joins its children with its own conjunction, and says whether it joined anything at all —
// a group that rendered one condition reads as that condition, so its caller knows not to
// parenthesise it. An empty group is empty rather than `()`, which is also why its conjunction
// goes unread: an unfiltered read carries the zero group and names no conjunction.
func (d Dialect) group(
	group driver.TableGroup,
	columns []driver.Column,
) (clause string, joined bool, err error) {
	parts := make([]string, 0, len(group.Children))

	for _, child := range group.Children {
		one, err := d.criterion(child, columns)
		if err != nil {
			return "", false, err
		}
		if one != "" {
			parts = append(parts, one)
		}
	}

	if len(parts) == 0 {
		return "", false, nil
	}
	if len(parts) == 1 {
		return parts[0], false, nil
	}

	join, ok := conjunctions[group.Conjunction]
	if !ok {
		return "", false, filterError("is joined by nothing this side knows")
	}
	return strings.Join(parts, join), true, nil
}

// criterion is where a nested group earns its parentheses, which are the whole point of one: they
// hold the precedence the reader drew against whatever conjunction the group above uses.
func (d Dialect) criterion(node driver.TableCriterion, columns []driver.Column) (string, error) {
	switch node := node.(type) {
	case driver.TableGroup:
		clause, joined, err := d.group(node, columns)
		if err != nil || !joined {
			return clause, err
		}
		return "(" + clause + ")", nil
	case driver.TableFilter:
		return d.filter(node, columns)
	default:
		return "", filterError("is neither a condition nor a group")
	}
}

func (d Dialect) filter(filter driver.TableFilter, columns []driver.Column) (string, error) {
	if !named(columns, filter.Column) {
		return "", filterError("names no column of this table")
	}
	column := d.QuoteIdent(filter.Column)

	if clause, ok := valueless[filter.Op]; ok {
		return column + " " + clause, nil
	}

	if operator, ok := comparisons[filter.Op]; ok {
		values, err := exactly(filter, 1)
		if err != nil {
			return "", err
		}
		return column + " " + operator + " " + d.QuoteString(values[0]), nil
	}

	if match, ok := textMatches[filter.Op]; ok {
		values, err := exactly(filter, 1)
		if err != nil {
			return "", err
		}
		return "CAST(" + column + " AS " + d.TextType + ") " + match.match + " " +
			d.QuoteString(match.before+values[0]+match.after), nil
	}

	if operator, ok := ranges[filter.Op]; ok {
		values, err := exactly(filter, 2)
		if err != nil {
			return "", err
		}
		return column + " " + operator + " " +
			d.QuoteString(values[0]) + " AND " + d.QuoteString(values[1]), nil
	}

	if operator, ok := lists[filter.Op]; ok {
		if len(filter.Values) == 0 {
			return "", filterError("lists no value to compare against")
		}
		quoted := make([]string, len(filter.Values))
		for i, value := range filter.Values {
			quoted[i] = d.QuoteString(value)
		}
		return column + " " + operator + " (" + strings.Join(quoted, ", ") + ")", nil
	}

	return "", filterError("is not a comparison this side knows")
}

// exactly reads the values an operator takes, and refuses a condition carrying the wrong number
// of them: half a BETWEEN is not a comparison, and a reader who has not finished spelling one is
// better told so than shown rows they did not ask for.
func exactly(filter driver.TableFilter, wanted int) ([]string, error) {
	if len(filter.Values) != wanted {
		return nil, filterError("does not carry the values its operator compares")
	}
	return filter.Values, nil
}

// DistinctValues spells the read behind a filter's value list: what one column actually holds,
// deduplicated, in order, and capped. NULL is left out, a reader reaching it with the IS NULL
// operator rather than by picking it from a list, and the column is checked against the table's
// own rather than quoted in blind, exactly as a filter's is. The table arrives already qualified,
// because which half of a Namespace qualifies one is the engine's own business.
func (d Dialect) DistinctValues(
	table, column string,
	columns []driver.Column,
	limit int,
) (string, error) {
	if !named(columns, column) {
		return "", domain.ValidationError{
			Field:   "column",
			Message: "names no column of this table",
		}
	}

	quoted := d.QuoteIdent(column)
	return "SELECT DISTINCT " + quoted +
		" FROM " + table +
		" WHERE " + quoted + " IS NOT NULL" +
		" ORDER BY 1 LIMIT " + strconv.Itoa(limit), nil
}

func filterError(message string) error {
	return domain.ValidationError{Field: "filter", Message: message}
}
