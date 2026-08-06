package mongo

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func value(text string) *string { return &text }

func written(mode driver.RowWriteMode, batch int, rows [][]*string) []string {
	return (&session{}).RowStatements(driver.Namespace{Database: "shop"}, "people", driver.RowWrite{
		Mode:    mode,
		Columns: []string{"_id", "name"},
		Keys:    []string{"_id"},
		Rows:    rows,
		Batch:   batch,
	})
}

var people = [][]*string{
	{value("1"), value("Ada")},
	{value("2"), value("Grace")},
}

// Every command this builds has to be one the app's own reader accepts, or an import would produce
// a file of commands the client refuses before the server ever sees them. `parse` is the same
// reader `Query` runs a typed command through.
func readsBack(t *testing.T, statements []string) []statement {
	t.Helper()

	read := make([]statement, 0, len(statements))
	for _, one := range statements {
		parsed, err := parse(one)
		if err != nil {
			t.Fatalf("this app cannot read back what it wrote:\n  %s\n  -> %v", one, err)
		}
		read = append(read, parsed)
	}
	return read
}

// A batch of one is what a dump and an export are, and it has to stay the command it has always
// been: a file of `insertMany([…])` a thousand documents wide is not one anybody reads back.
func TestAppendWithABatchOfOneStaysInsertOne(t *testing.T) {
	got := written(driver.WriteAppend, 1, people)

	if len(got) != 2 {
		t.Fatalf("got %d commands, want 2", len(got))
	}
	// `1` and not `"1"`: a cell's text is read back into the value it was drawn from — see
	// `shellValue` — which is what makes an export and an import of the same collection agree.
	want := `db.people.insertOne({ _id: 1, name: "Ada" })`
	if got[0] != want {
		t.Errorf("command = %s, want %s", got[0], want)
	}
	readsBack(t, got)
}

func TestAppendBatchesIntoInsertMany(t *testing.T) {
	got := written(driver.WriteAppend, 10, people)

	if len(got) != 1 || !strings.HasPrefix(got[0], "db.people.insertMany([") {
		t.Fatalf("command = %v", got)
	}
	if read := readsBack(t, got); read[0].Method != "insertMany" {
		t.Errorf("method = %s, want insertMany", read[0].Method)
	}
}

// The three keyed modes are three shapes of one command, and the whole of what tells them apart is
// which update operator and whether the document may be made.
func TestKeyedModesAreSpelledAsTheShellWritesThem(t *testing.T) {
	cases := []struct {
		mode     driver.RowWriteMode
		contains []string
	}{
		{driver.WriteUpdate, []string{"updateMany(", "{ _id: 1 }", "$set", `name: "Ada"`}},
		{driver.WriteUpsert, []string{"updateOne(", "$set", "upsert: true"}},
		{driver.WriteSkip, []string{"updateOne(", "$setOnInsert", "upsert: true"}},
		{driver.WriteDelete, []string{"deleteMany(", "{ _id: 1 }"}},
	}

	for _, one := range cases {
		got := written(one.mode, 10, people[:1])
		if len(got) != 1 {
			t.Fatalf("%s produced %d commands, want 1", one.mode, len(got))
		}
		for _, part := range one.contains {
			if !strings.Contains(got[0], part) {
				t.Errorf("%s command %s does not hold %s", one.mode, got[0], part)
			}
		}
		readsBack(t, got)
	}
}

// The key is what the document was found by, so setting it again says nothing — and on an upsert
// the server writes it from the filter itself.
func TestTheKeyIsNotAlsoSet(t *testing.T) {
	got := written(driver.WriteUpdate, 10, people[:1])

	if strings.Contains(got[0], `$set: { _id`) {
		t.Errorf("the key was set as well as matched: %s", got[0])
	}
}

// The command that must never be written. `deleteMany({})` empties a collection, and a mapping with
// no key in it must not be the thing that runs it — so a row with nothing to find it by matches
// nothing, and the filter says so in a way the reader's own hazard rule also reads as unfiltered.
func TestARowWithNoKeyMatchesNothing(t *testing.T) {
	got := (&session{}).RowStatements(driver.Namespace{}, "people", driver.RowWrite{
		Mode:    driver.WriteDelete,
		Columns: []string{"name"},
		Keys:    []string{"_id"},
		Rows:    [][]*string{{value("Ada")}},
		Batch:   1,
	})

	if len(got) != 1 || strings.Contains(got[0], "deleteMany({})") {
		t.Fatalf("command = %v", got)
	}
	if !strings.Contains(got[0], "$expr: false") {
		t.Errorf("a keyless row did not match nothing: %s", got[0])
	}
	if domain.ImportHazard(domain.ImportDelete, false) != domain.HazardUnfiltered {
		t.Error("a keyless delete is not being answered for as unfiltered")
	}
}

// A NULL is left out of a document being written — a field nobody wrote is not a field holding
// nothing — and is the `null` a document may genuinely hold when it is being matched on.
func TestNullIsAbsentInADocumentAndPresentInAFilter(t *testing.T) {
	rows := [][]*string{{nil, value("Ada")}}

	added := written(driver.WriteAppend, 1, rows)
	if strings.Contains(added[0], "_id") {
		t.Errorf("a NULL was written as a field: %s", added[0])
	}

	removed := written(driver.WriteDelete, 1, rows)
	if !strings.Contains(removed[0], "_id: null") {
		t.Errorf("a NULL key was not matched as null: %s", removed[0])
	}
}
