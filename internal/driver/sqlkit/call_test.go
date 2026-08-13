package sqlkit

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func value(text string) *string { return &text }

func inArg(name, kind, text string) driver.RoutineArg {
	return driver.RoutineArg{
		RoutineParam: driver.RoutineParam{Name: name, Mode: driver.ParamIn, Type: kind},
		Value:        value(text),
	}
}

func outArg(name, kind string) driver.RoutineArg {
	return driver.RoutineArg{
		RoutineParam: driver.RoutineParam{Name: name, Mode: driver.ParamOut, Type: kind},
	}
}

func procedure(name string, args ...driver.RoutineArg) driver.RoutineCall {
	return driver.RoutineCall{
		SourceRef: driver.SourceRef{Kind: driver.SourceProcedure, Name: name},
		Args:      args,
	}
}

func function(name string, args ...driver.RoutineArg) driver.RoutineCall {
	return driver.RoutineCall{
		SourceRef: driver.SourceRef{Kind: driver.SourceFunction, Name: name},
		Args:      args,
	}
}

// A procedure that takes its arguments and hands nothing back is one statement on four engines, and
// the differences between them are the whole of what CallStyle is for: the verb, the block around
// it, and how the routine is named.
func TestCallProcedureIsSpelledTheWayEachEngineRunsOne(t *testing.T) {
	call := procedure("sweep", inArg("days", "int", "30"))

	cases := []struct {
		engine  string
		dialect Dialect
		ns      driver.Namespace
		want    string
	}{
		{"MySQL", MySQL, driver.Namespace{Database: "shop"},
			"CALL `shop`.`sweep`('30')"},
		{"Postgres", Postgres, driver.Namespace{Database: "shop", Schema: "public"},
			`CALL "public"."sweep"('30')`},
		{"SQLServer", SQLServer, driver.Namespace{Database: "shop", Schema: "dbo"},
			"EXEC [shop].[dbo].[sweep] '30'"},
		{"Oracle", Oracle, driver.Namespace{Database: "SHOP"},
			`BEGIN "SHOP"."sweep"('30'); END;`},
	}

	for _, one := range cases {
		spelled := one.dialect.Call(one.ns, call)
		if len(spelled.Statements) != 1 || spelled.Statements[0] != one.want {
			t.Errorf("%s Call = %v, want [%s]", one.engine, spelled.Statements, one.want)
		}
	}
}

// A function answers with a value, so it is *read*. PostgreSQL reads one in the FROM clause — which
// is what lets a function with OUT parameters answer as columns — and Oracle needs a table to read
// from at all.
func TestCallFunctionIsRead(t *testing.T) {
	call := function("tax", inArg("amount", "numeric", "10.50"))

	cases := []struct {
		engine  string
		dialect Dialect
		ns      driver.Namespace
		want    string
	}{
		{"MySQL", MySQL, driver.Namespace{Database: "shop"},
			"SELECT `shop`.`tax`('10.50')"},
		{"Postgres", Postgres, driver.Namespace{Database: "shop", Schema: "public"},
			`SELECT * FROM "public"."tax"('10.50')`},
		{"SQLServer", SQLServer, driver.Namespace{Database: "shop", Schema: "dbo"},
			"SELECT [shop].[dbo].[tax]('10.50')"},
		{"Oracle", Oracle, driver.Namespace{Database: "SHOP"},
			`SELECT "SHOP"."tax"('10.50') FROM dual`},
	}

	for _, one := range cases {
		spelled := one.dialect.Call(one.ns, call)
		if len(spelled.Statements) != 1 || spelled.Statements[0] != one.want {
			t.Errorf("%s Call = %v, want [%s]", one.engine, spelled.Statements, one.want)
		}
		if !spelled.Reads {
			t.Errorf("%s Call does not read, and a function answers with a value", one.engine)
		}
	}
}

// A value that is not there is the keyword and not the word, exactly as a row on its way into an
// INSERT is.
func TestCallWritesNullAsTheKeyword(t *testing.T) {
	call := procedure("sweep", driver.RoutineArg{
		RoutineParam: driver.RoutineParam{Name: "days", Mode: driver.ParamIn, Type: "int"},
	})

	got := Postgres.Call(driver.Namespace{Schema: "public"}, call).Statements
	want := `CALL "public"."sweep"(NULL)`

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// A value the reader typed reaches the statement as a string literal and never as SQL, which is the
// same bargain a filter's value makes — and the quoting is the dialect's, so a quote inside one is
// doubled rather than ending it.
func TestCallQuotesTheValueItWasGiven(t *testing.T) {
	call := procedure("find", inArg("name", "text", "O'Hara"))

	got := Postgres.Call(driver.Namespace{Schema: "public"}, call).Statements
	want := `CALL "public"."find"('O''Hara')`

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// MySQL hands an OUT parameter back through a **user variable**, so the call is three statements:
// the variable is set so that it exists whatever the routine does, passed, and read back under the
// parameter's own name.
func TestCallThroughVariablesSetsPassesAndReads(t *testing.T) {
	call := procedure("total", inArg("year", "int", "2026"), outArg("amount", "decimal(10,2)"))

	got := MySQL.Call(driver.Namespace{Database: "shop"}, call).Statements
	want := []string{
		"SET @vibe_arg_2 = NULL",
		"CALL `shop`.`total`('2026', @vibe_arg_2)",
		"SELECT @vibe_arg_2 AS `amount`",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Call = %v, want %v", got, want)
	}
}

// SQL Server's variable belongs to the **batch**, so the declaration, the call and the read are one
// statement — a second batch would not know the name. The type is the catalogue's own and is what
// the variable is declared as.
func TestCallInBatchIsOneStatement(t *testing.T) {
	call := procedure("total", inArg("year", "int", "2026"), outArg("amount", "decimal(10,2)"))

	got := SQLServer.Call(driver.Namespace{Database: "shop", Schema: "dbo"}, call).Statements
	want := "DECLARE @vibe_arg_2 decimal(10,2);\n" +
		"EXEC [shop].[dbo].[total] '2026', @vibe_arg_2 OUTPUT;\n" +
		"SELECT @vibe_arg_2 AS [amount]"

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// An INOUT parameter is both, so it is set from what the reader typed before the call and read back
// after it.
func TestCallInBatchSetsAnInOutParameterFirst(t *testing.T) {
	call := procedure("bump", driver.RoutineArg{
		RoutineParam: driver.RoutineParam{Name: "n", Mode: driver.ParamInOut, Type: "int"},
		Value:        value("1"),
	})

	got := SQLServer.Call(driver.Namespace{Database: "shop", Schema: "dbo"}, call).Statements
	want := "DECLARE @vibe_arg_1 int;\n" +
		"SET @vibe_arg_1 = '1';\n" +
		"EXEC [shop].[dbo].[bump] @vibe_arg_1 OUTPUT;\n" +
		"SELECT @vibe_arg_1 AS [n]"

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// PostgreSQL takes an OUT parameter's place in the argument list and answers with a row, so nothing
// is set beforehand and nothing is read afterwards.
func TestCallDirectPassesAnOutParameterAsNull(t *testing.T) {
	call := procedure("total", inArg("year", "int", "2026"), outArg("amount", "numeric"))

	got := Postgres.Call(driver.Namespace{Schema: "public"}, call).Statements
	want := `CALL "public"."total"('2026', NULL)`

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// A function's OUT parameter is not passed at all where the engine answers with it as a column.
func TestCallDirectLeavesAFunctionsOutParameterOutOfTheList(t *testing.T) {
	call := function("split", inArg("text", "text", "a,b"), outArg("head", "text"))

	got := Postgres.Call(driver.Namespace{Schema: "public"}, call).Statements
	want := `SELECT * FROM "public"."split"('a,b')`

	if len(got) != 1 || got[0] != want {
		t.Errorf("Call = %v, want [%s]", got, want)
	}
}

// Oracle's block answers with nothing, so a parameter it would hand back has nowhere to go: the
// call spells **nothing at all**, which is what every engine here does for a thing it has not got.
func TestCallInBlockSpellsNothingForAnOutParameter(t *testing.T) {
	call := procedure("total", inArg("year", "int", "2026"), outArg("amount", "NUMBER"))

	if got := Oracle.Call(driver.Namespace{Database: "SHOP"}, call); len(got.Statements) != 0 {
		t.Errorf("Call = %v, want nothing spelled", got.Statements)
	}
}

// An engine with no stored routine spells nothing whatever it is asked, which is what the run of it
// answers `domain.Unsupported` for.
func TestCallSpellsNothingWhereThereAreNoRoutines(t *testing.T) {
	call := procedure("sweep", inArg("days", "int", "30"))

	if got := SQLite.Call(driver.Namespace{Database: "main"}, call); len(got.Statements) != 0 {
		t.Errorf("Call = %v, want nothing spelled", got.Statements)
	}
}

// A block hands back nothing, so nothing reads: the run reports what it changed instead of rows.
func TestCallInBlockDoesNotRead(t *testing.T) {
	call := procedure("sweep", inArg("days", "int", "30"))

	if Oracle.Call(driver.Namespace{Database: "SHOP"}, call).Reads {
		t.Error("Call reads, and an Oracle block answers with no rows")
	}
}

// Each engine's own word for what a routine does with a parameter reads back as the one mode, and a
// word none of them uses is a value taken — which is what a parameter with nothing said about it is.
func TestParamModeOfReadsEveryEnginesWord(t *testing.T) {
	cases := map[string]driver.ParamMode{
		"IN":     driver.ParamIn,
		"i":      driver.ParamIn,
		"v":      driver.ParamIn,
		"OUT":    driver.ParamOut,
		"o":      driver.ParamOut,
		"t":      driver.ParamOut,
		"INOUT":  driver.ParamInOut,
		"IN/OUT": driver.ParamInOut,
		"b":      driver.ParamInOut,
		"":       driver.ParamIn,
	}

	for word, want := range cases {
		if got := ParamModeOf(word); got != want {
			t.Errorf("ParamModeOf(%q) = %s, want %s", word, got, want)
		}
	}
}
