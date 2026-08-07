package sqlkit

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func mysqlPage(table string, limit, offset int) driver.TablePage {
	return driver.TablePage{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     table,
		Limit:     limit,
		Offset:    offset,
	}
}

func TestSelectPageOrdersByPrimaryKey(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
	}

	got := MySQL.SelectPage(mysqlPage("users", 100, 200), columns, "")
	want := "SELECT `id`, `email` FROM `shop`.`users` ORDER BY `id` LIMIT 100 OFFSET 200"

	if got != want {
		t.Errorf("SelectPage =\n%s\nwant\n%s", got, want)
	}
}

func TestSelectPageWithoutPrimaryKeyHasNoOrderBy(t *testing.T) {
	columns := []driver.Column{{Name: "email"}}

	got := MySQL.SelectPage(mysqlPage("log", 100, 0), columns, "")
	want := "SELECT `email` FROM `shop`.`log` LIMIT 100 OFFSET 0"

	if got != want {
		t.Errorf("SelectPage =\n%s\nwant\n%s", got, want)
	}
}

func TestSelectPageWithoutLimitReadsEveryRow(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}}

	got := MySQL.SelectPage(mysqlPage("users", 0, 200), columns, "")
	want := "SELECT `id` FROM `shop`.`users` ORDER BY `id`"

	if got != want {
		t.Errorf("SelectPage =\n%s\nwant\n%s", got, want)
	}
}

func TestSelectPageFiltersBeforeItOrders(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}}

	got := MySQL.SelectPage(mysqlPage("users", 100, 0), columns, " WHERE `id` > '7'")
	want := "SELECT `id` FROM `shop`.`users` WHERE `id` > '7' ORDER BY `id` LIMIT 100 OFFSET 0"

	if got != want {
		t.Errorf("SelectPage =\n%s\nwant\n%s", got, want)
	}
}

// The two engines that have no LIMIT cut a page the standard's way, and only one of them refuses
// to do it without an order — which is why a read with nothing to order by is given one that
// orders nothing rather than a column picked here.
func TestSelectPageCutsWithFetchWhereThereIsNoLimit(t *testing.T) {
	unkeyed := []driver.Column{{Name: "email"}}

	page := driver.TablePage{
		Namespace: driver.Namespace{Database: "shop", Schema: "dbo"},
		Table:     "log",
		Limit:     100,
		Offset:    200,
	}
	got := SQLServer.SelectPage(page, unkeyed, "")
	want := "SELECT [email] FROM [shop].[dbo].[log]" +
		" ORDER BY (SELECT NULL) OFFSET 200 ROWS FETCH NEXT 100 ROWS ONLY"
	if got != want {
		t.Errorf("SQLServer.SelectPage =\n%s\nwant\n%s", got, want)
	}

	oracle := driver.TablePage{
		Namespace: driver.Namespace{Database: "HR"},
		Table:     "LOG",
		Limit:     100,
		Offset:    200,
	}
	got = Oracle.SelectPage(oracle, unkeyed, "")
	want = `SELECT "email" FROM "HR"."LOG" OFFSET 200 ROWS FETCH NEXT 100 ROWS ONLY`
	if got != want {
		t.Errorf("Oracle.SelectPage =\n%s\nwant\n%s", got, want)
	}
}

func TestUpdateCellBindsValueThenKeysInOrder(t *testing.T) {
	edit := driver.CellEdit{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "order_items",
		Column:    "quantity",
		Value:     nil,
		Key:       map[string]string{"order_id": "7", "sku": "A-1"},
	}

	query, args := MySQL.UpdateCell(edit, []string{"order_id", "sku"})

	want := "UPDATE `shop`.`order_items` SET `quantity` = ? " +
		"WHERE `order_id` = ? AND `sku` = ? LIMIT 1"
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}

	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3", len(args))
	}
	if args[0] != (*string)(nil) {
		t.Errorf("args[0] = %v, want a nil *string so the cell is set to NULL", args[0])
	}
	if args[1] != "7" || args[2] != "A-1" {
		t.Error("key args must follow the primary-key column order, not map order")
	}
}

// A numbered placeholder has to count in the order the arguments are appended, or the value and
// the key it is pinned by change places.
func TestUpdateCellNumbersPlaceholdersWhereTheEngineDoes(t *testing.T) {
	edit := driver.CellEdit{
		Namespace: driver.Namespace{Schema: "public"},
		Table:     "users",
		Column:    "email",
		Key:       map[string]string{"id": "7"},
	}

	query, _ := Postgres.UpdateCell(edit, []string{"id"})
	want := `UPDATE "public"."users" SET "email" = $1 WHERE "id" = $2`
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}

	query, _ = Oracle.UpdateCell(edit, []string{"id"})
	want = `UPDATE ""."users" SET "email" = :1 WHERE "id" = :2`
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}
}

func TestInsertRowNamesOnlyFilledColumnsInTableOrder(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
		{Name: "created_at"},
	}
	value := "a@b.c"
	insert := driver.RowInsert{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "users",
		Values:    map[string]*string{"email": &value, "id": nil},
	}

	query, args := MySQL.InsertRow(insert, columns)

	want := "INSERT INTO `shop`.`users` (`id`, `email`) VALUES (?, ?)"
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}
	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2: created_at was never filled in", len(args))
	}
	if args[0] != (*string)(nil) || args[1] != &value {
		t.Error("insert args must follow the table's column order, not map order")
	}
}

// A row that filled nothing in is the one place the engines have three answers, and an engine with
// neither of the first two names every column rather than writing a statement it would refuse.
func TestInsertRowWithNothingFilledInTakesEveryDefault(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}, {Name: "email"}}
	insert := driver.RowInsert{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "users",
	}

	cases := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{"MySQL", MySQL, "INSERT INTO `shop`.`users` () VALUES ()"},
		{"SQLite", SQLite, `INSERT INTO "shop"."users" DEFAULT VALUES`},
		{"Oracle", Oracle, `INSERT INTO "shop"."users" ("id", "email") VALUES (DEFAULT, DEFAULT)`},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			query, args := one.dialect.InsertRow(insert, columns)
			if query != one.want {
				t.Errorf("query =\n%s\nwant\n%s", query, one.want)
			}
			if args != nil {
				t.Errorf("args = %v, want none", args)
			}
		})
	}
}

func TestDeleteRowPinsEveryKeyColumn(t *testing.T) {
	removal := driver.RowDelete{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "order_items",
		Key:       map[string]string{"order_id": "7", "sku": "A-1"},
	}

	query, args := MySQL.DeleteRow(removal, []string{"order_id", "sku"})

	want := "DELETE FROM `shop`.`order_items` WHERE `order_id` = ? AND `sku` = ? LIMIT 1"
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}
	if len(args) != 2 || args[0] != "7" || args[1] != "A-1" {
		t.Errorf("args = %v, want [7 A-1] in primary-key column order", args)
	}
}

// A name is quoted so that nothing in it can end the quoting early, and what ends it is the
// *closing* character — which on the one engine with a pair is not the one that opened it.
func TestQuoteIdentDoublesTheClosingQuote(t *testing.T) {
	if got, want := SQLServer.QuoteIdent("us]er"), "[us]]er]"; got != want {
		t.Errorf("SQLServer.QuoteIdent = %q, want %q", got, want)
	}
	if got, want := SQLServer.Table(driver.Namespace{Database: "shop", Schema: "dbo"}, "users"),
		"[shop].[dbo].[users]"; got != want {
		t.Errorf("SQLServer.Table = %q, want %q", got, want)
	}
}
