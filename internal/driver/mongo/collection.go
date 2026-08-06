package mongo

import (
	"context"
	"sort"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	// sampleDocs is how many documents `fields` reads to find out what a collection holds. A
	// document store keeps no column list, so this is the whole of the catalogue: it is a *sample*
	// and is honest about being one — a field that appears in none of the first few hundred
	// documents is a field the tree does not draw until a read that returns it does.
	//
	// The first N in natural order rather than a random N: `$sample` would give a different answer
	// every time the same table was opened, and a column list that moved between refreshes is worse
	// than one that is merely incomplete.
	sampleDocs = 200

	// completionDocs is the same read for the editor, which needs every collection of a database at
	// once and only wants the names. Fewer documents each, because there are far more of these.
	completionDocs = 25
)

func (s *session) TableRows(ctx context.Context, page driver.TablePage) (driver.TableRows, error) {
	known, err := s.fields(ctx, page.Namespace, page.Table)
	if err != nil {
		return driver.TableRows{}, err
	}

	filter, err := filterDoc(page.Filter, known)
	if err != nil {
		return driver.TableRows{}, err
	}
	sort := sortDoc(page.Sort, known)

	collection := s.collection(page.Namespace, page.Table)

	// The total is counted through the same filter the documents are read through, or the pager
	// would walk pages of a collection the reader has narrowed away.
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return driver.TableRows{}, translateError(err)
	}

	opts := options.Find().SetSort(sort)
	if page.Limit > 0 {
		opts.SetLimit(int64(page.Limit)).SetSkip(int64(page.Offset))
	}

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return driver.TableRows{}, translateError(err)
	}

	// The find itself bounds the read, so no cap is applied here — the same arrangement both SQL
	// adapters have, where the statement's own LIMIT is the bound.
	read, err := collect(ctx, cursor, known, 0, page.Stream)
	if err != nil {
		return driver.TableRows{}, err
	}

	return driver.TableRows{
		Columns:   read.Columns,
		Rows:      read.Rows,
		Total:     total,
		Statement: findText(page, filter, sort),
	}, nil
}

// findText spells the read that produced the page, exactly as it ran, so the status line under the
// grid can show it and a reader can paste it into the editor and get the same documents back. It is
// built from the *same documents* that were sent, which is the only arrangement in which what ran
// and what is shown cannot drift apart.
func findText(page driver.TablePage, filter, sort bson.D) string {
	statement := "db." + key(page.Table) + ".find(" + documentText(filter) + ")" +
		".sort(" + documentText(sort) + ")"

	if page.Limit <= 0 {
		return statement
	}
	return statement +
		".skip(" + strconv.Itoa(page.Offset) + ")" +
		".limit(" + strconv.Itoa(page.Limit) + ")"
}

// documentText renders a document the way the shell writes one, by marshalling it and reading it
// back: one renderer serves both a value that came off the wire and one this side built, so a filter
// shown to the reader is spelled exactly as a document read out of a collection is.
func documentText(value any) string {
	raw, err := bson.Marshal(value)
	if err != nil {
		return "{}"
	}
	return Document(raw)
}

// ColumnValues lists the values one field actually holds, for a reader picking a filter's value
// rather than typing it. It is an aggregation and not `distinct` because the cap has to be the
// server's: `distinct` answers with every value there is and would hand back a megabyte to fill a
// list of two hundred.
func (s *session) ColumnValues(ctx context.Context, pick driver.ColumnPick) ([]string, error) {
	known, err := s.fields(ctx, pick.Namespace, pick.Table)
	if err != nil {
		return nil, err
	}
	if _, ok := field(known, pick.Column); !ok {
		return nil, filterError("names no field of this collection")
	}

	// Grouping by the field is what deduplicates it; the `$match` after it drops the absence of a
	// value, which is not one — a reader reaches that with the `is null` operator.
	pipeline := []bson.D{
		{{Key: "$group", Value: bson.D{{Key: idField, Value: "$" + pick.Column}}}},
		{{Key: "$match", Value: bson.D{{Key: idField, Value: bson.D{{Key: "$ne", Value: nil}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: idField, Value: 1}}}},
		{{Key: "$limit", Value: pick.Limit}},
	}

	cursor, err := s.collection(pick.Namespace, pick.Table).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, translateError(err)
	}

	read, err := collect(ctx, cursor, []driver.Column{{Name: idField}}, 0, nil)
	if err != nil {
		return nil, err
	}
	return sqlkit.FirstColumn(read.Rows), nil
}

// MatchingRows counts the documents of one collection that hold the reader's text — the data half
// of a Find in Schema. **The fields it looks in are the sampled ones**, which is the only field
// list a collection has: a field appearing in none of the first `sampleDocs` documents is not
// searched, exactly as it is not drawn — see fields and docs/grid.md.
func (s *session) MatchingRows(
	ctx context.Context,
	search driver.RowSearch,
) (driver.RowMatches, error) {
	known, err := s.fields(ctx, search.Namespace, search.Table)
	if err != nil {
		return driver.RowMatches{}, err
	}

	filter, err := matchDoc(search.Match, known)
	if err != nil {
		return driver.RowMatches{}, err
	}

	total, err := s.collection(search.Namespace, search.Table).CountDocuments(ctx, filter)
	if err != nil {
		return driver.RowMatches{}, translateError(err)
	}

	// The command that would return what was counted, spelled from the **same** document that
	// counted it — which is what `findText` does for a page, and for the same reason.
	statement := "db." + key(search.Table) + ".find(" + documentText(filter) + ")"
	return driver.RowMatches{Count: total, Statement: statement}, nil
}

// UpdateCell writes one field of one document, addressed by `_id`. A cell the reader emptied is set
// to null rather than removed: null is what the grid drew and what it will draw again, and a field
// quietly unset would leave the collection's own field list one field shorter than the reader left it.
func (s *session) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	known, err := s.fields(ctx, edit.Namespace, edit.Table)
	if err != nil {
		return err
	}

	identity, err := s.identity(known, edit.Key)
	if err != nil {
		return err
	}

	update := bson.D{{
		Key:   "$set",
		Value: bson.D{{Key: edit.Column, Value: coerced(edit.Value, typeOf(known, edit.Column))}},
	}}

	_, err = s.collection(edit.Namespace, edit.Table).UpdateOne(ctx, identity, update)
	return translateError(err)
}

// InsertRow writes a new document carrying only the fields the reader filled in, so a field left
// alone is simply absent — which is a document store's own answer to a server-side default. An
// `_id` nobody supplied is minted by the server.
//
// **A field the sample never saw is still written.** The known fields decide the *order* — a document
// keeps the order it was written in, and two rows inserted from one grid should read alike — but they
// cannot decide the *contents*: the first document of an empty collection carries fields nothing has
// seen yet, and a collection where that was dropped could never be filled at all.
func (s *session) InsertRow(ctx context.Context, insert driver.RowInsert) error {
	known, err := s.fields(ctx, insert.Namespace, insert.Table)
	if err != nil {
		return err
	}

	document := bson.D{}
	written := map[string]bool{}

	add := func(name string, value *string) {
		if name == idField && value == nil {
			return
		}
		document = append(document, bson.E{Key: name, Value: coerced(value, typeOf(known, name))})
		written[name] = true
	}

	for _, one := range known {
		if value, ok := insert.Values[one.Name]; ok {
			add(one.Name, value)
		}
	}

	// Whatever is left is a field the sample had never seen, in name order — the map it arrived in
	// has none of its own, and a stable order is what makes two inserts read alike.
	rest := make([]string, 0, len(insert.Values))
	for name := range insert.Values {
		if !written[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		add(name, insert.Values[name])
	}

	_, err = s.collection(insert.Namespace, insert.Table).InsertOne(ctx, document)
	return translateError(err)
}

// typeOf is the type the sample found a field to hold, or none where it never saw it — which is what
// leaves a brand-new field's value the text it plainly is.
func typeOf(known []driver.Column, name string) string {
	if held, ok := field(known, name); ok {
		return held.Type
	}
	return ""
}

func (s *session) DeleteRow(ctx context.Context, removal driver.RowDelete) error {
	known, err := s.fields(ctx, removal.Namespace, removal.Table)
	if err != nil {
		return err
	}

	identity, err := s.identity(known, removal.Key)
	if err != nil {
		return err
	}

	_, err = s.collection(removal.Namespace, removal.Table).DeleteOne(ctx, identity)
	return translateError(err)
}

// identity is the document that names exactly one stored document, which in a document store is
// always `_id` and only ever `_id`. It goes through `sqlkit.EditableKeys` for the same reason both
// SQL adapters do — a write whose key the caller did not carry names no single row — and the value
// is read back into the type the field holds, so an ObjectId the grid drew as hex addresses the
// document it came from.
func (s *session) identity(
	known []driver.Column,
	key map[string]string,
) (bson.D, error) {
	keys, err := sqlkit.EditableKeys(known, key)
	if err != nil {
		return nil, err
	}

	identity := bson.D{}
	for _, name := range keys {
		held, _ := field(known, name)
		identity = append(identity, bson.E{Key: name, Value: coerce(key[name], held.Type)})
	}
	return identity, nil
}

// coerced is a value on its way into a document: nil where the reader emptied the cell, and read
// into the field's own type otherwise — see coerce, which is the same translation a filter's value
// goes through.
func coerced(value *string, kind string) any {
	if value == nil {
		return nil
	}
	return coerce(*value, kind)
}

// fields is this engine's catalogue read, and the one place the difference between a document store
// and a table store is paid for: a collection declares nothing, so what it holds is read off the
// documents it holds — see sampleDocs.
func (s *session) fields(
	ctx context.Context,
	ns driver.Namespace,
	collection string,
) ([]driver.Column, error) {
	found, err := s.sample(ctx, ns, collection, sampleDocs)
	if err != nil {
		return nil, err
	}
	return found.list(), nil
}

// sample reads what a collection's documents turned out to carry.
//
// `_id` is always among them whatever the sample found, because every document has one: that is what
// lets an empty collection still be inserted into, what makes a page of one addressable, and what
// gives completion something to offer before anything has been written.
func (s *session) sample(
	ctx context.Context,
	ns driver.Namespace,
	collection string,
	limit int64,
) (*fieldSet, error) {
	cursor, err := s.collection(ns, collection).
		Find(ctx, bson.D{}, options.Find().SetLimit(limit))
	if err != nil {
		return nil, translateError(err)
	}
	defer cursor.Close(ctx)

	found := newFieldSet()
	for cursor.Next(ctx) {
		found.addDocument(bson.Raw(cursor.Current))
	}
	if err := cursor.Err(); err != nil {
		return nil, translateError(err)
	}

	found.add(idField, bson.TypeObjectID)
	return found, nil
}
