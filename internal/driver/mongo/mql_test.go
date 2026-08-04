package mongo

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestParseReadsACollectionCommand(t *testing.T) {
	one, err := parse(`db.users.find({ age: { $gt: 30 } })`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if one.Collection != "users" {
		t.Errorf("Collection = %q, want %q", one.Collection, "users")
	}
	if one.Method != "find" {
		t.Errorf("Method = %q, want %q", one.Method, "find")
	}
	if got := documentText(one.Filter()); got != `{ age: { $gt: 30 } }` {
		t.Errorf("Filter() = %s, want the document as written", got)
	}
}

// A command with no collection is addressed to the database, which is the one shape whose first
// word after `db.` is the method rather than a name.
func TestParseReadsADatabaseCommand(t *testing.T) {
	one, err := parse(`db.runCommand({ ping: 1 })`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if one.Collection != "" {
		t.Errorf("Collection = %q, want none", one.Collection)
	}
	if one.Method != "runCommand" {
		t.Errorf("Method = %q, want %q", one.Method, "runCommand")
	}
}

func TestParseReadsGetCollectionForANameThatIsNotABareWord(t *testing.T) {
	one, err := parse(`db.getCollection("audit-log").countDocuments({})`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if one.Collection != "audit-log" {
		t.Errorf("Collection = %q, want %q", one.Collection, "audit-log")
	}
	if one.Method != "countDocuments" {
		t.Errorf("Method = %q, want %q", one.Method, "countDocuments")
	}
}

func TestParseReadsTheCursorMethodsChainedOntoARead(t *testing.T) {
	one, err := parse(`db.users.find({}).sort({ name: -1 }).skip(20).limit(10)`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	read, err := bounds(one)
	if err != nil {
		t.Fatalf("bounds() error = %v", err)
	}

	if got := documentText(read.Sort); got != `{ name: -1 }` {
		t.Errorf("Sort = %s, want the keys as written", got)
	}
	if read.Skip != 20 {
		t.Errorf("Skip = %d, want 20", read.Skip)
	}
	if read.Limit != 10 {
		t.Errorf("Limit = %d, want 10", read.Limit)
	}
}

// The values that are not JSON are the whole reason this is a parser and not `json.Unmarshal`: a
// filter that spelled an ObjectId as a string would match nothing and say nothing about why.
func TestParseReadsTheShellsOwnConstructors(t *testing.T) {
	tests := []struct {
		name string
		mql  string
		want any
	}{
		{"objectId", `ObjectId("507f1f77bcf86cd799439011")`, objectID(t, "507f1f77bcf86cd799439011")},
		{"date", `ISODate("2024-01-31T09:00:00Z")`, dateAt(t, "2024-01-31T09:00:00Z")},
		{"bare date", `Date("2024-01-31")`, dateAt(t, "2024-01-31T00:00:00Z")},
		{"new date", `new ISODate("2024-01-31T09:00:00Z")`, dateAt(t, "2024-01-31T09:00:00Z")},
		{"long", `NumberLong("9007199254740993")`, int64(9007199254740993)},
		{"int", `NumberInt(7)`, int32(7)},
		{"min key", `MinKey()`, bson.MinKey{}},
		{"regexp", `RegExp("^a", "i")`, bson.Regex{Pattern: "^a", Options: "i"}},
		{"regex literal", `/^a.*b$/i`, bson.Regex{Pattern: "^a.*b$", Options: "i"}},
		{"true", `true`, true},
		{"null", `null`, nil},
		{"negative", `-42`, int32(-42)},
		{"double", `1.5`, 1.5},
		{"exponent", `1e-9`, 1e-9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			one, err := parse(`db.users.find({ f: ` + tt.mql + ` })`)
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			filter := one.Filter()
			if len(filter) != 1 {
				t.Fatalf("Filter() = %v, want one field", filter)
			}
			if got := filter[0].Value; got != tt.want {
				t.Errorf("value = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// A dotted path is one key however it is written, which is what makes `{ address.city: … }` reach
// the server as the field it names rather than as a nested document.
func TestParseReadsADottedPathAsOneKey(t *testing.T) {
	one, err := parse(`db.users.find({ address.city: "Hanoi" })`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if got := one.Filter()[0].Key; got != "address.city" {
		t.Errorf("key = %q, want %q", got, "address.city")
	}
}

func TestParseKeepsTheOrderOfADocumentsFields(t *testing.T) {
	one, err := parse(`db.users.find({ b: 1, a: 2, c: 3 })`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	want := []string{"b", "a", "c"}
	for i, key := range want {
		if got := one.Filter()[i].Key; got != key {
			t.Errorf("field %d = %q, want %q", i, got, key)
		}
	}
}

func TestParseReadsNestedDocumentsAndArrays(t *testing.T) {
	one, err := parse(`db.users.aggregate([{ $match: { tags: { $in: ["a", "b"] } } }, { $limit: 5 }])`)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	pipeline, err := stages(one.Arg(0))
	if err != nil {
		t.Fatalf("stages() error = %v", err)
	}
	if len(pipeline) != 2 {
		t.Fatalf("stages() = %d, want 2", len(pipeline))
	}
	if got := documentText(pipeline[0]); got != `{ $match: { tags: { $in: ["a", "b"] } } }` {
		t.Errorf("stage = %s, want the document as written", got)
	}
}

// What the reader typed wrong is reported as the engine's own rejection would be, and it says where
// it stopped: a message about a command says far less than one about the character.
func TestParseRefusesWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name string
		mql  string
	}{
		{"no db", `users.find({})`},
		{"no call", `db.users.find`},
		{"unclosed brace", `db.users.find({ a: 1`},
		{"unknown value", `db.users.find({ a: nope })`},
		{"bad object id", `db.users.find({ _id: ObjectId("nope") })`},
		{"trailing rubbish", `db.users.find({}) extra`},
		{"missing colon", `db.users.find({ a 1 })`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.mql)
			if err == nil {
				t.Fatalf("parse(%q) = no error, want one", tt.mql)
			}

			var statementErr domain.StatementError
			if !errors.As(err, &statementErr) {
				t.Fatalf("error = %T, want a domain.StatementError", err)
			}
			if statementErr.Message == "" {
				t.Error("message is empty, and the wording is the only useful thing to show")
			}
		})
	}
}

// A chunk holding nothing but a comment is not a command, and a note written above one does not stop
// the command under it from running.
func TestSplitDropsAChunkThatIsOnlyAComment(t *testing.T) {
	got := text{}.Split("// a note\ndb.users.find({});\n/* another */\n")

	if len(got) != 1 {
		t.Fatalf("Split() = %d commands (%q), want 1", len(got), got)
	}
	if _, err := parse(got[0]); err != nil {
		t.Errorf("parse(%q) error = %v, want the note stepped over", got[0], err)
	}
}

// A comment is never part of a command, wherever in one it is written.
func TestParseStepsOverACommentInsideACommand(t *testing.T) {
	one, err := parse("db.users /* the live ones */ .find({ live: true })")
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if one.Collection != "users" || one.Method != "find" {
		t.Errorf("parse() = %q.%q, want users.find", one.Collection, one.Method)
	}
}

// A semicolon inside a string ends nothing, which is the whole reason splitting is lexical.
func TestSplitIgnoresASemicolonInAString(t *testing.T) {
	got := text{}.Split(`db.users.find({ a: "one;two" }); db.users.countDocuments({})`)

	if len(got) != 2 {
		t.Fatalf("Split() = %d commands (%q), want 2", len(got), got)
	}
}

func TestParseValueReadsOneCellsWorth(t *testing.T) {
	value, err := parseValue(`{ a: 1 }`)
	if err != nil {
		t.Fatalf("parseValue() error = %v", err)
	}
	if got := documentText(value); got != `{ a: 1 }` {
		t.Errorf("parseValue() = %s, want the document as written", got)
	}
}

func objectID(t *testing.T, hex string) bson.ObjectID {
	t.Helper()
	id, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		t.Fatalf("ObjectIDFromHex(%q) error = %v", hex, err)
	}
	return id
}

func dateAt(t *testing.T, instant string) bson.DateTime {
	t.Helper()
	when, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", instant, err)
	}
	return bson.NewDateTimeFromTime(when)
}
