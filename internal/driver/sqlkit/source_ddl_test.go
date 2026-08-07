package sqlkit

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestDropSourceNamesTheKind(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	cases := []struct {
		ref  driver.SourceRef
		want string
	}{
		{driver.SourceRef{Kind: driver.SourceView, Name: "recent"}, `DROP VIEW "public"."recent"`},
		{driver.SourceRef{Kind: driver.SourceFunction, Name: "tax"}, `DROP FUNCTION "public"."tax"`},
		{driver.SourceRef{Kind: driver.SourceProcedure, Name: "sweep"}, `DROP PROCEDURE "public"."sweep"`},
	}

	for _, one := range cases {
		if got := Postgres.DropSource(ns, one.ref); got != one.want {
			t.Errorf("DropSource(%s) = %s, want %s", one.ref.Kind, got, one.want)
		}
	}
}

// The arguments are what tell two PostgreSQL functions of one name apart, so a drop that has them
// carries them — and one that has not is the same statement without.
func TestDropSourceCarriesTheArgumentsWhenThereAreAny(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}
	ref := driver.SourceRef{Kind: driver.SourceFunction, Name: "tax", Arguments: "numeric, text"}

	got := Postgres.DropSource(ns, ref)
	want := `DROP FUNCTION "public"."tax"(numeric, text)`

	if got != want {
		t.Errorf("DropSource = %s, want %s", got, want)
	}
}

// A kind that arrived as something else is a view rather than text on its way into a statement.
// What refuses one outright is the API layer; this is the second lock.
func TestSourceKeywordIsTotal(t *testing.T) {
	if got := driver.SourceKind("DROP DATABASE shop; --").Keyword(); got != "VIEW" {
		t.Errorf("Keyword = %s, want VIEW", got)
	}
}

func TestSourceStatementsDeclaresWithoutADrop(t *testing.T) {
	ns := driver.Namespace{Database: "shop"}
	ddl := DDL{Dialect: MySQL}

	got := ddl.SourceStatements(ns, driver.SourceSave{
		Definition: "  CREATE VIEW `shop`.`recent` AS SELECT 1  ",
	})
	want := []string{"CREATE VIEW `shop`.`recent` AS SELECT 1"}

	if !sameStatements(got, want) {
		t.Errorf("SourceStatements = %#v, want %#v", got, want)
	}
}

// Replacing is a drop and then the reader's own text, in that order — and the text is theirs
// verbatim, semicolons inside a routine body included: it is one statement however many it holds.
func TestSourceStatementsDropsWhatIsThereFirst(t *testing.T) {
	ns := driver.Namespace{Database: "shop"}
	ddl := DDL{Dialect: MySQL}

	body := "CREATE PROCEDURE `shop`.`sweep`()\nBEGIN\n  DELETE FROM t;\n  DELETE FROM u;\nEND"
	got := ddl.SourceStatements(ns, driver.SourceSave{
		Replaces:   driver.SourceRef{Kind: driver.SourceProcedure, Name: "sweep"},
		Definition: body,
	})
	want := []string{"DROP PROCEDURE `shop`.`sweep`", body}

	if !sameStatements(got, want) {
		t.Errorf("SourceStatements = %#v, want %#v", got, want)
	}
}

// A blank definition spells nothing at all rather than the drop on its own, which is the one thing
// a Save must never come to mean.
func TestSourceStatementsSpellsNothingForABlankDefinition(t *testing.T) {
	ns := driver.Namespace{Database: "shop"}
	ddl := DDL{Dialect: MySQL}

	got := ddl.SourceStatements(ns, driver.SourceSave{
		Replaces:   driver.SourceRef{Kind: driver.SourceView, Name: "recent"},
		Definition: "   \n  ",
	})

	if len(got) != 0 {
		t.Errorf("SourceStatements = %#v, want nothing", got)
	}
}

// A routine reaches a drop as the reference the tree already held for it, so what the menu drops
// and what a save drops in front of its create are one spelling.
func TestRoutineSourceIsItsOwnReference(t *testing.T) {
	routine := driver.Routine{Name: "tax", Procedure: false, Arguments: "numeric"}

	got := routine.Source()
	if got.Kind != driver.SourceFunction || got.Name != "tax" || got.Arguments != "numeric" {
		t.Errorf("Source = %#v", got)
	}
	procedure := driver.Routine{Name: "sweep", Procedure: true}
	if procedure.Source().Kind != driver.SourceProcedure {
		t.Error("a procedure's reference is not a procedure")
	}
}

func sameStatements(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00")
}
