package mongo

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// fields is what the sample found: an ObjectId key, a number, some text and a flag, which is enough
// to show that a value the reader typed is read as the type the field actually holds.
var sampled = []driver.Column{
	{Name: "_id", Type: "objectId", PrimaryKey: true},
	{Name: "age", Type: "int"},
	{Name: "name", Type: "string"},
	{Name: "live", Type: "bool"},
	{Name: "joined", Type: "date"},
}

func TestFilterDocSpellsEveryOperator(t *testing.T) {
	tests := []struct {
		op     driver.TableFilterOp
		column string
		values []string
		want   string
	}{
		{driver.FilterEqual, "age", []string{"30"}, `{ age: 30 }`},
		{driver.FilterNotEqual, "age", []string{"30"}, `{ age: { $ne: 30 } }`},
		{driver.FilterGreater, "age", []string{"30"}, `{ age: { $gt: 30 } }`},
		{driver.FilterGreaterEqual, "age", []string{"30"}, `{ age: { $gte: 30 } }`},
		{driver.FilterLess, "age", []string{"30"}, `{ age: { $lt: 30 } }`},
		{driver.FilterLessEqual, "age", []string{"30"}, `{ age: { $lte: 30 } }`},

		{driver.FilterContains, "name", []string{"an"}, `{ name: /an/i }`},
		{driver.FilterNotContains, "name", []string{"an"}, `{ name: { $not: /an/i } }`},
		{driver.FilterBegins, "name", []string{"an"}, `{ name: /^an/i }`},
		{driver.FilterEnds, "name", []string{"an"}, `{ name: /an$/i }`},

		// A missing field and an explicit null are one answer in a document store, and `null` is
		// the query that gives it.
		{driver.FilterNull, "name", nil, `{ name: null }`},
		{driver.FilterNotNull, "name", nil, `{ name: { $ne: null } }`},
		{driver.FilterEmpty, "name", nil, `{ name: "" }`},
		{driver.FilterNotEmpty, "name", nil, `{ name: { $ne: "" } }`},

		{driver.FilterBetween, "age", []string{"20", "30"}, `{ age: { $gte: 20, $lte: 30 } }`},
		{driver.FilterNotBetween, "age", []string{"20", "30"},
			`{ age: { $not: { $gte: 20, $lte: 30 } } }`},

		{driver.FilterInList, "age", []string{"20", "30"}, `{ age: { $in: [20, 30] } }`},
		{driver.FilterNotInList, "age", []string{"20", "30"}, `{ age: { $nin: [20, 30] } }`},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			group := driver.TableGroup{
				Children: []driver.TableCriterion{
					driver.TableFilter{Column: tt.column, Op: tt.op, Values: tt.values},
				},
			}

			document, err := filterDoc(group, sampled)
			if err != nil {
				t.Fatalf("filterDoc() error = %v", err)
			}
			if got := documentText(document); got != tt.want {
				t.Errorf("filterDoc() = %s, want %s", got, tt.want)
			}
		})
	}
}

// A regular expression the reader did not write is not one they get: a `contains` of `a.b` finds
// `a.b` and not `axb`, which is what escaping the value is for.
func TestFilterDocTakesATextMatchLiterally(t *testing.T) {
	group := driver.TableGroup{
		Children: []driver.TableCriterion{
			driver.TableFilter{Column: "name", Op: driver.FilterContains, Values: []string{"a.b*"}},
		},
	}

	document, err := filterDoc(group, sampled)
	if err != nil {
		t.Fatalf("filterDoc() error = %v", err)
	}
	if got := documentText(document); got != `{ name: /a\.b\*/i }` {
		t.Errorf("filterDoc() = %s, want the value escaped", got)
	}
}

// The parentheses a reader drew are the whole point of a group, so a nested one is nested here too.
func TestFilterDocKeepsTheGroupingTheReaderDrew(t *testing.T) {
	group := driver.TableGroup{
		Conjunction: driver.ConjunctionAnd,
		Children: []driver.TableCriterion{
			driver.TableFilter{Column: "live", Op: driver.FilterEqual, Values: []string{"true"}},
			driver.TableGroup{
				Conjunction: driver.ConjunctionOr,
				Children: []driver.TableCriterion{
					driver.TableFilter{Column: "age", Op: driver.FilterLess, Values: []string{"20"}},
					driver.TableFilter{Column: "age", Op: driver.FilterGreater, Values: []string{"60"}},
				},
			},
		},
	}

	document, err := filterDoc(group, sampled)
	if err != nil {
		t.Fatalf("filterDoc() error = %v", err)
	}

	const want = `{ $and: [{ live: true }, { $or: [{ age: { $lt: 20 } }, { age: { $gt: 60 } }] }] }`
	if got := documentText(document); got != want {
		t.Errorf("filterDoc() =\n%s\nwant\n%s", got, want)
	}
}

// A group of one condition reads as that condition, so nothing is wrapped in an `$and` nobody wrote.
func TestFilterDocDoesNotWrapALoneCondition(t *testing.T) {
	group := driver.TableGroup{
		Conjunction: driver.ConjunctionAnd,
		Children: []driver.TableCriterion{
			driver.TableFilter{Column: "live", Op: driver.FilterEqual, Values: []string{"true"}},
		},
	}

	document, err := filterDoc(group, sampled)
	if err != nil {
		t.Fatalf("filterDoc() error = %v", err)
	}
	if got := documentText(document); got != `{ live: true }` {
		t.Errorf("filterDoc() = %s, want the condition alone", got)
	}
}

func TestFilterDocOfNothingNarrowsNothing(t *testing.T) {
	document, err := filterDoc(driver.TableGroup{}, sampled)
	if err != nil {
		t.Fatalf("filterDoc() error = %v", err)
	}
	if got := documentText(document); got != `{}` {
		t.Errorf("filterDoc() = %s, want the empty document", got)
	}
}

// What a reader could not have meant is refused here rather than sent, exactly as `sqlkit.Where`
// refuses it: a field the collection never held, a token this side does not know, half a BETWEEN.
func TestFilterDocRefusesWhatItCannotSpell(t *testing.T) {
	tests := []struct {
		name  string
		child driver.TableCriterion
	}{
		{"unknown field", driver.TableFilter{Column: "nope", Op: driver.FilterEqual, Values: []string{"1"}}},
		{"unknown operator", driver.TableFilter{Column: "age", Op: "frobnicate", Values: []string{"1"}}},
		{"half a range", driver.TableFilter{Column: "age", Op: driver.FilterBetween, Values: []string{"1"}}},
		{"an empty list", driver.TableFilter{Column: "age", Op: driver.FilterInList}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := filterDoc(driver.TableGroup{Children: []driver.TableCriterion{tt.child}}, sampled)
			if err == nil {
				t.Fatal("filterDoc() = no error, want one")
			}

			var validationErr domain.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %T, want a domain.ValidationError", err)
			}
		})
	}
}

// A conjunction this side does not know is refused rather than defaulted to `$and`: a read narrowed
// the wrong way looks like data.
func TestFilterDocRefusesAnUnknownConjunction(t *testing.T) {
	group := driver.TableGroup{
		Conjunction: "maybe",
		Children: []driver.TableCriterion{
			driver.TableFilter{Column: "age", Op: driver.FilterEqual, Values: []string{"1"}},
			driver.TableFilter{Column: "live", Op: driver.FilterEqual, Values: []string{"true"}},
		},
	}

	if _, err := filterDoc(group, sampled); err == nil {
		t.Fatal("filterDoc() = no error, want one")
	}
}

func TestCoerceReadsTextAsTheTypeTheFieldHolds(t *testing.T) {
	tests := []struct {
		kind  string
		value string
		want  any
	}{
		{"objectId", "507f1f77bcf86cd799439011", objectID(t, "507f1f77bcf86cd799439011")},
		{"int", "42", int32(42)},
		{"long", "9007199254740993", int64(9007199254740993)},
		{"double", "1.5", 1.5},
		{"bool", "true", true},
		{"date", "2024-01-31T09:00:00Z", dateAt(t, "2024-01-31T09:00:00Z")},
		{"string", "42", "42"},

		// A value that will not read as the type the field holds stays the text it is: a filter that
		// silently became a different value would be worse than one that matches nothing.
		{"int", "not a number", "not a number"},
		{"objectId", "nope", "nope"},
		{"", "anything", "anything"},
	}

	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.value, func(t *testing.T) {
			if got := coerce(tt.value, tt.kind); got != tt.want {
				t.Errorf("coerce(%q, %q) = %#v, want %#v", tt.value, tt.kind, got, tt.want)
			}
		})
	}
}

// A nested value is edited in the notation it was drawn in, which is what makes the grid's own
// spelling round-trip.
func TestCoerceReadsADocumentBackOutOfItsOwnNotation(t *testing.T) {
	got := coerce(`{ city: "Hanoi" }`, "object")

	document, ok := got.(bson.D)
	if !ok {
		t.Fatalf("coerce() = %T, want a bson.D", got)
	}
	if text := documentText(document); text != `{ city: "Hanoi" }` {
		t.Errorf("coerce() = %s, want the document as drawn", text)
	}
}

// A read that chose no key falls back to `_id`, which is what makes a page stable: a document store
// returns documents in no order at all otherwise.
func TestSortDocFallsBackToTheKey(t *testing.T) {
	if got := documentText(sortDoc(nil, sampled)); got != `{ _id: 1 }` {
		t.Errorf("sortDoc(nil) = %s, want the key", got)
	}
}

func TestSortDocOrdersByTheKeysTheReaderChose(t *testing.T) {
	sort := []driver.TableSort{
		{Column: "age", Descending: true},
		{Column: "name"},
	}

	if got := documentText(sortDoc(sort, sampled)); got != `{ age: -1, name: 1 }` {
		t.Errorf("sortDoc() = %s, want the keys in the order they were chosen", got)
	}
}

// A name the collection has never held is dropped rather than sent, so no caller can write the sort
// itself.
func TestSortDocDropsAFieldTheCollectionDoesNotHave(t *testing.T) {
	sort := []driver.TableSort{{Column: "nope"}, {Column: "age"}}

	if got := documentText(sortDoc(sort, sampled)); got != `{ age: 1 }` {
		t.Errorf("sortDoc() = %s, want the unknown field dropped", got)
	}
}
