package postgres

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A `serial` is an integer, a sequence and a default reading from it, so a description that wrote
// the type and the default out as it read them would name a sequence the dump never created.
func TestDeclaredTurnsASequenceDefaultIntoItsShorthand(t *testing.T) {
	nextval := "nextval('users_id_seq'::regclass)"
	literal := "0"
	columns := []driver.Column{
		{Name: "id", FullType: "integer", Default: &nextval, PrimaryKey: true},
		{Name: "big", FullType: "bigint", Default: &nextval},
		{Name: "small", FullType: "smallint", Default: &nextval},
		// An integer with a default of its own keeps both: only a sequence has to be declared.
		{Name: "count", FullType: "integer", Default: &literal},
		// And a type with no shorthand keeps the sequence default, there being nothing to say instead.
		{Name: "when", FullType: "timestamptz", Default: &nextval},
	}

	got := declared(columns)

	for at, want := range []string{"serial", "bigserial", "smallserial"} {
		if got[at].FullType != want {
			t.Errorf("%s type = %s, want %s", got[at].Name, got[at].FullType, want)
		}
		if got[at].Default != nil {
			t.Errorf("%s kept a default, which the shorthand already carries", got[at].Name)
		}
	}
	if got[3].FullType != "integer" || got[3].Default != &literal {
		t.Errorf("count = %s / %v, want integer with its own default", got[3].FullType, got[3].Default)
	}
	if got[4].FullType != "timestamptz" || got[4].Default != &nextval {
		t.Errorf("when = %s, want it left alone", got[4].FullType)
	}
}

func TestIndexStatementPutsTheMethodAfterTheTable(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}
	spec := driver.IndexSpec{
		Name:    "idx_orders_total",
		Columns: []string{"total"},
		Unique:  true,
		Method:  "btree",
	}

	got := indexStatement(ns, "orders", spec)
	want := `CREATE UNIQUE INDEX "idx_orders_total" ON "public"."orders" USING btree ("total")`

	if got != want {
		t.Errorf("indexStatement =\n%s\nwant\n%s", got, want)
	}
}
