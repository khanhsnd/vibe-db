package sqlkit

import (
	"context"
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The monitoring half of the five grammars, checked where every difference between them lives: which
// of the two kills an engine has a statement for, and how a process is named.

func cancel(id string) driver.ProcessKill {
	return driver.ProcessKill{
		Process: driver.ProcessRef{ID: id},
		Action:  domain.ProcessCancel,
	}
}

func closing(ref driver.ProcessRef) driver.ProcessKill {
	return driver.ProcessKill{Process: ref, Action: domain.ProcessClose}
}

// MySQL is the one engine that says both halves in one verb, so both are spelled and neither is a
// bare `KILL 5` — which would mean the connection whichever was meant.
func TestKillStatementsOnMySQL(t *testing.T) {
	kills := []driver.ProcessKill{cancel("42"), closing(driver.ProcessRef{ID: "42"})}

	want := []string{"KILL QUERY 42", "KILL CONNECTION 42"}
	assertStatements(t, "MySQL", Watch{Dialect: MySQL}.ProcessStatements(kills), want)
}

// PostgreSQL spells both as **functions**, which is why the statement is a SELECT and still comes
// out of one flag.
func TestKillStatementsOnPostgres(t *testing.T) {
	kills := []driver.ProcessKill{cancel("713"), closing(driver.ProcessRef{ID: "713"})}

	want := []string{"SELECT pg_cancel_backend(713)", "SELECT pg_terminate_backend(713)"}
	assertStatements(t, "PostgreSQL", Watch{Dialect: Postgres}.ProcessStatements(kills), want)
}

// SQL Server has `KILL` and nothing narrower, so a cancel spells **nothing** — which is what an empty
// preview means everywhere in this port, and what the run turns into a refusal.
func TestKillStatementsOnSQLServer(t *testing.T) {
	kills := []driver.ProcessKill{cancel("55"), closing(driver.ProcessRef{ID: "55"})}

	assertStatements(t, "SQL Server", Watch{Dialect: SQLServer}.ProcessStatements(kills), []string{
		"KILL 55",
	})
}

// Oracle's session **is** the pair, in one string literal: a sid alone would end whoever took it
// next.
func TestKillStatementsOnOracle(t *testing.T) {
	kills := []driver.ProcessKill{
		closing(driver.ProcessRef{ID: "15", Serial: "3421"}),
		cancel("15"),
	}

	assertStatements(t, "Oracle", Watch{Dialect: Oracle}.ProcessStatements(kills), []string{
		"ALTER SYSTEM KILL SESSION '15,3421' IMMEDIATE",
	})
}

// A reference that is not numbers spells nothing at all. It is the one place a value off the wire
// would otherwise be pasted into a statement, so this is about the quoting and not about tidiness.
func TestKillSpellsNothingForSomethingThatIsNotAProcess(t *testing.T) {
	cases := []struct {
		engine  string
		dialect Dialect
		process driver.ProcessRef
	}{
		{"MySQL", MySQL, driver.ProcessRef{ID: "1; DROP TABLE users"}},
		{"PostgreSQL", Postgres, driver.ProcessRef{ID: "713); DROP TABLE users --"}},
		{"SQL Server", SQLServer, driver.ProcessRef{ID: "55 WITH STATUSONLY"}},
		{"Oracle", Oracle, driver.ProcessRef{ID: "15' OR '1'='1", Serial: "3421"}},
		// The id is a number here and the serial is not, which is still not a session Oracle names.
		{"Oracle's other half", Oracle, driver.ProcessRef{ID: "15", Serial: "3421' OR '1'='1"}},
	}

	for _, one := range cases {
		if statement := one.dialect.KillProcess(closing(one.process)); statement != "" {
			t.Errorf("%s spelled %q, want nothing", one.engine, statement)
		}
	}
}

// And the run refuses it rather than quietly doing nothing: a kill that spelled nothing has to say
// why, or the reader is left believing a process was ended.
func TestKillProcessesRefusesSomethingThatIsNotAProcess(t *testing.T) {
	ran := 0
	watch := Watch{
		Dialect: MySQL,
		Run: func(context.Context, string, string) error {
			ran++
			return nil
		},
	}

	err := watch.KillProcesses(
		context.Background(),
		[]driver.ProcessKill{closing(driver.ProcessRef{ID: "42"}), closing(driver.ProcessRef{ID: "x"})},
	)

	var invalid domain.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("KillProcesses = %v, want a validation error", err)
	}
	// Refused **whole**: the check runs over every kill before the first statement does.
	if ran != 0 {
		t.Errorf("ran %d statements, want none", ran)
	}
}

// An engine with no statement for one half of it says so by name, so the reader answers it by not
// asking again.
func TestKillProcessesRefusesWhatTheEngineHasNotGot(t *testing.T) {
	watch := Watch{
		Dialect: SQLServer,
		Run:     func(context.Context, string, string) error { return nil },
	}

	err := watch.KillProcesses(context.Background(), []driver.ProcessKill{cancel("55")})
	if !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("KillProcesses = %v, want unsupported", err)
	}
}

// A reference is read out of the list by the **name** of the column that holds it, and `own` marks
// the connection this session's own read came back on.
func TestProcessRefsAreReadByName(t *testing.T) {
	columns := []driver.Column{{Name: "User"}, {Name: "ID"}, {Name: "Serial"}}
	rows := [][]*string{
		{cell("app"), cell("42"), cell("3421")},
		{cell("app"), cell("43"), nil},
	}

	refs := ProcessRefs(columns, rows, "ID", "Serial", "43")
	if len(refs) != 2 {
		t.Fatalf("read %d references, want 2", len(refs))
	}
	if refs[0].ID != "42" || refs[0].Serial != "3421" || refs[0].Own {
		t.Errorf("refs[0] = %+v", refs[0])
	}
	if refs[1].ID != "43" || refs[1].Serial != "" || !refs[1].Own {
		t.Errorf("refs[1] = %+v", refs[1])
	}
}

// Nothing is marked as the reader's own where the engine would not say which connection is asking.
func TestProcessRefsMarkNothingWithoutAnOwnID(t *testing.T) {
	columns := []driver.Column{{Name: "ID"}}
	rows := [][]*string{{cell("42")}}

	for _, ref := range ProcessRefs(columns, rows, "ID", "", "") {
		if ref.Own {
			t.Errorf("%+v is marked as ours", ref)
		}
	}
}
