package mongo

import (
	"regexp"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// comparisons is every operator that compares one value, spelled as the server's own.
var comparisons = map[driver.TableFilterOp]string{
	driver.FilterNotEqual:     "$ne",
	driver.FilterGreater:      "$gt",
	driver.FilterGreaterEqual: "$gte",
	driver.FilterLess:         "$lt",
	driver.FilterLessEqual:    "$lte",
}

// textMatches is every operator that matches text, and where the pattern's anchors go around the
// reader's own value. **The value is taken literally**: a document store's own matching is a regular
// expression, and letting a dozen metacharacters through where SQL's LIKE has two would mean a
// reader searching for `a.b` was shown `axb`. A reader who wants a pattern writes one in the query
// tab, where `$regex` is theirs to spell.
var textMatches = map[driver.TableFilterOp]struct {
	before, after string
	negated       bool
}{
	driver.FilterContains:    {"", "", false},
	driver.FilterNotContains: {"", "", true},
	driver.FilterBegins:      {"^", "", false},
	driver.FilterNotBegins:   {"^", "", true},
	driver.FilterEnds:        {"", "$", false},
	driver.FilterNotEnds:     {"", "$", true},
}

// lists takes any number of values above none.
var lists = map[driver.TableFilterOp]string{
	driver.FilterInList:    "$in",
	driver.FilterNotInList: "$nin",
}

// filterDoc spells the tree a collection read is narrowed by, as the document the server takes. It
// is `sqlkit.Dialect.Where`'s counterpart and refuses the same things for the same reasons: a field
// the collection has never held, an operator this side does not know, an operator carrying the wrong
// number of values. A group of nothing narrows nothing, which is what an unfiltered read carries.
//
// Nothing a reader typed is ever spelled into a query. A filter travels as a *document*, so the
// text is a value and can only ever be one — which is the whole of why a document store needs no
// quoting rules at all.
func filterDoc(group driver.TableGroup, fields []driver.Column) (bson.D, error) {
	document, _, err := groupDoc(group, fields)
	return document, err
}

// groupDoc joins its children with its own conjunction, and says whether it joined anything: a
// group that rendered one condition *is* that condition, so a lone child is not wrapped in a `$and`
// nobody wrote.
func groupDoc(
	group driver.TableGroup,
	fields []driver.Column,
) (document bson.D, joined bool, err error) {
	parts := make([]any, 0, len(group.Children))

	for _, child := range group.Children {
		one, err := criterionDoc(child, fields)
		if err != nil {
			return nil, false, err
		}
		if len(one) > 0 {
			parts = append(parts, one)
		}
	}

	switch {
	case len(parts) == 0:
		return bson.D{}, false, nil
	case len(parts) == 1:
		return parts[0].(bson.D), false, nil
	}

	operator, ok := conjunctions[group.Conjunction]
	if !ok {
		return nil, false, filterError("is joined by nothing this side knows")
	}
	return bson.D{{Key: operator, Value: bson.A(parts)}}, true, nil
}

// conjunctions is how a group joins what is under it. One this side does not know is refused rather
// than defaulted to `$and`: a read narrowed the wrong way looks like data.
var conjunctions = map[driver.TableConjunction]string{
	driver.ConjunctionAnd: "$and",
	driver.ConjunctionOr:  "$or",
}

func criterionDoc(node driver.TableCriterion, fields []driver.Column) (bson.D, error) {
	switch node := node.(type) {
	case driver.TableGroup:
		document, _, err := groupDoc(node, fields)
		return document, err
	case driver.TableFilter:
		return conditionDoc(node, fields)
	default:
		return nil, filterError("is neither a condition nor a group")
	}
}

func conditionDoc(filter driver.TableFilter, fields []driver.Column) (bson.D, error) {
	held, ok := field(fields, filter.Column)
	if !ok {
		return nil, filterError("names no field of this collection")
	}
	name := filter.Column

	// A missing field and an explicit null are one answer in a document store, and `null` is the
	// query that gives it: `{a: null}` finds both, and `{a: {$ne: null}}` finds neither.
	switch filter.Op {
	case driver.FilterNull:
		return bson.D{{Key: name, Value: nil}}, nil
	case driver.FilterNotNull:
		return bson.D{{Key: name, Value: bson.D{{Key: "$ne", Value: nil}}}}, nil
	case driver.FilterEmpty:
		return bson.D{{Key: name, Value: ""}}, nil
	case driver.FilterNotEmpty:
		return bson.D{{Key: name, Value: bson.D{{Key: "$ne", Value: ""}}}}, nil
	}

	// Equality is written bare rather than as `$eq`, which is how it also matches an *element* of
	// an array field — the one behaviour a document store has that SQL has no way to ask for.
	if filter.Op == driver.FilterEqual {
		values, err := exactly(filter, 1)
		if err != nil {
			return nil, err
		}
		return bson.D{{Key: name, Value: coerce(values[0], held.Type)}}, nil
	}

	if operator, ok := comparisons[filter.Op]; ok {
		values, err := exactly(filter, 1)
		if err != nil {
			return nil, err
		}
		return bson.D{{
			Key:   name,
			Value: bson.D{{Key: operator, Value: coerce(values[0], held.Type)}},
		}}, nil
	}

	if match, ok := textMatches[filter.Op]; ok {
		values, err := exactly(filter, 1)
		if err != nil {
			return nil, err
		}
		pattern := bson.Regex{
			Pattern: match.before + regexp.QuoteMeta(values[0]) + match.after,
			Options: "i",
		}
		if match.negated {
			return bson.D{{Key: name, Value: bson.D{{Key: "$not", Value: pattern}}}}, nil
		}
		return bson.D{{Key: name, Value: pattern}}, nil
	}

	if filter.Op == driver.FilterBetween || filter.Op == driver.FilterNotBetween {
		values, err := exactly(filter, 2)
		if err != nil {
			return nil, err
		}
		low, high := coerce(values[0], held.Type), coerce(values[1], held.Type)
		span := bson.D{{Key: "$gte", Value: low}, {Key: "$lte", Value: high}}

		if filter.Op == driver.FilterBetween {
			return bson.D{{Key: name, Value: span}}, nil
		}
		// `$not` over a range is the one spelling that keeps a document missing the field out of
		// neither side, which is what `NOT BETWEEN` means.
		return bson.D{{Key: name, Value: bson.D{{Key: "$not", Value: span}}}}, nil
	}

	if operator, ok := lists[filter.Op]; ok {
		if len(filter.Values) == 0 {
			return nil, filterError("lists no value to compare against")
		}
		values := make(bson.A, len(filter.Values))
		for i, value := range filter.Values {
			values[i] = coerce(value, held.Type)
		}
		return bson.D{{Key: name, Value: bson.D{{Key: operator, Value: values}}}}, nil
	}

	return nil, filterError("is not a comparison this side knows")
}

// exactly reads the values an operator takes, and refuses a condition carrying the wrong number of
// them, exactly as `sqlkit` does: half a BETWEEN is not a comparison.
func exactly(filter driver.TableFilter, wanted int) ([]string, error) {
	if len(filter.Values) != wanted {
		return nil, filterError("does not carry the values its operator compares")
	}
	return filter.Values, nil
}

// sortDoc orders a read by the keys the reader chose, the first deciding and the rest breaking its
// ties. A name the collection has never held is dropped rather than sent, so no caller can write the
// sort itself; a read choosing nothing falls back to `_id`, which is what makes a page stable — a
// document store returns documents in no order at all otherwise.
func sortDoc(sort []driver.TableSort, fields []driver.Column) bson.D {
	keys := bson.D{}
	for _, one := range sort {
		if _, ok := field(fields, one.Column); !ok {
			continue
		}
		direction := 1
		if one.Descending {
			direction = -1
		}
		keys = append(keys, bson.E{Key: one.Column, Value: direction})
	}

	if len(keys) == 0 {
		return bson.D{{Key: idField, Value: 1}}
	}
	return keys
}

// coerce reads the text a reader typed as the type the field actually holds. It is what stands
// where a SQL engine's own cast stands: the grid deals in text and a document store does not, so
// `"5"` typed against an `int` field has to become 5 or match nothing at all.
//
// A document or an array is read as the shell notation the grid drew it in, so a nested value can be
// filtered and edited in the same spelling it was shown.
//
// **A field the sample never saw is read as the shell would read it**, which is the only answer left:
// the first document of an empty collection carries fields nothing knows the type of, and `36` there
// has to be the number a reader plainly meant. It is the same rule the query tab uses, so one
// spelling means one value throughout — and a reader who means the *text* of a number quotes it. A
// filter never reaches this: one naming a field the collection has never held is refused above.
func coerce(value, kind string) any {
	switch kind {
	case "":
		return shellValue(value)

	case "objectId":
		if id, err := bson.ObjectIDFromHex(value); err == nil {
			return id
		}

	case "int", "long":
		if whole, err := strconv.ParseInt(value, 10, 64); err == nil {
			if kind == "int" && whole >= -2147483648 && whole <= 2147483647 {
				return int32(whole)
			}
			return whole
		}

	case "double":
		if real, err := strconv.ParseFloat(value, 64); err == nil {
			return real
		}

	case "decimal":
		if number, err := bson.ParseDecimal128(value); err == nil {
			return number
		}

	case "bool":
		if yes, err := strconv.ParseBool(value); err == nil {
			return yes
		}

	case "date":
		if when, err := parseDate(value); err == nil {
			return when
		}

	case "binData":
		if data, err := binaryData(value); err == nil {
			return data
		}

	case "string":
		// Drawn bare and read back bare. A string field holding `36` is the *text* of a number, and
		// reading it as one would silently store something else.
		return value
	}

	// The grid drew this cell with `literal`, so the shell's own grammar reads back exactly what it
	// showed — `ObjectId("…")`, `ISODate("…")`, `{ a: 1 }`. It is tried *after* the bare forms above,
	// because a reader typing a hex id or `2024-01-31` by hand writes no constructor around it, and
	// tried at all because without it a cell the grid drew could not be edited in the notation it was
	// drawn in.
	if parsed, err := parseValue(value); err == nil {
		return parsed
	}
	return value
}

// binaryData reads the `\x…` hex a binary field was drawn as, which is the form bytes that are not
// text survive the trip to the grid in — see sqlkit.TextOrNil, whose spelling this is. It names the
// **plain** subtype and only that one, the hex carrying none: a value with a subtype is drawn as its
// own constructor instead, and reaches this function's caller through parseValue — see cell.
func binaryData(value string) (bson.Binary, error) {
	if !strings.HasPrefix(value, sqlkit.BinaryPrefix) {
		return bson.Binary{}, statementError("binary data is written as " + sqlkit.BinaryPrefix + "hex")
	}
	data, err := hexBytes(value[len(sqlkit.BinaryPrefix):])
	if err != nil {
		return bson.Binary{}, err
	}
	return bson.Binary{Subtype: bson.TypeBinaryGeneric, Data: data}, nil
}

// matchDoc is a Find in Schema's data half as the document the server takes: any of the fields the
// collection is known to hold matching the reader's text.
//
// **A regular expression is the only shape it has.** A document store has no LIKE, so every mode
// arrives here already spelled as an expression by the one rule the SQL engines use too
// (domain.TextMatch.Expression) — and what it can match is what `$regex` matches: a field holding a
// **string**. A number is not text to a document store, and nothing here pretends otherwise by
// converting one.
func matchDoc(match domain.TextMatch, fields []driver.Column) (bson.D, error) {
	if err := match.Validate(); err != nil {
		return nil, err
	}

	// `s` so a dot reaches a newline, and `i` for the box the reader ticked — the two flags the
	// same rule needs of every engine.
	options := "s"
	if match.CaseInsensitive {
		options = "is"
	}

	any := make(bson.A, 0, len(fields))
	for _, one := range fields {
		any = append(any, bson.D{{Key: one.Name, Value: bson.D{
			{Key: "$regex", Value: match.Expression(regexBoundary)},
			{Key: "$options", Value: options},
		}}})
	}

	// A collection whose sample turned up no fields at all holds nothing to look in, and a filter
	// no document satisfies says so — the same answer a row with nothing to find it by gets.
	if len(any) == 0 {
		return bson.D{{Key: "$expr", Value: false}}, nil
	}
	return bson.D{{Key: "$or", Value: any}}, nil
}

// regexBoundary is the word edge in the server's own flavour of expression, which is the same one
// MySQL's ICU reads and not PostgreSQL's.
const regexBoundary = `\b`

func field(fields []driver.Column, name string) (driver.Column, bool) {
	for _, one := range fields {
		if one.Name == name {
			return one, true
		}
	}
	return driver.Column{}, false
}

func filterError(message string) error {
	return domain.ValidationError{Field: "filter", Message: message}
}
