package compare

import (
	"reflect"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func column(name, fullType string, options ...func(*driver.Column)) driver.Column {
	one := driver.Column{Name: name, Type: fullType, FullType: fullType, Nullable: true}
	for _, option := range options {
		option(&one)
	}
	return one
}

func notNull(one *driver.Column) { one.Nullable = false }
func primary(one *driver.Column) { one.PrimaryKey = true; one.Nullable = false }
func defaults(text string) func(*driver.Column) {
	return func(one *driver.Column) { one.Default = &text }
}

func text(value string) *string { return &value }

func TestPairNamesKeepsTheLeftsOrderAndPlacesNewNamesWhereTheRightHasThem(t *testing.T) {
	pairs := pairNames([]string{"id", "name", "gone"}, []string{"id", "added", "name"})

	want := []NamePair{{0, 0}, {-1, 1}, {1, 2}, {2, -1}}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("pairNames() = %v, want %v", pairs, want)
	}
}

func TestCompareStructureNamesWhatDiffersAboutAField(t *testing.T) {
	left := driver.TableStructure{Columns: []driver.Column{
		column("id", "int", primary),
		column("name", "varchar(50)"),
		column("dropped", "text"),
	}}
	right := driver.TableStructure{Columns: []driver.Column{
		column("id", "int", primary),
		column("added", "text"),
		column("name", "varchar(100)", notNull),
	}}

	fields := CompareStructure(left, right).Fields
	if len(fields) != 4 {
		t.Fatalf("Fields = %d pairs, want 4", len(fields))
	}

	if fields[0].Status != Same {
		t.Errorf("id status = %q, want %q", fields[0].Status, Same)
	}
	if fields[1].Name != "added" || fields[1].Status != Added {
		t.Errorf("second pair = %q/%q, want added/%q", fields[1].Name, fields[1].Status, Added)
	}
	if fields[2].Status != Changed {
		t.Errorf("name status = %q, want %q", fields[2].Status, Changed)
	}
	if want := []string{diffType, diffNullable}; !reflect.DeepEqual(fields[2].Differs, want) {
		t.Errorf("name differs = %v, want %v", fields[2].Differs, want)
	}
	if fields[3].Name != "dropped" || fields[3].Status != Removed {
		t.Errorf("last pair = %q/%q, want dropped/%q", fields[3].Name, fields[3].Status, Removed)
	}
}

// Two engines spell one type two ways and neither is wrong, so case alone is not a difference —
// but a length is, `varchar(50)` being a different column from `varchar(100)`.
func TestCompareStructureFoldsCaseButNotLength(t *testing.T) {
	left := driver.TableStructure{Columns: []driver.Column{column("name", "VARCHAR(50)")}}
	right := driver.TableStructure{Columns: []driver.Column{column("name", "varchar(50)")}}

	if got := CompareStructure(left, right).Fields[0].Status; got != Same {
		t.Errorf("status = %q, want %q", got, Same)
	}
}

// A column with no default and one defaulting to the empty string are not the same column.
func TestCompareStructureTellsNoDefaultFromAnEmptyOne(t *testing.T) {
	left := driver.TableStructure{Columns: []driver.Column{column("name", "text")}}
	right := driver.TableStructure{Columns: []driver.Column{column("name", "text", defaults(""))}}

	pair := CompareStructure(left, right).Fields[0]
	if pair.Status != Changed || !reflect.DeepEqual(pair.Differs, []string{diffDefault}) {
		t.Errorf("pair = %q %v, want changed [default]", pair.Status, pair.Differs)
	}
}

func TestCompareStructureReadsAnIndexsColumnsInOrder(t *testing.T) {
	left := driver.TableStructure{Indexes: []driver.Index{
		{Name: "by_pair", Columns: []string{"a", "b"}, Method: "BTREE"},
	}}
	right := driver.TableStructure{Indexes: []driver.Index{
		{Name: "by_pair", Columns: []string{"b", "a"}, Method: "btree"},
	}}

	pair := CompareStructure(left, right).Indexes[0]
	if pair.Status != Changed || !reflect.DeepEqual(pair.Differs, []string{diffFields}) {
		t.Errorf("pair = %q %v, want changed [fields]", pair.Status, pair.Differs)
	}
}

func TestKeyForPrefersTheDeclaredPrimaryKey(t *testing.T) {
	shape := driver.TableStructure{Columns: []driver.Column{
		column("id", "int", primary),
		column("name", "text"),
	}}

	key := KeyFor(shape, shape)
	if !key.Declared || !reflect.DeepEqual(key.Columns, []string{"id"}) {
		t.Fatalf("KeyFor() = %v, want declared [id]", key)
	}
}

// With no key of their own, two tables are matched on the whole of a row — and the fallback says
// so, because nothing can then be reported as changed.
func TestKeyForFallsBackToEverySharedColumn(t *testing.T) {
	left := driver.TableStructure{Columns: []driver.Column{column("a", "int"), column("b", "int")}}
	right := driver.TableStructure{Columns: []driver.Column{column("a", "int"), column("c", "int")}}

	key := KeyFor(left, right)
	if key.Declared || !reflect.DeepEqual(key.Columns, []string{"a"}) {
		t.Fatalf("KeyFor() = %v, want undeclared [a]", key)
	}
}

func TestCompareRowsMatchesOnTheKeyAndMarksTheCellsThatDiffer(t *testing.T) {
	shape := []driver.Column{column("id", "int", primary), column("name", "text")}

	left := Table{Columns: shape, Rows: [][]*string{
		{text("1"), text("ann")},
		{text("2"), text("bob")},
		{text("3"), text("cat")},
	}}
	right := Table{Columns: shape, Rows: [][]*string{
		{text("1"), text("ann")},
		{text("2"), text("BOB")},
		{text("4"), text("dan")},
	}}

	data := CompareRows(left, right, Key{Columns: []string{"id"}, Declared: true})

	statuses := make([]Status, 0, len(data.Rows))
	for _, row := range data.Rows {
		statuses = append(statuses, row.Status)
	}
	want := []Status{Same, Changed, Removed, Added}
	if !reflect.DeepEqual(statuses, want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}

	if !reflect.DeepEqual(data.Rows[1].Differs, []bool{false, true}) {
		t.Errorf("changed row differs = %v, want [false true]", data.Rows[1].Differs)
	}
	if !reflect.DeepEqual(data.Rows[2].Key, []string{"3"}) {
		t.Errorf("removed row key = %v, want [3]", data.Rows[2].Key)
	}
}

// Only the columns both sides hold are compared; the rest are named so the reader is told they
// were left out rather than shown as differences they are not.
func TestCompareRowsComparesTheSharedColumnsAndNamesTheRest(t *testing.T) {
	left := Table{
		Columns: []driver.Column{column("id", "int", primary), column("gone", "text")},
		Rows:    [][]*string{{text("1"), text("x")}},
	}
	right := Table{
		Columns: []driver.Column{column("extra", "text"), column("id", "int", primary)},
		Rows:    [][]*string{{text("y"), text("1")}},
	}

	data := CompareRows(left, right, Key{Columns: []string{"id"}, Declared: true})

	if !reflect.DeepEqual(data.Columns, []string{"id"}) {
		t.Errorf("Columns = %v, want [id]", data.Columns)
	}
	if !reflect.DeepEqual(data.LeftOnly, []string{"gone"}) {
		t.Errorf("LeftOnly = %v, want [gone]", data.LeftOnly)
	}
	if !reflect.DeepEqual(data.RightOnly, []string{"extra"}) {
		t.Errorf("RightOnly = %v, want [extra]", data.RightOnly)
	}
	if data.Rows[0].Status != Same {
		t.Errorf("status = %q, want %q", data.Rows[0].Status, Same)
	}
}

// A NULL is not the empty string, here as everywhere else in the app.
func TestCompareRowsTellsNullFromTheEmptyString(t *testing.T) {
	shape := []driver.Column{column("id", "int", primary), column("note", "text")}

	left := Table{Columns: shape, Rows: [][]*string{{text("1"), nil}}}
	right := Table{Columns: shape, Rows: [][]*string{{text("1"), text("")}}}

	data := CompareRows(left, right, Key{Columns: []string{"id"}, Declared: true})
	if data.Rows[0].Status != Changed {
		t.Errorf("status = %q, want %q", data.Rows[0].Status, Changed)
	}
}

// A key held twice pairs first-to-first rather than one row standing in for both, which is what a
// table with no key of its own is compared as.
func TestCompareRowsPairsDuplicateKeysOneForOne(t *testing.T) {
	shape := []driver.Column{column("name", "text")}

	left := Table{Columns: shape, Rows: [][]*string{{text("a")}, {text("a")}, {text("a")}}}
	right := Table{Columns: shape, Rows: [][]*string{{text("a")}, {text("a")}}}

	data := CompareRows(left, right, Key{Columns: []string{"name"}})

	statuses := make([]Status, 0, len(data.Rows))
	for _, row := range data.Rows {
		statuses = append(statuses, row.Status)
	}
	if want := []Status{Same, Same, Removed}; !reflect.DeepEqual(statuses, want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
}

// A key of two columns where one is NULL is not the key where the other is, and neither can be
// read as any other pair of values — see encodeKey.
func TestEncodeKeyCannotConfuseTwoDifferentKeys(t *testing.T) {
	cases := [][]*string{
		{text("a"), text("bc")},
		{text("ab"), text("c")},
		{nil, text("abc")},
		{text(""), text("abc")},
	}

	seen := make(map[string]bool, len(cases))
	for _, values := range cases {
		encoded := encodeKey(values)
		if seen[encoded] {
			t.Fatalf("encodeKey(%v) collided with an earlier key", values)
		}
		seen[encoded] = true
	}
}
