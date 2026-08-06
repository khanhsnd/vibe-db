package mongo

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A field of a document is not a column of a table, and that is the whole of what this file is
// about. There is nothing to alter: a collection holds no declaration to change, so a change to a
// *field* is a change to the documents that carry it — which the server does with one update and an
// aggregation operator, and which is exactly as reversible as the equivalent `ALTER TABLE` is.
//
//	Add a field     $set on the documents that lack it, so an existing document keeps its value
//	Rename a field  $rename
//	Drop a field    $unset
//	Alter a field   $convert, in a pipeline update — the one that has no SQL-shaped answer at all
//
// Two things a table has, a collection has not, and both say so rather than being approximated:
// a primary key other than `_id`, and a foreign key. See domain.Unsupported.

// CreateTable creates the collection, and **only** the collection: the declared fields are the one
// part of a TableSpec a document store has nothing to do with.
//
// The tempting alternative was a `$jsonSchema` validator built from them, and it is a trap. A
// validator has to be kept in step by every field change that follows, and the moment one is not —
// `AlterColumn` converting a field the schema still calls an `int` — the collection starts rejecting
// the very edits this designer offers. A collection that will not accept its own designer's writes is
// worse than one that declared nothing, which is what a MongoDB collection normally is: no other
// client writes a validator for a new collection either.
//
// So a new collection's fields arrive with the first document, and the Fields grid reads them back off
// it — which is the same thing it does for every other collection.
func (s *session) CreateTable(ctx context.Context, spec driver.TableSpec) error {
	if err := domain.RequireName("collection", spec.Table); err != nil {
		return err
	}

	err := s.database(spec.Namespace).CreateCollection(ctx, spec.Table)
	return translateError(err)
}

// RenameTable is an `admin` command rather than a collection method, and it names both collections
// in full — `db.coll` — because the server allows a rename to cross databases.
func (s *session) RenameTable(ctx context.Context, ns driver.Namespace, table, name string) error {
	if err := domain.RequireNames("collection", table, "collection", name); err != nil {
		return err
	}

	command := bson.D{
		{Key: "renameCollection", Value: s.qualified(ns, table)},
		{Key: "to", Value: s.qualified(ns, name)},
	}
	_, err := s.runCommand(ctx, driver.Namespace{Database: adminDatabase}, command)
	return err
}

// DuplicateTable copies a collection, with its documents or with its indexes alone. There is no
// command that does it: `$out` writes an aggregation's documents into a collection and makes one
// where there is none — even out of a source holding nothing — but it copies no index, and the
// indexes are therefore built afterwards on whichever collection is by then there.
//
// **The name is checked here rather than left to the server.** `create` answers `ok` for a
// collection it already has and `$out` writes straight over one, so between them nothing would
// stop a copy destroying a collection the reader meant to keep — which is not what the same item
// does on an engine where `CREATE TABLE` refuses a name that is taken.
func (s *session) DuplicateTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
	withRows bool,
) error {
	if err := domain.RequireNames("collection", table, "collection", name); err != nil {
		return err
	}

	taken, err := s.database(ns).ListCollectionNames(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil {
		return translateError(err)
	}
	if len(taken) > 0 {
		return domain.ValidationError{Field: "collection", Message: "already exists"}
	}

	if withRows {
		cursor, err := s.collection(ns, table).
			Aggregate(ctx, []bson.D{{{Key: "$out", Value: name}}})
		if err != nil {
			return translateError(err)
		}
		// The write is the aggregation's own and is done by the time the command answers; the
		// cursor it answers with carries no documents and is closed rather than read.
		if err := cursor.Close(ctx); err != nil {
			return translateError(err)
		}
	} else if err := s.database(ns).CreateCollection(ctx, name); err != nil {
		return translateError(err)
	}

	return s.copyIndexes(ctx, ns, table, name)
}

// copyIndexes builds on the copy every index the original keeps, `_id_` excepted: the server makes
// that one itself. The keys document travels verbatim, so a descending, compound or special index
// is copied as what it is rather than as what a read of it could describe.
func (s *session) copyIndexes(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	found, err := s.collection(ns, table).Indexes().ListSpecifications(ctx)
	if err != nil {
		return translateError(err)
	}

	models := []mgo.IndexModel{}
	for _, one := range found {
		if one.Name == idIndex {
			continue
		}

		opts := options.Index().SetName(one.Name)
		if one.Unique != nil {
			opts.SetUnique(*one.Unique)
		}
		models = append(models, mgo.IndexModel{Keys: one.KeysDocument, Options: opts})
	}
	if len(models) == 0 {
		return nil
	}

	_, err = s.collection(ns, name).Indexes().CreateMany(ctx, models)
	return translateError(err)
}

func (s *session) DropTable(ctx context.Context, ns driver.Namespace, table string) error {
	if err := domain.RequireName("collection", table); err != nil {
		return err
	}
	return translateError(s.collection(ns, table).Drop(ctx))
}

// TruncateTable and EmptyTable are the same command, and deliberately: a collection is not a table
// with storage to reset, so the only way to take every document out of one and leave the collection
// — and its indexes — standing is to delete them. Dropping and recreating would be faster and would
// silently lose every index the collection had.
func (s *session) TruncateTable(ctx context.Context, ns driver.Namespace, table string) error {
	return s.EmptyTable(ctx, ns, table)
}

func (s *session) EmptyTable(ctx context.Context, ns driver.Namespace, table string) error {
	if err := domain.RequireName("collection", table); err != nil {
		return err
	}
	_, err := s.collection(ns, table).DeleteMany(ctx, bson.D{})
	return translateError(err)
}

// AddColumn writes the field onto the documents that do not carry it, taking the declared default
// where there is one and null where there is not. **A document that already has the field keeps its
// value**, which is what makes this the counterpart of `ADD COLUMN` and not of an overwrite.
func (s *session) AddColumn(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ColumnSpec,
) error {
	if err := domain.RequireNames("collection", table, "field", spec.Name); err != nil {
		return err
	}

	// A default here is a *value* and not the expression a SQL engine would parse — there is no
	// statement for one to sit in — so it is read as the shell would read it: `"bronze"` is that
	// string, `0` is that number, `{ … }` is that document, and anything the grammar cannot read is
	// the text it plainly is.
	value := any(nil)
	if spec.Default != nil && strings.TrimSpace(*spec.Default) != "" {
		value = shellValue(strings.TrimSpace(*spec.Default))
	}

	filter := bson.D{{Key: spec.Name, Value: bson.D{{Key: "$exists", Value: false}}}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: spec.Name, Value: value}}}}

	_, err := s.collection(ns, table).UpdateMany(ctx, filter, update)
	return translateError(err)
}

// AlterColumn converts the field's values to the declared type, in place, across every document that
// carries it. A value the server cannot convert is **removed rather than left wrong**: that is what
// `onError` says, and it is the only outcome in which the collection afterwards holds what the
// reader declared it would.
//
// The clause names the field being changed rather than the name in the spec, so renaming is
// RenameColumn's alone and never a side effect of an edit — the same rule MySQL's MODIFY COLUMN
// follows here.
func (s *session) AlterColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) error {
	if err := domain.RequireNames("collection", table, "field", column); err != nil {
		return err
	}

	kind := strings.TrimSpace(spec.Type)
	if kind == "" {
		return domain.ValidationError{Field: "type", Message: "is required to convert a field"}
	}
	if column == idField {
		return domain.Unsupported("changing the type of _id")
	}

	convert := bson.D{{Key: "$convert", Value: bson.D{
		{Key: "input", Value: "$" + column},
		{Key: "to", Value: kind},
		{Key: "onError", Value: "$$REMOVE"},
		{Key: "onNull", Value: "$$REMOVE"},
	}}}

	filter := bson.D{{Key: column, Value: bson.D{{Key: "$exists", Value: true}}}}
	pipeline := []bson.D{{{Key: "$set", Value: bson.D{{Key: column, Value: convert}}}}}

	_, err := s.collection(ns, table).UpdateMany(ctx, filter, pipeline)
	return translateError(err)
}

func (s *session) RenameColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column, name string,
) error {
	if err := domain.RequireNames("collection", table, "field", column, "field", name); err != nil {
		return err
	}

	filter := bson.D{{Key: column, Value: bson.D{{Key: "$exists", Value: true}}}}
	update := bson.D{{Key: "$rename", Value: bson.D{{Key: column, Value: name}}}}

	_, err := s.collection(ns, table).UpdateMany(ctx, filter, update)
	return translateError(err)
}

func (s *session) DropColumn(ctx context.Context, ns driver.Namespace, table, column string) error {
	if err := domain.RequireNames("collection", table, "field", column); err != nil {
		return err
	}
	if column == idField {
		return domain.Unsupported("dropping _id")
	}

	update := bson.D{{Key: "$unset", Value: bson.D{{Key: column, Value: ""}}}}
	_, err := s.collection(ns, table).UpdateMany(ctx, bson.D{}, update)
	return translateError(err)
}

// SetPrimaryKey has one answer and it is no. `_id` is the key of every collection, the server mints
// it, indexes it and refuses to let it be changed — so a request naming it is already true and one
// naming anything else cannot be granted. A reader who wants some other field to be unique wants a
// unique index, which CreateIndex makes.
func (s *session) SetPrimaryKey(
	_ context.Context,
	_ driver.Namespace,
	_ string,
	columns []string,
) error {
	if len(columns) == 1 && columns[0] == idField {
		return nil
	}
	return domain.Unsupported("a primary key other than _id")
}

// CreateIndex builds the keys document, where MongoDB keeps what SQL puts in a USING clause: an
// ordinary index's key is `1`, and a special one's key *is* its method — `"hashed"`, `"text"`,
// `"2dsphere"`. An empty method is the ordinary ascending index, which is what an empty one means
// everywhere else in the app.
func (s *session) CreateIndex(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.IndexSpec,
) error {
	if err := domain.RequireNames("collection", table, "index", spec.Name); err != nil {
		return err
	}
	if len(spec.Columns) == 0 {
		return domain.ValidationError{Field: "columns", Message: "are required for an index"}
	}

	method := strings.TrimSpace(spec.Method)
	keys := bson.D{}
	for _, column := range spec.Columns {
		if err := domain.RequireName("field", column); err != nil {
			return err
		}
		if method == "" || method == "asc" {
			keys = append(keys, bson.E{Key: column, Value: 1})
			continue
		}
		keys = append(keys, bson.E{Key: column, Value: method})
	}

	model := mgo.IndexModel{
		Keys:    keys,
		Options: options.Index().SetName(spec.Name).SetUnique(spec.Unique),
	}
	_, err := s.collection(ns, table).Indexes().CreateOne(ctx, model)
	return translateError(err)
}

func (s *session) DropIndex(ctx context.Context, ns driver.Namespace, table, index string) error {
	if err := domain.RequireNames("collection", table, "index", index); err != nil {
		return err
	}
	return translateError(s.collection(ns, table).Indexes().DropOne(ctx, index))
}

// CreateForeignKey and DropForeignKey both refuse. MongoDB has no referential constraints of any
// kind: there is nothing to create, nothing to drop, and nothing that would check the documents
// afterwards. Saying so is the honest answer — the alternative is a statement the server rejects for
// a reason the reader cannot read.
func (*session) CreateForeignKey(
	context.Context,
	driver.Namespace,
	string,
	driver.ForeignKeySpec,
) error {
	return domain.Unsupported("a foreign key")
}

func (*session) DropForeignKey(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("a foreign key")
}

// InsertStatements spells documents as the commands that would put them back, which is what an
// export is. An absent value is left out of the document rather than written as null, because that
// is the difference a document store keeps: a field nobody wrote is not a field holding nothing.
//
// Every value is written with `literal`, the same renderer the grid drew it with, so what is exported
// is what was shown and what is imported is what was exported.
func (s *session) InsertStatements(
	_ driver.Namespace,
	table string,
	columns []string,
	rows [][]*string,
) []string {
	statements := make([]string, 0, len(rows))

	for _, row := range rows {
		fields := make([]string, 0, len(row))
		for at, value := range row {
			if value == nil || at >= len(columns) {
				continue
			}
			fields = append(fields, key(columns[at])+": "+valueText(shellValue(*value)))
		}
		statements = append(statements,
			"db."+key(table)+".insertOne({ "+strings.Join(fields, ", ")+" })")
	}

	return statements
}

// CreateStatements spells a collection as the commands that would make it again, which is the
// structure half of a dump.
//
// **A collection declares nothing about its documents** — see the note at the top of this file — so
// its structure is the collection itself and the indexes the server really keeps, and *nothing about
// its fields*: those arrive with the documents, which is InsertStatements' half. A dump of structure
// alone therefore writes a collection with no fields in it, and that is not a loss — it is what a
// new collection on this engine is.
//
// `_id_` is left out. The server builds that index whether or not anybody asked for one, so a
// command to build it again is a command that does nothing. Every other index travels as its own
// keys document, so a descending, compound or special index is written as what it is rather than as
// what a read of it could describe.
func (s *session) CreateStatements(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]string, error) {
	found, err := s.collection(ns, table).Indexes().ListSpecifications(ctx)
	if err != nil {
		return nil, translateError(err)
	}

	statements := []string{"db.createCollection(" + valueText(table) + ")"}
	for _, one := range found {
		if one.Name == idIndex {
			continue
		}

		opts := bson.D{{Key: "name", Value: one.Name}}
		if one.Unique != nil && *one.Unique {
			opts = append(opts, bson.E{Key: "unique", Value: true})
		}

		statements = append(statements, "db."+key(table)+".createIndex("+
			valueText(one.KeysDocument)+", "+valueText(opts)+")")
	}

	return statements, nil
}

// DropView drops the view. A MongoDB view is a collection whose contents are an aggregation over
// another, so it goes the same way an ordinary collection does — there is no second command for it.
func (s *session) DropView(ctx context.Context, ns driver.Namespace, view string) error {
	return s.DropTable(ctx, ns, view)
}

// DropRoutine refuses. Server-side JavaScript was removed from the server, so a MongoDB deployment
// holds no stored functions or procedures — which is also why `Objects` never lists any and this can
// only ever be reached by a caller that invented one.
func (*session) DropRoutine(context.Context, driver.Namespace, driver.Routine) error {
	return domain.Unsupported("a stored routine")
}
