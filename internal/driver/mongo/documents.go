package mongo

import (
	"context"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// idField is the one field every document has and the only key a document can be addressed by.
// MongoDB mints it where a document arrives without one, will not let it be changed, and indexes
// it whether or not anybody asked — which is exactly what a primary key is, so that is what this
// adapter reports it as.
const idField = "_id"

// fieldSet is the columns a read turned out to have, in the order they were first seen. A document
// store declares no columns, so this is what stands in for a catalogue read: the fields of the
// documents themselves, `_id` first because it is the one every document has.
type fieldSet struct {
	order  []string
	byName map[string]driver.Column
}

func newFieldSet() *fieldSet {
	return &fieldSet{byName: map[string]driver.Column{}}
}

// add notes a field. The **first** type seen wins, and a later null never overwrites it: a field
// that is a date in most documents and missing in one is a date, and calling it null because the
// last document read had none would be the wrong answer about the column.
func (f *fieldSet) add(name string, kind bson.Type) {
	held, seen := f.byName[name]
	if !seen {
		f.order = append(f.order, name)
		f.byName[name] = column(name, kind)
		return
	}
	if held.Type == "" || held.Type == "null" {
		f.byName[name] = column(name, kind)
	}
}

// addDocument notes every top-level field of one document. **Only the top level**: a nested
// document is one value in one column, drawn in the shell's own notation, because a column per
// path would give two documents of one collection different columns and a grid cannot draw that.
func (f *fieldSet) addDocument(raw bson.Raw) {
	elements, err := raw.Elements()
	if err != nil {
		return
	}
	for _, element := range elements {
		name, err := element.KeyErr()
		if err != nil {
			continue
		}
		f.add(name, element.Value().Type)
	}
}

func (f *fieldSet) addAll(columns []driver.Column) {
	for _, one := range columns {
		if _, seen := f.byName[one.Name]; !seen {
			f.order = append(f.order, one.Name)
		}
		f.byName[one.Name] = one
	}
}

// list is the fields as columns, `_id` moved to the front where the documents carried one. It does
// not *invent* one — a command's answer document is read by this too, and `{ ok: 1 }` has no key —
// so a collection read adds it for itself; see fields.
func (f *fieldSet) list() []driver.Column {
	names := slices.Clone(f.order)
	if at := slices.Index(names, idField); at > 0 {
		names = slices.Insert(slices.Delete(names, at, at+1), 0, idField)
	}

	columns := make([]driver.Column, 0, len(names))
	for _, name := range names {
		columns = append(columns, f.byName[name])
	}
	return columns
}

// column is one field as the rest of the app reads a column. Everything but `_id` is nullable,
// because in a document store a field is absent as easily as it is null and the two are the same
// question to a filter.
func column(name string, kind bson.Type) driver.Column {
	return driver.Column{
		Name:       name,
		Type:       typeName(kind),
		FullType:   typeName(kind),
		Nullable:   name != idField,
		PrimaryKey: name == idField,
	}
}

// documents is what a read of a cursor produced: the columns the documents turned out to have and
// their values as text.
type documents struct {
	Columns   []driver.Column
	Rows      [][]*string
	Truncated bool
}

// collect reads a cursor into the rows the grid draws.
//
// **A bounded read takes its columns from the documents it actually returned**, merged onto the
// fields already known so the order stays the collection's: nothing a document holds is left out of
// what the reader is shown.
//
// **A streamed read cannot.** Its header crosses the bridge before its first batch does, so its
// columns are the sampled ones and a field outside the sample is not drawn — see fields, and
// docs/grid.md. Everything else about the two is the same, including that whatever was handed over
// is always every row that was counted.
func collect(
	ctx context.Context,
	cursor *mgo.Cursor,
	known []driver.Column,
	maxRows int,
	stream *driver.RowStream,
) (documents, error) {
	defer cursor.Close(ctx)

	fields := newFieldSet()
	fields.addAll(known)

	// A streamed read's columns are settled before the first row goes out; a bounded one's grow
	// with the documents, which is why it keeps them until the end.
	fixed := stream != nil
	columns := fields.list()

	held := []bson.Raw{}
	out := [][]*string{}
	fetched := 0
	counted := time.Now()
	truncated := false

	hand := func() {
		if stream != nil && len(out) > 0 {
			stream.Rows(out)
			out = [][]*string{}
		}
	}
	count := func() {
		if stream != nil {
			stream.Counted(fetched)
			counted = time.Now()
		}
	}

	for cursor.Next(ctx) {
		if maxRows > 0 && fetched == maxRows {
			truncated = true
			break
		}

		// The cursor reuses its buffer between iterations, so a document kept past this line has
		// to be a copy of one rather than a view onto the next.
		raw := bson.Raw(slices.Clone([]byte(cursor.Current)))
		fetched++

		if !fixed {
			fields.addDocument(raw)
			held = append(held, raw)
			continue
		}

		out = append(out, row(raw, columns))
		if len(out) == sqlkit.BatchRows {
			hand()
		}
		if time.Since(counted) >= sqlkit.CountEvery {
			count()
		}
	}

	if err := cursor.Err(); err != nil {
		hand()
		count()
		return documents{}, translateError(err)
	}

	if fixed {
		hand()
		count()
		return documents{Columns: columns, Rows: out, Truncated: truncated}, nil
	}

	columns = fields.list()
	rows := make([][]*string, 0, len(held))
	for _, raw := range held {
		rows = append(rows, row(raw, columns))
	}
	return documents{Columns: columns, Rows: rows, Truncated: truncated}, nil
}

// row is one document against one column list. A field the document does not carry is nil, which
// is the same thing the grid draws for one carrying null — in a document store they are one answer.
func row(raw bson.Raw, columns []driver.Column) []*string {
	values := make([]*string, len(columns))
	for i, one := range columns {
		values[i] = cell(raw.Lookup(one.Name))
	}
	return values
}
