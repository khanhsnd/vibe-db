package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestQuoteIdentEscapesBackticks(t *testing.T) {
	if got, want := quoteIdent("us`er"), "`us``er`"; got != want {
		t.Errorf("quoteIdent = %q, want %q", got, want)
	}
}

func TestBuildSelectOrdersByPrimaryKey(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
	}

	got := buildSelect(driver.TablePage{Database: "shop", Table: "users"}, columns)
	want := "SELECT `id`, `email` FROM `shop`.`users` ORDER BY `id` LIMIT ? OFFSET ?"

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildSelectWithoutPrimaryKeyHasNoOrderBy(t *testing.T) {
	columns := []driver.Column{{Name: "email"}}

	got := buildSelect(driver.TablePage{Database: "shop", Table: "log"}, columns)
	want := "SELECT `email` FROM `shop`.`log` LIMIT ? OFFSET ?"

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildUpdateBindsValueThenKeysInOrder(t *testing.T) {
	edit := driver.CellEdit{
		Database: "shop",
		Table:    "order_items",
		Column:   "quantity",
		Value:    nil,
		Key:      map[string]string{"order_id": "7", "sku": "A-1"},
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

func TestPrimaryKeyNamesKeepsOrdinalOrder(t *testing.T) {
	columns := []driver.Column{
		{Name: "order_id", PrimaryKey: true},
		{Name: "quantity"},
		{Name: "sku", PrimaryKey: true},
	}

	got := primaryKeyNames(columns)

	if len(got) != 2 || got[0] != "order_id" || got[1] != "sku" {
		t.Errorf("primaryKeyNames = %v, want [order_id sku]", got)
	}
}
