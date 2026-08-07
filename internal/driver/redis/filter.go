package redis

import (
	"sort"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A filter on a key space is answered **here** and not by the server, and that is the one thing
// about this file worth knowing. Redis narrows a read exactly one way — a glob against the key's
// name — and has no opinion at all about a value, a clock or a cost. So the glob is taken where the
// reader's filter can be spelled as one, because it is the only work the server will do, and the
// whole tree is then applied to what came back.
//
// Nothing here is quoted into anything. A pattern crosses as an *argument*, which is the same
// property the MongoDB adapter has for the same reason: there is no statement for a reader's text to
// escape into.

// columns is the set of columns a filter and a sort between them mention, which is what decides how
// much of each key has to be read: a filter on the name alone is answered from the scan.
type columns map[string]bool

// beyondName says something wants more of a key than its name, which is the difference between a
// read that asks the server nothing further and one that asks it three questions per key.
func (c columns) beyondName() bool {
	for name := range c {
		if name != columnKey {
			return true
		}
	}
	return false
}

// canonical is the column a filter names, however the caller spelled it. It is forgiving about case
// and about nothing else: a column a key space has not got is refused rather than quietly ignored,
// because a filter that matched everything would read as a filter that found everything.
func canonical(name string) (string, bool) {
	for _, column := range keyColumns {
		if strings.EqualFold(strings.TrimSpace(name), column.Name) {
			return column.Name, true
		}
	}
	return "", false
}

func unknownColumn(name string) error {
	return domain.ValidationError{
		Field:   "column",
		Message: name + " is not a column of a key space",
	}
}

// wants collects what the filter and the sort need read. It is also where both are *validated*, so a
// column or an operator this side does not know is refused before a single key is scanned rather
// than after a million of them are.
func wants(filter driver.TableGroup, sorts []driver.TableSort) (columns, error) {
	wanted := columns{}

	var walk func(criterion driver.TableCriterion) error
	walk = func(criterion driver.TableCriterion) error {
		switch node := criterion.(type) {
		case driver.TableFilter:
			name, ok := canonical(node.Column)
			if !ok {
				return unknownColumn(node.Column)
			}
			if err := checkOperator(node); err != nil {
				return err
			}
			wanted[name] = true
		case driver.TableGroup:
			// A conjunction is only asked about where there is something to join, which is the same
			// point `sqlkit.Dialect.group` asks at: a group of one is that one condition, and an
			// unfiltered read carries a group of none.
			joined := node.Conjunction == driver.ConjunctionAnd || node.Conjunction == driver.ConjunctionOr
			if len(node.Children) > 1 && !joined {
				return domain.ValidationError{
					Field:   "conjunction",
					Message: "is not a conjunction this app knows",
				}
			}
			for _, child := range node.Children {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(filter); err != nil {
		return nil, err
	}

	for _, one := range sorts {
		name, ok := canonical(one.Column)
		if !ok {
			return nil, unknownColumn(one.Column)
		}
		wanted[name] = true
	}
	return wanted, nil
}

// operands is how many values each operator takes, which is what refuses half a BETWEEN. It is the
// same list `sqlkit.Dialect.Where` and the MongoDB filter refuse from, and for the same reason: an
// operator given the wrong number of values is a filter nobody meant.
var operands = map[driver.TableFilterOp]int{
	driver.FilterEqual: 1, driver.FilterNotEqual: 1,
	driver.FilterGreater: 1, driver.FilterGreaterEqual: 1,
	driver.FilterLess: 1, driver.FilterLessEqual: 1,
	driver.FilterContains: 1, driver.FilterNotContains: 1,
	driver.FilterBegins: 1, driver.FilterNotBegins: 1,
	driver.FilterEnds: 1, driver.FilterNotEnds: 1,
	driver.FilterMatches: 1, driver.FilterContainsFold: 1,
	driver.FilterNull: 0, driver.FilterNotNull: 0,
	driver.FilterEmpty: 0, driver.FilterNotEmpty: 0,
	driver.FilterBetween: 2, driver.FilterNotBetween: 2,
}

func checkOperator(filter driver.TableFilter) error {
	switch filter.Op {
	case driver.FilterInList, driver.FilterNotInList:
		if len(filter.Values) == 0 {
			return domain.ValidationError{Field: "values", Message: "a list needs a value in it"}
		}
		return nil
	}

	needed, known := operands[filter.Op]
	if !known {
		return domain.ValidationError{
			Field:   "operator",
			Message: string(filter.Op) + " is not an operator this app knows",
		}
	}
	if len(filter.Values) < needed {
		return domain.ValidationError{
			Field:   "values",
			Message: string(filter.Op) + " needs " + strconv.Itoa(needed) + " of them",
		}
	}
	return nil
}

// glob is the part of the reader's filter the *server* can answer, and `*` where there is none.
//
// Only the top level of an AND is read, and only a filter on the key's name. That is not a
// simplification but the rule: a condition inside an OR narrows nothing on its own — `key begins
// user OR type is hash` admits every hash in the database — so taking a glob from one would hide
// rows the reader asked for. Everything else about the tree is applied afterwards, to the keys this
// let through.
func glob(filter driver.TableGroup) string {
	if filter.Conjunction == driver.ConjunctionOr && len(filter.Children) > 1 {
		return "*"
	}

	for _, child := range filter.Children {
		one, ok := child.(driver.TableFilter)
		if !ok {
			continue
		}
		if name, known := canonical(one.Column); !known || name != columnKey {
			continue
		}
		if len(one.Values) == 0 {
			continue
		}

		text := one.Values[0]
		switch one.Op {
		case driver.FilterEqual:
			return escapeGlob(text)
		case driver.FilterContains:
			return "*" + escapeGlob(text) + "*"
		case driver.FilterContainsFold:
			return "*" + foldGlob(text) + "*"
		case driver.FilterBegins:
			return escapeGlob(text) + "*"
		case driver.FilterEnds:
			return "*" + escapeGlob(text)
		case driver.FilterMatches:
			// The one operator whose value is **not** escaped, that being the whole of what it says:
			// the reader is spelling the server's own glob, and `*email*` is three characters of
			// pattern and five of text.
			return text
		}
	}

	return "*"
}

// field is one column of a row as a filter reads it: nil where the row holds nothing, which is a key
// with no expiry, a server that would not say what a key costs, and a value nothing read.
func (r keyRow) field(column string) *string {
	switch column {
	case columnKey:
		return cell(r.name)
	case columnType:
		return cell(r.kind)
	case columnValue:
		return optional(r.valued, r.value)
	case columnTTL:
		return seconds(r.ttl)
	case columnSize:
		return number(r.size)
	}
	return nil
}

// numeric says a column is compared as a number rather than as text, which is the difference between
// a TTL of 9 coming before one of 10 and after it.
func numeric(column string) bool {
	return column == columnTTL || column == columnSize
}

// matches applies the whole tree to one row. A group of nothing narrows nothing — which is what an
// unfiltered read carries — and the two conjunctions are the only ones there are.
func matches(row keyRow, group driver.TableGroup) (bool, error) {
	if len(group.Children) == 0 {
		return true, nil
	}

	for _, child := range group.Children {
		var (
			ok  bool
			err error
		)
		switch node := child.(type) {
		case driver.TableFilter:
			ok, err = matchesFilter(row, node)
		case driver.TableGroup:
			ok, err = matches(row, node)
		default:
			return false, domain.ValidationError{
				Field:   "filter",
				Message: "is not a filter this app knows",
			}
		}
		if err != nil {
			return false, err
		}

		if group.Conjunction == driver.ConjunctionOr {
			if ok {
				return true, nil
			}
			continue
		}
		if !ok {
			return false, nil
		}
	}

	return group.Conjunction != driver.ConjunctionOr, nil
}

// matchesFilter is one condition against one row. **A NULL satisfies nothing but IS NULL**, which is
// the rule every SQL engine has and the reason the two are asked first: a key with no expiry is not
// a key whose expiry is less than ten.
func matchesFilter(row keyRow, filter driver.TableFilter) (bool, error) {
	column, ok := canonical(filter.Column)
	if !ok {
		return false, unknownColumn(filter.Column)
	}
	if err := checkOperator(filter); err != nil {
		return false, err
	}

	held := row.field(column)
	switch filter.Op {
	case driver.FilterNull:
		return held == nil, nil
	case driver.FilterNotNull:
		return held != nil, nil
	}
	if held == nil {
		return false, nil
	}

	value := *held
	compare := func(against string) int { return compareValues(value, against, numeric(column)) }

	switch filter.Op {
	case driver.FilterEmpty:
		return value == "", nil
	case driver.FilterNotEmpty:
		return value != "", nil
	case driver.FilterEqual:
		return compare(filter.Values[0]) == 0, nil
	case driver.FilterNotEqual:
		return compare(filter.Values[0]) != 0, nil
	case driver.FilterGreater:
		return compare(filter.Values[0]) > 0, nil
	case driver.FilterGreaterEqual:
		return compare(filter.Values[0]) >= 0, nil
	case driver.FilterLess:
		return compare(filter.Values[0]) < 0, nil
	case driver.FilterLessEqual:
		return compare(filter.Values[0]) <= 0, nil
	case driver.FilterContains:
		return strings.Contains(value, filter.Values[0]), nil
	case driver.FilterContainsFold:
		return strings.Contains(strings.ToLower(value), strings.ToLower(filter.Values[0])), nil
	case driver.FilterMatches:
		return globMatch(filter.Values[0], value), nil
	case driver.FilterNotContains:
		return !strings.Contains(value, filter.Values[0]), nil
	case driver.FilterBegins:
		return strings.HasPrefix(value, filter.Values[0]), nil
	case driver.FilterNotBegins:
		return !strings.HasPrefix(value, filter.Values[0]), nil
	case driver.FilterEnds:
		return strings.HasSuffix(value, filter.Values[0]), nil
	case driver.FilterNotEnds:
		return !strings.HasSuffix(value, filter.Values[0]), nil
	case driver.FilterBetween:
		return compare(filter.Values[0]) >= 0 && compare(filter.Values[1]) <= 0, nil
	case driver.FilterNotBetween:
		return compare(filter.Values[0]) < 0 || compare(filter.Values[1]) > 0, nil
	case driver.FilterInList, driver.FilterNotInList:
		found := false
		for _, one := range filter.Values {
			if compare(one) == 0 {
				found = true
				break
			}
		}
		return found == (filter.Op == driver.FilterInList), nil
	}

	return false, domain.ValidationError{
		Field:   "operator",
		Message: string(filter.Op) + " is not an operator this app knows",
	}
}

// compareValues is the one comparison every operator above is built on. A numeric column compares as
// a number *when both sides are one*, and as text otherwise, so a reader who typed `soon` into the
// TTL box gets no match rather than an error.
func compareValues(left, right string, asNumber bool) int {
	if asNumber {
		first, leftErr := strconv.ParseFloat(left, 64)
		second, rightErr := strconv.ParseFloat(right, 64)
		if leftErr == nil && rightErr == nil {
			switch {
			case first < second:
				return -1
			case first > second:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(left, right)
}

// order sorts the rows the reader is going to page through, the first key deciding and the rest
// breaking its ties. A read carrying no sort is already in key order — which is the order a scan's
// answer is put into before anything else happens to it, because a page cut from an unordered list
// is a different page every time it is read.
func order(rows []keyRow, sorts []driver.TableSort) error {
	if len(sorts) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sorts))
	for _, one := range sorts {
		column, ok := canonical(one.Column)
		if !ok {
			return unknownColumn(one.Column)
		}
		keys = append(keys, column)
	}

	sort.SliceStable(rows, func(a, b int) bool {
		for i, column := range keys {
			ranking := rank(rows[a], rows[b], column)
			if ranking == 0 {
				continue
			}
			if sorts[i].Descending {
				return ranking > 0
			}
			return ranking < 0
		}
		return false
	})
	return nil
}

// rank orders two rows by one column, **a row holding nothing last** whichever way the sort runs:
// a key with no expiry is not a key that expires first, and burying the absence at the end of an
// ascending read is what every engine does with a NULL by default.
func rank(left, right keyRow, column string) int {
	first, second := left.field(column), right.field(column)
	switch {
	case first == nil && second == nil:
		return 0
	case first == nil:
		return 1
	case second == nil:
		return -1
	}
	return compareValues(*first, *second, numeric(column))
}
