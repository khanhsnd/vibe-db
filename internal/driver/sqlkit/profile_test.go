package sqlkit

import (
	"context"
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The profiling half of the five grammars, checked where every difference between them lives: which
// changes an engine has a statement for at all, and what unit its threshold is kept in.

func recording(milliseconds int) driver.ProfileChange {
	return driver.ProfileChange{Action: domain.ProfileEnable, ThresholdMs: milliseconds}
}

var (
	stopping  = driver.ProfileChange{Action: domain.ProfileDisable}
	clearing  = driver.ProfileChange{Action: domain.ProfileReset}
	noContext = driver.Namespace{}
)

// MySQL is the one engine with all four, and the one whose threshold is in **seconds** — so a reader
// asking for 200ms gets a decimal rather than the number they typed. `log_output` leads because the
// switch fills nothing without it.
func TestProfileStatementsOnMySQL(t *testing.T) {
	changes := []driver.ProfileChange{recording(200), stopping, clearing}

	want := []string{
		"SET GLOBAL log_output = 'TABLE'",
		"SET GLOBAL slow_query_log = 'ON'",
		"SET GLOBAL long_query_time = 0.2",
		"SET GLOBAL slow_query_log = 'OFF'",
		"TRUNCATE TABLE mysql.slow_log",
	}
	assertStatements(t, "MySQL", Profile{Dialect: MySQL}.ProfileStatements(changes), want)
}

// A threshold that divides into whole seconds is written as one: `long_query_time = 2` reads as the
// setting a reader would have typed, and `2.0` would only invite the question of what the decimal is
// carrying.
func TestWholeSecondsAreNotWrittenAsDecimals(t *testing.T) {
	want := []string{
		"SET GLOBAL log_output = 'TABLE'",
		"SET GLOBAL slow_query_log = 'ON'",
		"SET GLOBAL long_query_time = 2",
	}
	assertStatements(t, "MySQL", Profile{Dialect: MySQL}.ProfileStatements(
		[]driver.ProfileChange{recording(2000)}), want)
}

// PostgreSQL's recording is an extension rather than a setting, and it has no threshold: the switch
// spells one statement and the number the reader typed spells **nothing**, rather than a `SET` that
// would change something else entirely.
func TestProfileStatementsOnPostgres(t *testing.T) {
	changes := []driver.ProfileChange{recording(500), stopping, clearing}

	want := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"DROP EXTENSION IF EXISTS pg_stat_statements",
		"SELECT pg_stat_statements_reset()",
	}
	assertStatements(t, "PostgreSQL", Profile{Dialect: Postgres}.ProfileStatements(changes), want)
}

// SQL Server records whether or not anybody asked, so turning it on or off spells nothing at all —
// which is what an empty preview means everywhere in this port, and what the run turns into a
// refusal. What it does have is the reset.
func TestProfileStatementsOnSQLServer(t *testing.T) {
	changes := []driver.ProfileChange{recording(100), stopping, clearing}

	assertStatements(t, "SQL Server",
		Profile{Dialect: SQLServer}.ProfileStatements(changes),
		[]string{"DBCC FREEPROCCACHE"})
}

// Oracle has none of the four. Its shared pool is not a log a reader empties from a button, so this
// engine spells nothing rather than offering a flush of the whole instance.
func TestProfileStatementsOnOracle(t *testing.T) {
	changes := []driver.ProfileChange{recording(100), stopping, clearing}

	assertStatements(t, "Oracle", Profile{Dialect: Oracle}.ProfileStatements(changes), nil)
}

// A change an engine has no statement for is refused by the **run**, naming which — the preview
// having already said so by spelling nothing.
func TestApplyRefusesWhatTheEngineHasNot(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		change  driver.ProfileChange
	}{
		{"turning Oracle's recording on", Oracle, recording(100)},
		{"turning SQL Server's recording off", SQLServer, stopping},
		{"clearing Oracle's", Oracle, clearing},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			profile := Profile{
				Dialect: one.dialect,
				Run: func(context.Context, string, string) error {
					t.Error("a change the engine has not got reached the server")
					return nil
				},
			}

			err := profile.ApplyProfileChanges(
				context.Background(), noContext, []driver.ProfileChange{one.change})

			var unsupported domain.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("ApplyProfileChanges = %v, want an unsupported error", err)
			}
		})
	}
}

// A list holding one change this engine cannot make is refused **whole**, so the reset in front of
// it does not empty a log on the way to a failure.
func TestApplyRefusesTheWholeListBeforeRunningAny(t *testing.T) {
	profile := Profile{
		Dialect: SQLServer,
		Run: func(context.Context, string, string) error {
			t.Error("a statement ran although a later change was unsupported")
			return nil
		},
	}

	err := profile.ApplyProfileChanges(
		context.Background(), noContext, []driver.ProfileChange{clearing, stopping})

	var unsupported domain.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("ApplyProfileChanges = %v, want an unsupported error", err)
	}
}

// The threshold is checked as a number before it reaches a statement, for the reason a process id is:
// a `SET GLOBAL` binds no arguments, so nothing else stands between the value and the server.
func TestANegativeThresholdIsRefused(t *testing.T) {
	profile := Profile{
		Dialect: MySQL,
		Run: func(context.Context, string, string) error {
			t.Error("a negative threshold reached the server")
			return nil
		},
	}

	err := profile.ApplyProfileChanges(
		context.Background(), noContext, []driver.ProfileChange{recording(-1)})

	var invalid domain.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("ApplyProfileChanges = %v, want a validation error", err)
	}
}

// A threshold of zero moves nothing. It is what a config carries on an engine with no such setting,
// so writing it would let a dialog that never drew the box set `long_query_time = 0` — which on
// MySQL means "record every statement there is".
func TestAZeroThresholdMovesNothing(t *testing.T) {
	want := []string{
		"SET GLOBAL log_output = 'TABLE'",
		"SET GLOBAL slow_query_log = 'ON'",
	}
	assertStatements(t, "MySQL", Profile{Dialect: MySQL}.ProfileStatements(
		[]driver.ProfileChange{recording(0)}), want)
}

// What a run sends is what the preview showed, in that order — rule 13 where there is no Changes
// pane to hold it.
func TestApplyRunsExactlyWhatWasPreviewed(t *testing.T) {
	sent := []string{}
	profile := Profile{
		Dialect: MySQL,
		Run: func(_ context.Context, _ string, statement string) error {
			sent = append(sent, statement)
			return nil
		},
	}

	changes := []driver.ProfileChange{recording(1500), clearing}
	shown := profile.ProfileStatements(changes)

	if err := profile.ApplyProfileChanges(context.Background(), noContext, changes); err != nil {
		t.Fatalf("ApplyProfileChanges: %v", err)
	}
	assertStatements(t, "MySQL", sent, shown)
}

// PostgreSQL's extension installs into the database it is run in, so the namespace has to reach the
// send — a reader who turned the recording on while reading `shop` has not turned it on in `admin`.
func TestTheDatabaseReachesTheSend(t *testing.T) {
	seen := ""
	profile := Profile{
		Dialect: Postgres,
		Run: func(_ context.Context, database, _ string) error {
			seen = database
			return nil
		},
	}

	err := profile.ApplyProfileChanges(
		context.Background(),
		driver.Namespace{Database: "shop"},
		[]driver.ProfileChange{recording(0)},
	)
	if err != nil {
		t.Fatalf("ApplyProfileChanges: %v", err)
	}
	if seen != "shop" {
		t.Errorf("statement ran against %q, want shop", seen)
	}
}

// What a reader copies out of a slow list is a statement that has to **run somewhere else**, so the
// four engines' four ways of entering a database are checked here — that being the whole of what
// stands between a recorded fragment and a statement a colleague can paste.

func TestRunnableEntersTheDatabase(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{"MySQL switches with a statement", MySQL, "USE `shop`;\nSELECT 1;"},
		{"SQL Server needs its batch separator", SQLServer, "USE [shop];\nGO\nSELECT 1;"},
		{
			"Oracle enters a schema, having no database below its service",
			Oracle,
			"ALTER SESSION SET CURRENT_SCHEMA = \"shop\";\nSELECT 1;",
		},
		{
			"PostgreSQL has no such statement and says which database instead",
			Postgres,
			"-- Connect to the database \"shop\" to run this.\nSELECT 1;",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := one.dialect.Runnable("shop", "SELECT 1", true)
			if got != one.want {
				t.Errorf("Runnable =\n%s\nwant\n%s", got, one.want)
			}
		})
	}
}

// A statement the engine normalized carries a note saying so, because a reader who pastes `$1` into
// a terminal and is refused has been told nothing by the refusal.
func TestRunnableSaysWhenItWillNotRunAsItStands(t *testing.T) {
	got := Postgres.Runnable("", "SELECT * FROM t WHERE id = $1", false)

	want := "-- This server records the statement with its values already replaced.\n" +
		"-- Fill the placeholders in before running it.\n" +
		"SELECT * FROM t WHERE id = $1;"
	if got != want {
		t.Errorf("Runnable =\n%s\nwant\n%s", got, want)
	}
}

// A statement that already ends in one does not collect a second, and a row the server recorded no
// statement for answers with nothing rather than with a `USE` and a blank line after it.
func TestRunnableEdges(t *testing.T) {
	if got := MySQL.Runnable("", "SELECT 1;", true); got != "SELECT 1;" {
		t.Errorf("Runnable = %q, want a single semicolon", got)
	}
	if got := MySQL.Runnable("shop", "   ", true); got != "" {
		t.Errorf("Runnable = %q, want nothing", got)
	}
}

// The statement and the database are read out of the list by the **names** of their columns, so a
// column added in front of either does not silently start being copied instead.
func TestSlowQueryRefsReadByName(t *testing.T) {
	columns := []driver.Column{{Name: "Started"}, {Name: "Database"}, {Name: "Statement"}}
	text, where, when := "SELECT 1", "shop", "0.5"
	rows := [][]*string{{&when, &where, &text}}

	refs := SlowQueryRefs(columns, rows, MySQL, "Statement", "Database", true)

	if len(refs) != 1 {
		t.Fatalf("SlowQueryRefs = %+v, want one", refs)
	}
	if refs[0].Runnable != "USE `shop`;\nSELECT 1;" {
		t.Errorf("Runnable = %q", refs[0].Runnable)
	}
	if !refs[0].Complete {
		t.Error("a ref lost the mark the adapter put on it")
	}
}

// A column that is not there reads nothing rather than whichever one came first.
func TestSlowQueryRefsWithNoSuchColumn(t *testing.T) {
	columns := []driver.Column{{Name: "Started"}}
	text := "SELECT 1"
	rows := [][]*string{{&text}}

	refs := SlowQueryRefs(columns, rows, MySQL, "Statement", "", true)

	if len(refs) != 1 || refs[0].Runnable != "" {
		t.Fatalf("SlowQueryRefs = %+v, want one holding nothing", refs)
	}
}
