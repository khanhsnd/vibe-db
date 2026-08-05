package sqlkit

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestTableQualifiesByTheEngineLevel(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	if got, want := MySQL.Table(ns, "users"), "`shop`.`users`"; got != want {
		t.Errorf("MySQL.Table = %s, want %s", got, want)
	}
	if got, want := Postgres.Table(ns, "users"), `"public"."users"`; got != want {
		t.Errorf("Postgres.Table = %s, want %s", got, want)
	}
}

func TestColumnClauseCarriesTypeAndDefaultVerbatim(t *testing.T) {
	expression := "now()"
	spec := driver.ColumnSpec{
		Name:     "created_at",
		Type:     " timestamptz ",
		Nullable: false,
		Default:  &expression,
	}

	got := Postgres.ColumnClause(spec)
	want := `"created_at" timestamptz NOT NULL DEFAULT now()`

	if got != want {
		t.Errorf("ColumnClause = %s, want %s", got, want)
	}
}

// A default that is present but blank is no default at all, or the statement would end in a
// dangling DEFAULT keyword.
func TestColumnClauseDropsABlankDefault(t *testing.T) {
	blank := "   "
	spec := driver.ColumnSpec{Name: "note", Type: "text", Nullable: true, Default: &blank}

	got := Postgres.ColumnClause(spec)
	want := `"note" text`

	if got != want {
		t.Errorf("ColumnClause = %s, want %s", got, want)
	}
}

// Neither engine will key a nullable column, so a key column is NOT NULL whatever it claimed.
func TestColumnClauseForcesNotNullOnAKeyColumn(t *testing.T) {
	spec := driver.ColumnSpec{Name: "id", Type: "uuid", Nullable: true, PrimaryKey: true}

	got := Postgres.ColumnClause(spec)
	want := `"id" uuid NOT NULL`

	if got != want {
		t.Errorf("ColumnClause = %s, want %s", got, want)
	}
}

func TestCreateTableCollectsOneKeyClause(t *testing.T) {
	spec := driver.TableSpec{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "order_lines",
		Columns: []driver.ColumnSpec{
			{Name: "order_id", Type: "int", PrimaryKey: true},
			{Name: "line", Type: "int", PrimaryKey: true},
			{Name: "note", Type: "text", Nullable: true},
		},
	}

	got := MySQL.CreateTable(spec)
	want := "CREATE TABLE `shop`.`order_lines` (\n" +
		"  `order_id` int NOT NULL,\n" +
		"  `line` int NOT NULL,\n" +
		"  `note` text,\n" +
		"  PRIMARY KEY (`order_id`, `line`)\n" +
		")"

	if got != want {
		t.Errorf("CreateTable =\n%s\nwant\n%s", got, want)
	}
}

func TestCreateTableWithoutAKeyHasNoKeyClause(t *testing.T) {
	spec := driver.TableSpec{
		Namespace: driver.Namespace{Database: "shop", Schema: "public"},
		Table:     "log",
		Columns:   []driver.ColumnSpec{{Name: "line", Type: "text", Nullable: true}},
	}

	got := Postgres.CreateTable(spec)
	want := "CREATE TABLE \"public\".\"log\" (\n  \"line\" text\n)"

	if got != want {
		t.Errorf("CreateTable =\n%s\nwant\n%s", got, want)
	}
}

// MySQL's LIKE brings the defaults and indexes unbidden; PostgreSQL's stands where a column
// would and has to be asked for them.
func TestCopyTableTakesEachEngineLikeClause(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	got := MySQL.CopyTable(ns, "orders", "orders_copy")
	if want := "CREATE TABLE `shop`.`orders_copy` LIKE `shop`.`orders`"; got != want {
		t.Errorf("MySQL.CopyTable = %s, want %s", got, want)
	}

	got = Postgres.CopyTable(ns, "orders", "orders_copy")
	want := `CREATE TABLE "public"."orders_copy" (LIKE "public"."orders" INCLUDING ALL)`
	if got != want {
		t.Errorf("Postgres.CopyTable = %s, want %s", got, want)
	}
}

// PostgreSQL refuses a row that supplies a value for the identity column its copy took with it
// unless the write says it is overriding one; MySQL has no such clause.
func TestCopyRowsOverridesIdentityWhereTheEngineDemandsIt(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	got := MySQL.CopyRows(ns, "orders", "orders_copy")
	if want := "INSERT INTO `shop`.`orders_copy` SELECT * FROM `shop`.`orders`"; got != want {
		t.Errorf("MySQL.CopyRows = %s, want %s", got, want)
	}

	got = Postgres.CopyRows(ns, "orders", "orders_copy")
	want := `INSERT INTO "public"."orders_copy" OVERRIDING SYSTEM VALUE ` +
		`SELECT * FROM "public"."orders"`
	if got != want {
		t.Errorf("Postgres.CopyRows = %s, want %s", got, want)
	}
}

func TestAppendIndexColumnFoldsOneEntryPerIndex(t *testing.T) {
	indexes := []driver.Index{}
	indexes = AppendIndexColumn(indexes, driver.Index{Name: "by_pair", Unique: true}, "a")
	indexes = AppendIndexColumn(indexes, driver.Index{Name: "by_pair", Unique: true}, "b")
	indexes = AppendIndexColumn(indexes, driver.Index{Name: "by_c"}, "c")

	if len(indexes) != 2 {
		t.Fatalf("got %d indexes, want 2", len(indexes))
	}
	if got := indexes[0].Columns; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("by_pair columns = %v, want [a b]", got)
	}
	if !indexes[0].Unique {
		t.Error("by_pair lost its Unique flag")
	}
	if got := indexes[1].Columns; len(got) != 1 || got[0] != "c" {
		t.Errorf("by_c columns = %v, want [c]", got)
	}
}

func TestAppendKeyColumnPairsBothSides(t *testing.T) {
	keys := []driver.ForeignKey{}
	found := driver.ForeignKey{Name: "fk", ReferencedTable: "parent"}
	keys = AppendKeyColumn(keys, found, "a", "x")
	keys = AppendKeyColumn(keys, found, "b", "y")

	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1", len(keys))
	}
	if got := keys[0].Columns; len(got) != 2 || got[1] != "b" {
		t.Errorf("columns = %v, want [a b]", got)
	}
	if got := keys[0].ReferencedColumns; len(got) != 2 || got[1] != "y" {
		t.Errorf("referenced columns = %v, want [x y]", got)
	}
}
