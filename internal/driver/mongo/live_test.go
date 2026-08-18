package mongo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A live run against a real server, because the two things about this adapter that cannot be checked
// any other way are exactly the two that make it different from a SQL one: a field list is *inferred*
// from documents, so it is only right if real documents say so; and a command is *interpreted* rather
// than passed through, so only a server can confirm the method did what its name says.
//
// It is skipped unless `VIBE_MONGO_DSN` names a server — see docs/development.md — and it works in a
// database of its own so nothing it does can be mistaken for something the reader put there.
//
//	VIBE_MONGO_DSN=root:mongodb@127.0.0.1:27017 go test ./internal/driver/mongo -run TestLive -v
const liveEnv = "VIBE_MONGO_DSN"

const liveDatabase = "vibe_db_live_test"

func TestLiveAgainstARealServer(t *testing.T) {
	live, ns := openLive(t)

	// The catalogue reads a reader meets before anything else.
	t.Run("ServerInfo", func(t *testing.T) {
		info, err := live.ServerInfo(context.Background())
		if err != nil {
			t.Fatalf("ServerInfo() error = %v", err)
		}
		if info.Version == "" {
			t.Error("Version is empty")
		}
		t.Logf("server version %s", info.Version)
	})

	t.Run("CreateTable and Objects", func(t *testing.T) {
		ctx := context.Background()

		spec := driver.TableSpec{
			Namespace: ns,
			Table:     "people",
			Columns: []driver.ColumnSpec{
				{Name: "name", Type: "string"},
				{Name: "age", Type: "int", Nullable: true},
			},
		}
		if err := live.CreateTable(ctx, spec); err != nil {
			t.Fatalf("CreateTable() error = %v", err)
		}

		objects, err := live.Objects(ctx, ns)
		if err != nil {
			t.Fatalf("Objects() error = %v", err)
		}
		if len(objects.Tables) != 1 || objects.Tables[0] != "people" {
			t.Errorf("Tables = %v, want [people]", objects.Tables)
		}
		if len(objects.Routines) != 0 {
			t.Errorf("Routines = %v, want none — MongoDB has no stored routines", objects.Routines)
		}
	})

	// An empty collection still has `_id`, which is what lets one be inserted into at all.
	t.Run("fields of an empty collection", func(t *testing.T) {
		fields, err := live.fields(context.Background(), ns, "people")
		if err != nil {
			t.Fatalf("fields() error = %v", err)
		}
		if len(fields) != 1 || fields[0].Name != idField || !fields[0].PrimaryKey {
			t.Errorf("fields() = %v, want _id alone and keyed", fields)
		}
	})

	// The sample asks the server for `$type` and takes its answer as the column's type, so the two
	// lists have to be one list. Only a server can say they are.
	t.Run("fields are typed in the server's own names", func(t *testing.T) {
		ctx := context.Background()

		document := bson.D{
			{Key: "s", Value: "text"},
			{Key: "n", Value: int32(1)},
			{Key: "big", Value: int64(9000000000)},
			{Key: "f", Value: 1.5},
			{Key: "b", Value: true},
			{Key: "when", Value: time.Now()},
			{Key: "nested", Value: bson.D{{Key: "a", Value: 1}}},
			{Key: "list", Value: bson.A{1, 2}},
			{Key: "nothing", Value: nil},
			{Key: "bin", Value: bson.Binary{Data: []byte{1, 2}}},
			{Key: "dec", Value: bson.NewDecimal128(0, 42)},
		}
		if _, err := live.collection(ns, "kinds").InsertOne(ctx, document); err != nil {
			t.Fatalf("InsertOne() error = %v", err)
		}
		// It goes again, because a later run reads the whole collection list.
		t.Cleanup(func() {
			if err := live.collection(ns, "kinds").Drop(ctx); err != nil {
				t.Errorf("Drop() error = %v", err)
			}
		})

		fields, err := live.fields(ctx, ns, "kinds")
		if err != nil {
			t.Fatalf("fields() error = %v", err)
		}

		want := map[string]string{
			idField: "objectId", "s": "string", "n": "int", "big": "long", "f": "double",
			"b": "bool", "when": "date", "nested": "object", "list": "array",
			"nothing": "null", "bin": "binData", "dec": "decimal",
		}
		if len(fields) != len(want) {
			t.Errorf("fields() = %d columns, want %d", len(fields), len(want))
		}
		for _, one := range fields {
			if one.Type != want[one.Name] {
				t.Errorf("field %q type = %q, want %q", one.Name, one.Type, want[one.Name])
			}
		}
	})

	t.Run("InsertRow and TableRows", func(t *testing.T) {
		ctx := context.Background()

		for _, one := range []map[string]*string{
			{"name": ptr("Ada"), "age": ptr("36")},
			{"name": ptr("Grace"), "age": ptr("45")},
			// A document with a field the others do not have, which is the case a table store
			// cannot produce and this adapter has to show.
			{"name": ptr("Alan"), "tags": ptr(`["logic", "code"]`)},
		} {
			insert := driver.RowInsert{Namespace: ns, Table: "people", Values: one}
			if err := live.InsertRow(ctx, insert); err != nil {
				t.Fatalf("InsertRow(%v) error = %v", one, err)
			}
		}

		page := driver.TablePage{Namespace: ns, Table: "people", Limit: 10}
		rows, err := live.TableRows(ctx, page)
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}

		if rows.Total != 3 {
			t.Errorf("Total = %d, want 3", rows.Total)
		}
		if len(rows.Rows) != 3 {
			t.Fatalf("Rows = %d, want 3", len(rows.Rows))
		}
		// A bounded read shows every field its own documents carried, `tags` included.
		if !hasColumn(rows.Columns, "tags") {
			t.Errorf("Columns = %v, want the tags one document had", columnNames(rows.Columns))
		}
		if rows.Columns[0].Name != idField {
			t.Errorf("first column = %q, want %q", rows.Columns[0].Name, idField)
		}
		t.Logf("columns %v", columnNames(rows.Columns))
		t.Logf("statement %s", rows.Statement)
	})

	t.Run("TableRows narrowed and sorted", func(t *testing.T) {
		page := driver.TablePage{
			Namespace: ns,
			Table:     "people",
			Limit:     10,
			Sort:      []driver.TableSort{{Column: "name"}},
			Filter: driver.TableGroup{
				Children: []driver.TableCriterion{
					driver.TableFilter{
						Column: "age",
						Op:     driver.FilterGreaterEqual,
						Values: []string{"40"},
					},
				},
			},
		}

		rows, err := live.TableRows(context.Background(), page)
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 1 {
			t.Errorf("Total = %d, want 1 — the filter has to narrow the count too", rows.Total)
		}
		if len(rows.Rows) != 1 {
			t.Fatalf("Rows = %d, want 1", len(rows.Rows))
		}
		if got := valueOf(rows, "name", 0); got != "Grace" {
			t.Errorf("name = %q, want Grace", got)
		}
	})

	t.Run("ColumnValues", func(t *testing.T) {
		pick := driver.ColumnPick{Namespace: ns, Table: "people", Column: "name", Limit: 10}
		values, err := live.ColumnValues(context.Background(), pick)
		if err != nil {
			t.Fatalf("ColumnValues() error = %v", err)
		}
		if len(values) != 3 {
			t.Errorf("ColumnValues() = %v, want three names", values)
		}
	})

	// A field's type is read off the documents, so an `_id` the reader typed has to be read back
	// into whatever type it turned out to be.
	t.Run("UpdateCell and DeleteRow by _id", func(t *testing.T) {
		ctx := context.Background()

		page := driver.TablePage{Namespace: ns, Table: "people", Limit: 1}
		rows, err := live.TableRows(ctx, page)
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		id := valueOf(rows, idField, 0)
		if id == "" {
			t.Fatal("no _id in the page")
		}

		edit := driver.CellEdit{
			Namespace: ns,
			Table:     "people",
			Column:    "name",
			Value:     ptr("Ada Lovelace"),
			Key:       map[string]string{idField: id},
		}
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}

		after, err := live.TableRows(ctx, page)
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if got := valueOf(after, "name", 0); got != "Ada Lovelace" {
			t.Errorf("name = %q, want the edit to have landed", got)
		}

		removal := driver.RowDelete{
			Namespace: ns,
			Table:     "people",
			Key:       map[string]string{idField: id},
		}
		if err := live.DeleteRow(ctx, removal); err != nil {
			t.Fatalf("DeleteRow() error = %v", err)
		}

		left, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: "people", Limit: 10})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if left.Total != 2 {
			t.Errorf("Total = %d, want 2 after the delete", left.Total)
		}
	})

	t.Run("indexes and TableStructure", func(t *testing.T) {
		ctx := context.Background()

		spec := driver.IndexSpec{Name: "name_1", Columns: []string{"name"}, Unique: true}
		if err := live.CreateIndex(ctx, ns, "people", spec); err != nil {
			t.Fatalf("CreateIndex() error = %v", err)
		}

		structure, err := live.TableStructure(ctx, ns, "people")
		if err != nil {
			t.Fatalf("TableStructure() error = %v", err)
		}
		if len(structure.ForeignKeys) != 0 {
			t.Errorf("ForeignKeys = %v, want none", structure.ForeignKeys)
		}
		if len(structure.Indexes) != 2 {
			t.Fatalf("Indexes = %d, want the key's and the one just made", len(structure.Indexes))
		}
		if !structure.Indexes[0].Primary {
			t.Error("the first index is not the key's, and it should be ordered there")
		}

		found := false
		for _, index := range structure.Indexes {
			if index.Name == "name_1" {
				found = true
				if !index.Unique {
					t.Error("name_1 is not reported unique")
				}
				if len(index.Columns) != 1 || index.Columns[0] != "name" {
					t.Errorf("name_1 columns = %v, want [name]", index.Columns)
				}
			}
		}
		if !found {
			t.Error("name_1 is not in the structure")
		}

		if err := live.DropIndex(ctx, ns, "people", "name_1"); err != nil {
			t.Fatalf("DropIndex() error = %v", err)
		}
	})

	// A field is not a column, so each of these is an update over the documents.
	t.Run("field DDL", func(t *testing.T) {
		ctx := context.Background()

		add := driver.ColumnSpec{Name: "tier", Type: "string", Default: ptr(`"bronze"`)}
		if err := live.AddColumn(ctx, ns, "people", add); err != nil {
			t.Fatalf("AddColumn() error = %v", err)
		}
		if got := oneValue(t, live, ns, "tier"); got != "bronze" {
			t.Errorf("tier = %q, want the default written onto the documents", got)
		}

		if err := live.RenameColumn(ctx, ns, "people", "tier", "level"); err != nil {
			t.Fatalf("RenameColumn() error = %v", err)
		}
		if got := oneValue(t, live, ns, "level"); got != "bronze" {
			t.Errorf("level = %q, want the rename to have carried the value", got)
		}

		// `age` was written as an int; converting it to a string is the one field change with no
		// SQL-shaped answer, and it is a pipeline update.
		alter := driver.ColumnSpec{Name: "age", Type: "string"}
		if err := live.AlterColumn(ctx, ns, "people", "age", alter); err != nil {
			t.Fatalf("AlterColumn() error = %v", err)
		}
		fields, err := live.fields(ctx, ns, "people")
		if err != nil {
			t.Fatalf("fields() error = %v", err)
		}
		if held, ok := field(fields, "age"); !ok || held.Type != "string" {
			t.Errorf("age = %v, want it converted to a string", held)
		}

		if err := live.DropColumn(ctx, ns, "people", "level"); err != nil {
			t.Fatalf("DropColumn() error = %v", err)
		}
		fields, err = live.fields(ctx, ns, "people")
		if err != nil {
			t.Fatalf("fields() error = %v", err)
		}
		if _, ok := field(fields, "level"); ok {
			t.Error("level survived the drop")
		}
	})

	// What the engine has not got says so here rather than at the server.
	t.Run("what it has not got", func(t *testing.T) {
		ctx := context.Background()

		refusals := map[string]error{
			"a foreign key": live.CreateForeignKey(ctx, ns, "people", driver.ForeignKeySpec{
				Name: "fk", Columns: []string{"name"}, ReferencedTable: "other",
				ReferencedColumns: []string{"name"},
			}),
			"dropping a foreign key": live.DropForeignKey(ctx, ns, "people", "fk"),
			"another primary key":    live.SetPrimaryKey(ctx, ns, "people", []string{"name"}),
			"a routine": live.DropSource(ctx, ns,
				driver.SourceRef{Kind: driver.SourceFunction, Name: "f"}),
			"a trigger": live.DropSource(ctx, ns,
				driver.SourceRef{Kind: driver.SourceTrigger, Name: "t", Table: "people"}),
		}

		for what, err := range refusals {
			if err == nil {
				t.Errorf("%s: no error, want it refused", what)
				continue
			}
			if !errorsIsUnsupported(err) {
				t.Errorf("%s: error = %v, want domain.ErrUnsupported", what, err)
			}
		}

		// `_id` alone is already true, so asking for it is not a refusal.
		if err := live.SetPrimaryKey(ctx, ns, "people", []string{idField}); err != nil {
			t.Errorf("SetPrimaryKey(_id) = %v, want no error", err)
		}
	})

	t.Run("Query", func(t *testing.T) {
		ctx := context.Background()

		tests := []struct {
			name    string
			command string
			rows    int
			columns bool
		}{
			{"find", `db.people.find({})`, 2, true},
			{"find narrowed", `db.people.find({ name: "Grace" })`, 1, true},
			{"find bounded", `db.people.find({}).sort({ name: 1 }).limit(1)`, 1, true},
			{"count", `db.people.countDocuments({})`, 1, true},
			{"aggregate", `db.people.aggregate([{ $group: { _id: null, n: { $sum: 1 } } }])`, 1, true},
			{"distinct", `db.people.distinct("name")`, 2, true},
			{"getIndexes", `db.people.getIndexes()`, 1, true},
			{"runCommand", `db.runCommand({ ping: 1 })`, 1, true},
			{"collection names", `db.getCollectionNames()`, 1, true},
			{"stats", `db.stats()`, 1, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := live.Query(ctx, driver.Query{
					Namespace: ns,
					SQL:       tt.command,
					MaxRows:   100,
				})
				if err != nil {
					t.Fatalf("Query(%s) error = %v", tt.command, err)
				}
				if len(result.Rows) != tt.rows {
					t.Errorf("Query(%s) = %d rows, want %d", tt.command, len(result.Rows), tt.rows)
				}
				if tt.columns && len(result.Columns) == 0 {
					t.Errorf("Query(%s) returned no columns", tt.command)
				}
			})
		}
	})

	// A plain find on one collection is writable; one that cannot carry `_id` back is not.
	t.Run("Query marks a writable result", func(t *testing.T) {
		ctx := context.Background()

		writable, err := live.Query(ctx, driver.Query{Namespace: ns, SQL: `db.people.find({})`})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if writable.Table != "people" || writable.Database != liveDatabase {
			t.Errorf("source = %s.%s, want %s.people", writable.Database, writable.Table, liveDatabase)
		}

		projected, err := live.Query(ctx, driver.Query{
			Namespace: ns,
			SQL:       `db.people.find({}, { name: 1, _id: 0 })`,
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if projected.Table != "" {
			t.Errorf("table = %q, want none — a projection without _id names no stored document",
				projected.Table)
		}
	})

	t.Run("Query writes and counts", func(t *testing.T) {
		ctx := context.Background()

		commands := []struct {
			command string
			want    int64
		}{
			{`db.people.insertOne({ name: "Edsger" })`, 1},
			{`db.people.updateMany({ name: "Edsger" }, { $set: { note: "sorted" } })`, 1},
			{`db.people.deleteMany({ name: "Edsger" })`, 1},
		}

		for _, tt := range commands {
			result, err := live.Query(ctx, driver.Query{Namespace: ns, SQL: tt.command})
			if err != nil {
				t.Fatalf("Query(%s) error = %v", tt.command, err)
			}
			if result.RowsAffected != tt.want {
				t.Errorf("Query(%s) affected %d, want %d", tt.command, result.RowsAffected, tt.want)
			}
			if len(result.Rows) != 0 {
				t.Errorf("Query(%s) returned rows, want none", tt.command)
			}
		}
	})

	t.Run("Explain", func(t *testing.T) {
		result, err := live.Query(context.Background(), driver.Query{
			Namespace: ns,
			SQL:       `db.people.find({ name: "Grace" })`,
			Explain:   true,
		})
		if err != nil {
			t.Fatalf("Query(explain) error = %v", err)
		}
		if len(result.Rows) != 1 || len(result.Columns) == 0 {
			t.Fatalf("explain = %d rows and %d columns, want one row of a plan",
				len(result.Rows), len(result.Columns))
		}

		if _, err := live.Query(context.Background(), driver.Query{
			Namespace: ns,
			SQL:       `db.people.insertOne({ a: 1 })`,
			Explain:   true,
		}); !errorsIsUnsupported(err) {
			t.Errorf("explaining an insert = %v, want it refused", err)
		}
	})

	t.Run("TableList", func(t *testing.T) {
		list, err := live.TableList(context.Background(), ns)
		if err != nil {
			t.Fatalf("TableList() error = %v", err)
		}
		if len(list.Rows) != 1 {
			t.Fatalf("TableList() = %d rows, want 1", len(list.Rows))
		}
		if len(list.Rows[0]) != len(tableListColumns) {
			t.Errorf("row has %d cells, want %d", len(list.Rows[0]), len(tableListColumns))
		}
		for at, cell := range list.Rows[0] {
			if cell == nil {
				t.Errorf("%s is null on a real collection", tableListColumns[at].Name)
			}
		}
	})

	t.Run("TableSchemas", func(t *testing.T) {
		tables, err := live.TableSchemas(context.Background(), ns)
		if err != nil {
			t.Fatalf("TableSchemas() error = %v", err)
		}
		if len(tables) != 1 || tables[0].Name != "people" {
			t.Fatalf("TableSchemas() = %v, want one collection", tables)
		}
		if len(tables[0].Columns) == 0 {
			t.Error("no fields for completion to offer")
		}
	})

	// A view is a collection whose contents are an aggregation, and it has no storage of its own —
	// so it is listed as a view and its row in the collection list carries no numbers.
	t.Run("a view", func(t *testing.T) {
		ctx := context.Background()

		if _, err := live.Query(ctx, driver.Query{
			Namespace: ns,
			SQL:       `db.createView("adults", "people", [{ $match: { name: { $exists: true } } }])`,
		}); err != nil {
			t.Fatalf("createView error = %v", err)
		}

		objects, err := live.Objects(ctx, ns)
		if err != nil {
			t.Fatalf("Objects() error = %v", err)
		}
		if len(objects.Views) != 1 || objects.Views[0] != "adults" {
			t.Errorf("Views = %v, want [adults]", objects.Views)
		}

		list, err := live.TableList(ctx, ns)
		if err != nil {
			t.Fatalf("TableList() error = %v", err)
		}
		for _, row := range list.Rows {
			if row[0] != nil && *row[0] == "adults" {
				if row[1] == nil || *row[1] != "view" {
					t.Errorf("kind = %v, want view", show(row[1]))
				}
				if row[2] != nil {
					t.Errorf("Documents = %v, want nothing — a view has no storage", show(row[2]))
				}
			}
		}

		// A view's rows read like any other collection's.
		rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: "adults", Limit: 10})
		if err != nil {
			t.Fatalf("TableRows(view) error = %v", err)
		}
		if len(rows.Rows) != 2 {
			t.Errorf("view rows = %d, want 2", len(rows.Rows))
		}

		view := driver.SourceRef{Kind: driver.SourceView, Name: "adults"}
		if err := live.DropSource(ctx, ns, view); err != nil {
			t.Fatalf("DropSource(view) error = %v", err)
		}
	})

	// A copy is a second collection, and the two ways of asking for one differ only in whether the
	// documents come with it. The indexes come either way.
	t.Run("DuplicateTable", func(t *testing.T) {
		ctx := context.Background()

		spec := driver.IndexSpec{Name: "name_1", Columns: []string{"name"}}
		if err := live.CreateIndex(ctx, ns, "people", spec); err != nil {
			t.Fatalf("CreateIndex() error = %v", err)
		}

		source, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: "people"})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}

		copies := []struct {
			name     string
			withRows bool
			want     int64
		}{
			{"people_shape", false, 0},
			{"people_copy", true, source.Total},
		}

		for _, one := range copies {
			if err := live.DuplicateTable(ctx, ns, "people", one.name, one.withRows); err != nil {
				t.Fatalf("DuplicateTable(%s) error = %v", one.name, err)
			}

			rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: one.name})
			if err != nil {
				t.Fatalf("TableRows(%s) error = %v", one.name, err)
			}
			if rows.Total != one.want {
				t.Errorf("%s holds %d documents, want %d", one.name, rows.Total, one.want)
			}

			// Both indexes, which is what `$out` would have left behind had it run last.
			structure, err := live.TableStructure(ctx, ns, one.name)
			if err != nil {
				t.Fatalf("TableStructure(%s) error = %v", one.name, err)
			}
			if len(structure.Indexes) != 2 {
				t.Errorf("%s has %d indexes, want _id_ and name_1", one.name, len(structure.Indexes))
			}

			if err := live.DropTable(ctx, ns, one.name); err != nil {
				t.Fatalf("DropTable(%s) error = %v", one.name, err)
			}
		}

		// A name already taken is refused rather than written over, which is what making the copy
		// before filling it is for.
		if err := live.DuplicateTable(ctx, ns, "people", "people", true); err == nil {
			t.Error("DuplicateTable() over an existing collection was allowed")
		}

		if err := live.DropIndex(ctx, ns, "people", "name_1"); err != nil {
			t.Fatalf("DropIndex() error = %v", err)
		}
	})

	t.Run("Empty, Rename and Drop", func(t *testing.T) {
		ctx := context.Background()

		if err := live.EmptyTable(ctx, ns, "people"); err != nil {
			t.Fatalf("EmptyTable() error = %v", err)
		}
		rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: "people", Limit: 10})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 0 {
			t.Errorf("Total = %d, want 0", rows.Total)
		}

		if err := live.RenameTable(ctx, ns, "people", "humans"); err != nil {
			t.Fatalf("RenameTable() error = %v", err)
		}
		objects, err := live.Objects(ctx, ns)
		if err != nil {
			t.Fatalf("Objects() error = %v", err)
		}
		if len(objects.Tables) != 1 || objects.Tables[0] != "humans" {
			t.Errorf("Tables = %v, want [humans]", objects.Tables)
		}

		if err := live.DropTable(ctx, ns, "humans"); err != nil {
			t.Fatalf("DropTable() error = %v", err)
		}
	})

	// The whole reason `FirstCollection` exists: a database is made by making one.
	t.Run("CreateDatabase makes its first collection", func(t *testing.T) {
		ctx := context.Background()
		const second = liveDatabase + "_2"

		spec := driver.DatabaseSpec{Name: second, FirstCollection: "seed"}
		statements := live.CreateDatabaseSQL(spec)
		if len(statements) != 1 || statements[0].SQL == "" {
			t.Fatalf("CreateDatabaseSQL() = %v, want one command", statements)
		}
		t.Logf("preview %s", statements[0].SQL)

		if err := live.CreateDatabase(ctx, spec); err != nil {
			t.Fatalf("CreateDatabase() error = %v", err)
		}
		defer func() {
			if err := live.DropDatabase(ctx, second); err != nil {
				t.Errorf("DropDatabase() error = %v", err)
			}
		}()

		objects, err := live.Objects(ctx, driver.Namespace{Database: second})
		if err != nil {
			t.Fatalf("Objects() error = %v", err)
		}
		if len(objects.Tables) != 1 || objects.Tables[0] != "seed" {
			t.Errorf("Tables = %v, want [seed]", objects.Tables)
		}
	})
}

// openLive dials the server named in the environment and hands back a clean database of its own,
// dropped when the test ends.
func openLive(t *testing.T) (*session, driver.Namespace) {
	t.Helper()

	dsn := os.Getenv(liveEnv)
	if dsn == "" {
		t.Skipf("%s is not set; see docs/development.md", liveEnv)
	}

	profile, creds := liveProfile(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opened, err := New().Open(ctx, profile, creds, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	live, ok := opened.(*session)
	if !ok {
		t.Fatalf("Open() = %T, want *session", opened)
	}

	ns := driver.Namespace{Database: liveDatabase}
	// A run that died before its own cleanup leaves the database behind, so it goes first as well.
	_ = live.DropDatabase(context.Background(), liveDatabase)

	t.Cleanup(func() {
		if err := live.DropDatabase(context.Background(), liveDatabase); err != nil {
			t.Errorf("DropDatabase() error = %v", err)
		}
		if err := live.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	return live, ns
}

// liveProfile reads `user:password@host:port`, the password and the user both optional — a server
// started without access control takes neither.
func liveProfile(t *testing.T, dsn string) (domain.ConnectionProfile, driver.Credentials) {
	t.Helper()

	user, password, address := "", "", dsn
	if at := lastIndex(dsn, '@'); at >= 0 {
		user, address = dsn[:at], dsn[at+1:]
		if colon := lastIndex(user, ':'); colon >= 0 {
			user, password = user[:colon], user[colon+1:]
		}
	}

	host, port := address, domain.DefaultMongoPort
	if colon := lastIndex(address, ':'); colon >= 0 {
		host = address[:colon]
		port = atoi(address[colon+1:], domain.DefaultMongoPort)
	}

	profile := domain.ConnectionProfile{
		Name:   "live",
		Engine: domain.EngineMongo,
		Host:   host,
		Port:   port,
		User:   user,
		Auth:   domain.AuthPassword,
		TLS:    domain.TLSOptions{Mode: domain.TLSDisabled},
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("the profile %s builds is invalid: %v", liveEnv, err)
	}
	return profile, driver.Credentials{Password: password}
}

func lastIndex(text string, want byte) int {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == want {
			return i
		}
	}
	return -1
}

func atoi(text string, fallback int) int {
	value := 0
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return fallback
		}
		value = value*10 + int(text[i]-'0')
	}
	if value == 0 {
		return fallback
	}
	return value
}

func hasColumn(columns []driver.Column, name string) bool {
	_, ok := field(columns, name)
	return ok
}

func columnNames(columns []driver.Column) []string {
	names := make([]string, 0, len(columns))
	for _, one := range columns {
		names = append(names, one.Name)
	}
	return names
}

func valueOf(rows driver.TableRows, column string, row int) string {
	for at, one := range rows.Columns {
		if one.Name != column || row >= len(rows.Rows) || at >= len(rows.Rows[row]) {
			continue
		}
		if value := rows.Rows[row][at]; value != nil {
			return *value
		}
	}
	return ""
}

func oneValue(t *testing.T, live *session, ns driver.Namespace, column string) string {
	t.Helper()

	rows, err := live.TableRows(context.Background(), driver.TablePage{
		Namespace: ns,
		Table:     "people",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("TableRows() error = %v", err)
	}
	return valueOf(rows, column, 0)
}

func errorsIsUnsupported(err error) bool {
	return errors.Is(err, domain.ErrUnsupported)
}

// The monitoring half against a real server. A document store's answer to "what is this server doing"
// is a list of **operations**, and the two halves of its status are nested documents — so what only a
// server can check is the flattening: a path this side built wrongly is a row nobody can look up.
//
// There is no kill here. `killOp` needs an operation of somebody else's to be honest about, and this
// test has none to spare.
func TestLiveMonitor(t *testing.T) {
	live, _ := openLive(t)
	ctx := context.Background()

	t.Run("operations", func(t *testing.T) {
		list, err := live.Processes(ctx)
		if err != nil {
			t.Fatalf("Processes() error = %v", err)
		}
		if len(list.Refs) != len(list.Rows) {
			t.Fatalf("%d references for %d rows", len(list.Refs), len(list.Rows))
		}
		// An idle connection is not an operation, so an unused server may report none at all — what is
		// being checked is the shape rather than the count.
		for at, ref := range list.Refs {
			if ref.ID == "" {
				t.Errorf("operation %d has no id", at)
			}
		}
	})

	t.Run("status and parameters", func(t *testing.T) {
		status, err := live.ServerStatus(ctx)
		if err != nil {
			t.Fatalf("ServerStatus() error = %v", err)
		}
		if len(status.Rows) == 0 {
			t.Fatal("serverStatus answered with nothing")
		}
		// `ok` is the command reporting that it worked rather than a fact about the server, so it is
		// not one of the rows.
		for _, row := range status.Rows {
			if row[0] != nil && *row[0] == "ok" {
				t.Error("`ok` is in the status")
			}
		}
		// A nested document is rows under a dotted path, which is what makes the list searchable.
		if !anyName(status.Rows, ".") {
			t.Error("nothing in the status is a nested path")
		}

		parameters, err := live.ServerVariables(ctx)
		if err != nil {
			t.Fatalf("ServerVariables() error = %v", err)
		}
		if len(parameters.Rows) == 0 {
			t.Error("getParameter answered with nothing")
		}
	})
}

func anyName(rows [][]*string, holding string) bool {
	for _, row := range rows {
		if row[0] != nil && strings.Contains(*row[0], holding) {
			return true
		}
	}
	return false
}
