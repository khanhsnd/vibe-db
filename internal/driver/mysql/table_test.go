package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestBuildSelectOrdersByPrimaryKey(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
	}
	page := driver.TablePage{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "users",
		Limit:     100,
		Offset:    200,
	}

	got, args := buildSelect(page, columns)
	want := "SELECT `id`, `email` FROM `shop`.`users` ORDER BY `id` LIMIT ? OFFSET ?"

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
	if len(args) != 2 || args[0] != 100 || args[1] != 200 {
		t.Errorf("args = %v, want [100 200]", args)
	}
}

func TestBuildSelectWithoutPrimaryKeyHasNoOrderBy(t *testing.T) {
	columns := []driver.Column{{Name: "email"}}
	page := driver.TablePage{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "log",
		Limit:     100,
	}

	got, _ := buildSelect(page, columns)
	want := "SELECT `email` FROM `shop`.`log` LIMIT ? OFFSET ?"

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildSelectWithoutLimitReadsEveryRow(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}}
	page := driver.TablePage{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "users",
		Offset:    200,
	}

	got, args := buildSelect(page, columns)
	want := "SELECT `id` FROM `shop`.`users` ORDER BY `id`"

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
	if args != nil {
		t.Errorf("args = %v, want none: an offset without a limit binds nothing", args)
	}
}

func TestBuildUpdateBindsValueThenKeysInOrder(t *testing.T) {
	edit := driver.CellEdit{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "order_items",
		Column:    "quantity",
		Value:     nil,
		Key:       map[string]string{"order_id": "7", "sku": "A-1"},
	}

	query, args := buildUpdate(edit, []string{"order_id", "sku"})

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

func TestBuildInsertNamesOnlyFilledColumnsInTableOrder(t *testing.T) {
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

	query, args := buildInsert(insert, columns)

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

func TestBuildInsertWithNothingFilledInTakesEveryDefault(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}}
	insert := driver.RowInsert{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "users",
	}

	query, args := buildInsert(insert, columns)

	want := "INSERT INTO `shop`.`users` () VALUES ()"
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}
	if args != nil {
		t.Errorf("args = %v, want none", args)
	}
}

func TestBuildDeletePinsEveryKeyColumn(t *testing.T) {
	removal := driver.RowDelete{
		Namespace: driver.Namespace{Database: "shop"},
		Table:     "order_items",
		Key:       map[string]string{"order_id": "7", "sku": "A-1"},
	}

	query, args := buildDelete(removal, []string{"order_id", "sku"})

	want := "DELETE FROM `shop`.`order_items` WHERE `order_id` = ? AND `sku` = ? LIMIT 1"
	if query != want {
		t.Errorf("query =\n%s\nwant\n%s", query, want)
	}
	if len(args) != 2 || args[0] != "7" || args[1] != "A-1" {
		t.Errorf("args = %v, want [7 A-1] in primary-key column order", args)
	}
}
