package postgres

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestBuildSelectOrdersByPrimaryKey(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
	}
	page := driver.TablePage{Database: "public", Table: "users", Limit: 100, Offset: 200}

	got, args := buildSelect(page, columns)
	want := `SELECT "id", "email" FROM "public"."users" ORDER BY "id" LIMIT $1 OFFSET $2`

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
	if len(args) != 2 || args[0] != 100 || args[1] != 200 {
		t.Errorf("args = %v, want [100 200]", args)
	}
}

func TestBuildSelectWithoutPrimaryKeyHasNoOrderBy(t *testing.T) {
	columns := []driver.Column{{Name: "email"}}
	page := driver.TablePage{Database: "public", Table: "log", Limit: 100}

	got, _ := buildSelect(page, columns)
	want := `SELECT "email" FROM "public"."log" LIMIT $1 OFFSET $2`

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildSelectWithoutLimitReadsEveryRow(t *testing.T) {
	columns := []driver.Column{{Name: "id", PrimaryKey: true}}
	page := driver.TablePage{Database: "public", Table: "users", Offset: 200}

	got, args := buildSelect(page, columns)
	want := `SELECT "id" FROM "public"."users" ORDER BY "id"`

	if got != want {
		t.Errorf("buildSelect =\n%s\nwant\n%s", got, want)
	}
	if args != nil {
		t.Errorf("args = %v, want none: an offset without a limit binds nothing", args)
	}
}

func TestBuildUpdateBindsValueThenKeysInOrder(t *testing.T) {
	edit := driver.CellEdit{
		Database: "public",
		Table:    "order_items",
		Column:   "quantity",
		Value:    nil,
		Key:      map[string]string{"order_id": "7", "sku": "A-1"},
	}

	query, args := buildUpdate(edit, []string{"order_id", "sku"})

	want := `UPDATE "public"."order_items" SET "quantity" = $1 ` +
		`WHERE "order_id" = $2 AND "sku" = $3`
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
