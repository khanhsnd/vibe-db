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

// A trigger is dropped five ways, and the two that differ do so for different reasons. PostgreSQL
// finds one by its *table*, a trigger name there being unique per table rather than per schema, so
// its own name carries no namespace at all. SQL Server takes two parts where every other statement
// it is sent takes three. The other three drop it by its qualified name like any other object.
func TestDropTriggerIsAddressedTheWayEachEngineAddressesOne(t *testing.T) {
	ref := driver.SourceRef{Kind: driver.SourceTrigger, Name: "audit", Table: "users"}

	cases := []struct {
		engine  string
		dialect Dialect
		ns      driver.Namespace
		want    string
	}{
		{"MySQL", MySQL, driver.Namespace{Database: "shop"},
			"DROP TRIGGER `shop`.`audit`"},
		{"Postgres", Postgres, driver.Namespace{Database: "shop", Schema: "public"},
			`DROP TRIGGER "audit" ON "public"."users"`},
		{"SQLite", SQLite, driver.Namespace{Database: "main"},
			`DROP TRIGGER "main"."audit"`},
		{"SQLServer", SQLServer, driver.Namespace{Database: "shop", Schema: "dbo"},
			"DROP TRIGGER [dbo].[audit]"},
		{"Oracle", Oracle, driver.Namespace{Database: "HR"},
			`DROP TRIGGER "HR"."audit"`},
	}

	for _, one := range cases {
		if got := one.dialect.DropSource(one.ns, ref); got != one.want {
			t.Errorf("%s: DropSource = %s, want %s", one.engine, got, one.want)
		}
	}
}

// A new trigger starts from the engine's own grammar, and the skeleton addresses **two** things:
// the trigger, and the table it fires on. The two engines whose trigger name is not the name every
// other statement writes spell it their way here as well, which is the whole point of one speller.
func TestNewTriggerAddressesBothItselfAndItsTable(t *testing.T) {
	ref := driver.SourceRef{Kind: driver.SourceTrigger, Name: "audit", Table: "users"}

	cases := []struct {
		engine  string
		dialect Dialect
		ns      driver.Namespace
		want    string
	}{
		{"MySQL", MySQL, driver.Namespace{Database: "shop"},
			"CREATE TRIGGER `shop`.`audit`\nBEFORE INSERT ON `shop`.`users`"},
		{"Postgres", Postgres, driver.Namespace{Database: "shop", Schema: "public"},
			"CREATE TRIGGER \"audit\"\nBEFORE INSERT ON \"public\".\"users\""},
		// SQLite puts the schema on the trigger and refuses one on the table, exactly as it does
		// for an index.
		{"SQLite", SQLite, driver.Namespace{Database: "main"},
			"CREATE TRIGGER \"main\".\"audit\"\nAFTER INSERT ON \"users\""},
		{"SQLServer", SQLServer, driver.Namespace{Database: "shop", Schema: "dbo"},
			"CREATE TRIGGER [dbo].[audit]\nON [dbo].[users]"},
		{"Oracle", Oracle, driver.Namespace{Database: "HR"},
			"CREATE OR REPLACE TRIGGER \"HR\".\"audit\"\nBEFORE INSERT ON \"HR\".\"users\""},
	}

	for _, one := range cases {
		got := one.dialect.NewSource(one.ns, ref)
		if !strings.HasPrefix(got, one.want) {
			t.Errorf("%s: NewSource = %q, want it to open with %q", one.engine, got, one.want)
		}
	}
}

// An engine with no such object answers with nothing, which is what the frontend reads as there
// being no starter — SQLite has no stored routine to begin one of.
func TestNewSourceIsEmptyForAKindTheEngineHasNot(t *testing.T) {
	ns := driver.Namespace{Database: "main"}
	ref := driver.SourceRef{Kind: driver.SourceFunction, Name: "tax"}

	if got := SQLite.NewSource(ns, ref); got != "" {
		t.Errorf("NewSource = %q, want nothing", got)
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

func sameStatements(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00")
}
