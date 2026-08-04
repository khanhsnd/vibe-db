package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// cell is what the grid draws. A scalar reads bare, because that is what a reader is looking at a
// column of; a document or an array reads in the shell's own notation, because that is a notation
// they can paste back into the editor.
func TestCellDrawsAValueAsTheGridShowsIt(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  *string
	}{
		{"text", "Hanoi", ptr("Hanoi")},
		{"an int", int32(42), ptr("42")},
		{"a long", int64(9007199254740993), ptr("9007199254740993")},
		{"a double", 1.5, ptr("1.5")},
		{"true", true, ptr("true")},
		{"an object id", objectID(t, "507f1f77bcf86cd799439011"), ptr("ObjectId(\"507f1f77bcf86cd799439011\")")},
		{"a date", dateAt(t, "2024-01-31T09:00:00Z"), ptr(`ISODate("2024-01-31T09:00:00Z")`)},
		{"a regex", bson.Regex{Pattern: "^a", Options: "i"}, ptr("/^a/i")},
		{"a document", bson.D{{Key: "city", Value: "Hanoi"}}, ptr(`{ city: "Hanoi" }`)},
		{"an array", bson.A{int32(1), "two"}, ptr(`[1, "two"]`)},
		{"an empty document", bson.D{}, ptr("{}")},
		{"an empty array", bson.A{}, ptr("[]")},

		// The absence of a value and an explicit null read the same, because to a filter they are
		// one question.
		{"null", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cell(wrapped(t, tt.value))

			switch {
			case got == nil && tt.want == nil:
			case got == nil || tt.want == nil:
				t.Fatalf("cell() = %v, want %v", show(got), show(tt.want))
			case *got != *tt.want:
				t.Errorf("cell() = %q, want %q", *got, *tt.want)
			}
		})
	}
}

// A field the document does not carry is the same nothing an explicit null is.
func TestCellDrawsAnAbsentFieldAsNothing(t *testing.T) {
	if got := cell(bson.RawValue{}); got != nil {
		t.Errorf("cell() = %q, want nothing", *got)
	}
}

// Bytes that are not text cannot survive the trip to the WebView as a JSON string, so they are
// written as hex — the same spelling `sqlkit` gives a `bytea`, and the one the value viewer decodes.
func TestCellDrawsBinaryAsHex(t *testing.T) {
	got := cell(wrapped(t, bson.Binary{Subtype: 0, Data: []byte{0x89, 0x50, 0x4e, 0x47}}))

	if got == nil {
		t.Fatal("cell() = nothing, want hex")
	}
	if *got != sqlkit.BinaryPrefix+"89504e47" {
		t.Errorf("cell() = %q, want %q", *got, sqlkit.BinaryPrefix+"89504e47")
	}
}

// The grid's own notation is the parser's own notation, which is what makes an edit round-trip: a
// value drawn as `ObjectId("…")` goes back as an ObjectId and not as the text of one.
func TestTheGridsNotationReadsBackIntoTheSameValue(t *testing.T) {
	values := []any{
		objectID(t, "507f1f77bcf86cd799439011"),
		dateAt(t, "2024-01-31T09:00:00Z"),
		bson.Regex{Pattern: "^a", Options: "i"},
		bson.D{{Key: "city", Value: "Hanoi"}, {Key: "zip", Value: int32(10000)}},
		bson.A{int32(1), "two", true},
		int32(42),
		int64(9007199254740993),
		1.5,
		true,
	}

	for _, value := range values {
		drawn := cell(wrapped(t, value))
		if drawn == nil {
			t.Fatalf("cell(%#v) = nothing", value)
		}

		read, err := parseValue(*drawn)
		if err != nil {
			t.Fatalf("parseValue(%q) error = %v", *drawn, err)
		}
		if got, want := valueText(read), valueText(value); got != want {
			t.Errorf("%q read back as %s, want %s", *drawn, got, want)
		}
	}
}

// A field's name is written the way the shell writes one: bare where it can be, and quoted where a
// dot or a space would otherwise make it two names or none.
func TestKeyWritesAFieldNameTheWayTheShellDoes(t *testing.T) {
	tests := []struct{ name, want string }{
		{"name", "name"},
		{"_id", "_id"},
		{"$gt", "$gt"},
		{"a1", "a1"},
		{"address.city", `"address.city"`},
		{"a name", `"a name"`},
		{"1st", `"1st"`},
		{"", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := key(tt.name); got != tt.want {
				t.Errorf("key(%q) = %s, want %s", tt.name, got, tt.want)
			}
		})
	}
}

// A field set is what stands in for a catalogue read, and the order it answers in is the order the
// documents were read in — with `_id` first, because that is the column a reader looks for.
func TestFieldSetKeepsTheOrderTheDocumentsWereReadIn(t *testing.T) {
	found := newFieldSet()
	found.addDocument(rawOf(t, bson.D{
		{Key: "name", Value: "a"},
		{Key: "_id", Value: int32(1)},
		{Key: "age", Value: int32(2)},
	}))
	found.addDocument(rawOf(t, bson.D{{Key: "tier", Value: "gold"}}))

	want := []string{"_id", "name", "age", "tier"}
	got := found.list()
	if len(got) != len(want) {
		t.Fatalf("list() = %d fields, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("field %d = %q, want %q", i, got[i].Name, name)
		}
	}
	if !got[0].PrimaryKey {
		t.Error("_id is not reported as the key, and it is the only one a document has")
	}
}

// The first type seen wins. A field that is a date in most documents and missing in one is a date,
// and calling it null because the last document read had none would be the wrong answer.
func TestFieldSetKeepsTheFirstTypeItSaw(t *testing.T) {
	found := newFieldSet()
	found.addDocument(rawOf(t, bson.D{{Key: "joined", Value: dateAt(t, "2024-01-31T09:00:00Z")}}))
	found.addDocument(rawOf(t, bson.D{{Key: "joined", Value: nil}}))

	if got := found.list()[0].Type; got != "date" {
		t.Errorf("type = %q, want %q", got, "date")
	}
}

// A null seen first is replaced by the first real type, so a collection whose first document left a
// field empty still reports what that field is.
func TestFieldSetTakesARealTypeOverANull(t *testing.T) {
	found := newFieldSet()
	found.addDocument(rawOf(t, bson.D{{Key: "joined", Value: nil}}))
	found.addDocument(rawOf(t, bson.D{{Key: "joined", Value: dateAt(t, "2024-01-31T09:00:00Z")}}))

	if got := found.list()[0].Type; got != "date" {
		t.Errorf("type = %q, want %q", got, "date")
	}
}

// A command's answer is read by the same field set, and `{ ok: 1 }` has no key — so nothing invents
// one for it.
func TestFieldSetDoesNotInventAKey(t *testing.T) {
	found := newFieldSet()
	found.addDocument(rawOf(t, bson.D{{Key: "ok", Value: int32(1)}}))

	got := found.list()
	if len(got) != 1 || got[0].Name != "ok" {
		t.Fatalf("list() = %v, want the one field the document had", got)
	}
}

// A document missing a column's field is nil in that column, which is exactly what the grid draws
// for a null.
func TestRowLeavesAMissingFieldEmpty(t *testing.T) {
	columns := []driver.Column{{Name: "_id"}, {Name: "name"}, {Name: "age"}}
	got := row(rawOf(t, bson.D{{Key: "_id", Value: int32(1)}, {Key: "age", Value: int32(30)}}), columns)

	if len(got) != 3 {
		t.Fatalf("row() = %d values, want 3", len(got))
	}
	if got[1] != nil {
		t.Errorf("row()[1] = %q, want nothing", *got[1])
	}
	if got[2] == nil || *got[2] != "30" {
		t.Errorf("row()[2] = %v, want 30", show(got[2]))
	}
}

func TestSizeTextSpellsAByteCountTheWayAReaderReadsOne(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := sizeText(tt.bytes); got != tt.want {
				t.Errorf("sizeText(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// wrapped is one value as it arrives off the wire: BSON has no top level but a document, so a value
// on its own has to be read out of one.
func wrapped(t *testing.T, value any) bson.RawValue {
	t.Helper()
	return rawOf(t, bson.D{{Key: "v", Value: value}}).Lookup("v")
}

func rawOf(t *testing.T, document bson.D) bson.Raw {
	t.Helper()
	raw, err := bson.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal(%v) error = %v", document, err)
	}
	return raw
}

func ptr(text string) *string {
	return &text
}

func show(value *string) string {
	if value == nil {
		return "<nothing>"
	}
	return *value
}
