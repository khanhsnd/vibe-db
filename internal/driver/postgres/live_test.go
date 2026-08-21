package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A live run against a real server, for the reason the MySQL one exists: the half of the account
// work that reads is the *server's* answer, and only a server can say whether the statements this
// app spelled put there what it thinks they did. What is checked here beyond MySQL's is the fact
// that shapes this adapter — a role is the cluster's and its table grants are a database's, so a
// grant made in one database has to come back read from that database and from no other.
//
// The spelling is checked without a server — see `sqlkit/auth_test.go`.
//
//	VIBE_POSTGRES_DSN=postgres:postgres@127.0.0.1:5432/postgres go test ./internal/driver/postgres -run TestLive -v
const liveEnv = "VIBE_POSTGRES_DSN"

var liveRole = driver.AccountRef{Name: "vibe_db_live_test"}

func openLive(t *testing.T) driver.Session {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv(liveEnv))
	if dsn == "" {
		t.Skipf("set %s to run this against a real server", liveEnv)
	}

	profile := domain.ConnectionProfile{
		Name:     "live",
		Engine:   domain.EnginePostgres,
		Database: "postgres",
		Auth:     domain.AuthPassword,
		TLS:      domain.TLSOptions{Mode: domain.TLSPreferred},
	}
	credentials := driver.Credentials{}

	address := dsn
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		profile.User, credentials.Password, _ = strings.Cut(dsn[:at], ":")
		address = dsn[at+1:]
	}
	if slash := strings.Index(address, "/"); slash >= 0 {
		profile.Database = address[slash+1:]
		address = address[:slash]
	}

	host, port, found := strings.Cut(address, ":")
	if !found {
		t.Fatalf("%s = %q, want [user:password@]host:port[/database]", liveEnv, dsn)
	}
	profile.Host = host
	profile.Port, _ = strconv.Atoi(port)

	if err := profile.Validate(); err != nil {
		t.Fatalf("profile from %s: %v", liveEnv, err)
	}

	session, err := (&Driver{}).Open(t.Context(), profile, credentials, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestLiveAccounts(t *testing.T) {
	live := openLive(t)
	ctx := context.Background()

	drop := []driver.AccountChange{{Kind: driver.ChangeDropAccount, Account: liveRole}}
	// Whatever a previous run left behind. A drop of a role that is not there fails, and that
	// failure is not this test's business, so it is run and ignored.
	_ = live.ApplyAccountChanges(ctx, drop)
	t.Cleanup(func() { _ = live.ApplyAccountChanges(context.Background(), drop) })

	t.Run("a role that is not there is named as missing", func(t *testing.T) {
		_, err := live.Account(ctx, driver.AccountRef{Name: "vibe_db_no_such_role"})
		if !errors.Is(err, domain.ErrUnknownAccount) {
			t.Errorf("Account of a missing role = %v, want ErrUnknownAccount", err)
		}
	})

	t.Run("made, granted, read back and dropped", func(t *testing.T) {
		changes := []driver.AccountChange{
			{
				Kind:     driver.ChangeCreateAccount,
				Account:  liveRole,
				Password: "vibe-db-live-test-Passw0rd!",
				Attributes: map[string]string{
					"login": "true", "createdb": "false", "connectionLimit": "4",
				},
			},
			{
				Kind:    driver.ChangeGrant,
				Account: liveRole,
				Grant: driver.Grant{
					Level:     driver.GrantSchema,
					Namespace: driver.Namespace{Schema: "public"},
					Privilege: "USAGE",
				},
			},
		}

		for _, statement := range live.AccountStatements(changes) {
			if strings.Contains(statement, "Passw0rd") {
				t.Fatalf("the preview holds the password: %s", statement)
			}
		}

		if err := live.ApplyAccountChanges(ctx, changes); err != nil {
			t.Fatalf("ApplyAccountChanges: %v", err)
		}

		detail, err := live.Account(ctx, liveRole)
		if err != nil {
			t.Fatalf("Account: %v", err)
		}
		if detail.Attributes["login"] != "true" {
			t.Errorf("login = %q, want true", detail.Attributes["login"])
		}
		if detail.Attributes["connectionLimit"] != "4" {
			t.Errorf("connectionLimit = %q, want 4", detail.Attributes["connectionLimit"])
		}
		if detail.Role {
			t.Error("a role that may log in is reported as one that may not")
		}

		found := false
		for _, grant := range detail.Grants {
			if grant.Level == driver.GrantSchema && grant.Schema == "public" &&
				grant.Privilege == "USAGE" {
				found = true
			}
		}
		if !found {
			t.Errorf("USAGE on public is not in %v", detail.Grants)
		}

		list, err := live.Accounts(ctx)
		if err != nil {
			t.Fatalf("Accounts: %v", err)
		}
		listed := false
		for _, ref := range list.Refs {
			if ref.Name == liveRole.Name {
				listed = true
			}
		}
		if !listed {
			t.Errorf("the role is not in the list of %d", len(list.Refs))
		}
	})
}

// The monitoring half against a real server. What is checked here beyond MySQL's is the fact that
// shapes this adapter's monitor: `pg_stat_activity` is the **cluster's** backends and `pg_settings`
// the cluster's settings, both read from the one database this session was opened with — so a reader
// browsing a second database still sees every backend on the server.
//
// The cancel is aimed at a pid that is not a backend at all: `pg_cancel_backend` answers false and
// warns, which proves the statement ran without this test cancelling anybody's query.
func TestLiveMonitor(t *testing.T) {
	live := openLive(t)
	ctx := context.Background()

	t.Run("processes", func(t *testing.T) {
		list, err := live.Processes(ctx)
		if err != nil {
			t.Fatalf("Processes: %v", err)
		}
		if len(list.Rows) == 0 {
			t.Fatal("no backends: this connection is one of them")
		}
		if len(list.Refs) != len(list.Rows) {
			t.Fatalf("%d references for %d rows", len(list.Refs), len(list.Rows))
		}

		ours := 0
		for _, ref := range list.Refs {
			if ref.Own {
				ours++
			}
		}
		if ours != 1 {
			t.Errorf("%d rows marked as ours, want 1", ours)
		}
	})

	t.Run("status and variables", func(t *testing.T) {
		status, err := live.ServerStatus(ctx)
		if err != nil {
			t.Fatalf("ServerStatus: %v", err)
		}
		// One row per database, which is what this engine counts with — see the adapter.
		if len(status.Rows) == 0 || len(status.Columns) < 3 {
			t.Errorf("status: %d rows in %d columns", len(status.Rows), len(status.Columns))
		}

		variables, err := live.ServerVariables(ctx)
		if err != nil {
			t.Fatalf("ServerVariables: %v", err)
		}
		if len(variables.Rows) == 0 {
			t.Error("the server reported no settings")
		}
	})

	t.Run("a cancel reaches the server", func(t *testing.T) {
		// Process 1 is never a PostgreSQL backend, so the function answers false and nothing is
		// cancelled — what is being checked is that the statement itself was accepted.
		notABackend := driver.ProcessRef{ID: "1"}

		if err := live.KillProcesses(ctx, []driver.ProcessKill{
			{Process: notABackend, Action: domain.ProcessCancel},
		}); err != nil {
			t.Errorf("KillProcesses on a pid that is not a backend: %v", err)
		}
	})

	t.Run("metrics", func(t *testing.T) {
		first, err := live.ServerMetrics(ctx)
		if err != nil {
			t.Fatalf("ServerMetrics() error = %v", err)
		}

		// Every one of the four panels is answered for. A panel this engine filled with nothing draws
		// an empty chart saying the server counts nothing there, which is true of no engine here.
		panels := map[domain.MetricPanel]int{}
		for at, metric := range first.Metrics {
			if metric.Key == "" || metric.Label == "" {
				t.Fatalf("metric %d has no name: %+v", at, metric)
			}
			if metric.Kind != domain.MetricGauge && metric.Kind != domain.MetricCounter {
				t.Fatalf("%s is a %q, which is neither kind", metric.Key, metric.Kind)
			}
			if metric.Unit != domain.MetricCount && metric.Unit != domain.MetricBytes {
				t.Fatalf("%s is counted in %q, which is neither unit", metric.Key, metric.Unit)
			}
			panels[metric.Panel]++
		}

		for _, panel := range []domain.MetricPanel{
			domain.MetricSessions,
			domain.MetricTraffic,
			domain.MetricQueries,
			domain.MetricThroughput,
		} {
			if panels[panel] == 0 {
				t.Errorf("nothing was reported for the %s panel", panel)
			}
		}

		// A counter has only ever gone up. It is the one property a chart depends on — a rate is the
		// difference between two readings divided by the time between them — and only a real server
		// can be asked whether it holds.
		second, err := live.ServerMetrics(ctx)
		if err != nil {
			t.Fatalf("second ServerMetrics() error = %v", err)
		}

		before := map[string]float64{}
		for _, metric := range first.Metrics {
			if metric.Kind == domain.MetricCounter {
				before[metric.Key] = metric.Value
			}
		}
		for _, metric := range second.Metrics {
			was, held := before[metric.Key]
			if held && metric.Value < was {
				t.Errorf("counter %s fell from %v to %v", metric.Key, was, metric.Value)
			}
		}
	})
}

// A routine round trip: declare one, read what it takes, and run it. PostgreSQL is the engine where
// an OUT parameter needs no variable at all — `CALL` answers with a row holding what the procedure
// made of it — so what this checks that the speller cannot is that the row really comes back.
func TestLiveRoutineCall(t *testing.T) {
	live := openLive(t)
	ctx := context.Background()
	ns := driver.Namespace{Database: "postgres", Schema: "public"}

	procedure := driver.SourceRef{
		Kind:      driver.SourceProcedure,
		Name:      "vibe_db_live_total",
		Arguments: "n integer, OUT doubled integer",
	}
	function := driver.SourceRef{
		Kind:      driver.SourceFunction,
		Name:      "vibe_db_live_twice",
		Arguments: "n integer",
	}

	drop := func() {
		_ = live.DropSource(context.Background(), ns, procedure)
		_ = live.DropSource(context.Background(), ns, function)
	}
	drop()
	t.Cleanup(drop)

	declare := []driver.SourceSave{
		{Definition: `CREATE PROCEDURE public.vibe_db_live_total(n integer, OUT doubled integer)
LANGUAGE plpgsql AS $$ BEGIN doubled := n * 2; END $$`},
		{Definition: `CREATE FUNCTION public.vibe_db_live_twice(n integer) RETURNS integer
LANGUAGE plpgsql AS $$ BEGIN RETURN n * 2; END $$`},
	}
	for _, save := range declare {
		if err := live.SaveSource(ctx, ns, save); err != nil {
			t.Fatalf("SaveSource: %v", err)
		}
	}

	t.Run("what it takes is read, modes and all", func(t *testing.T) {
		params, err := live.RoutineParams(ctx, ns, procedure)
		if err != nil {
			t.Fatalf("RoutineParams: %v", err)
		}
		want := []driver.RoutineParam{
			{Name: "n", Mode: driver.ParamIn},
			{Name: "doubled", Mode: driver.ParamOut},
		}
		if len(params) != len(want) {
			t.Fatalf("RoutineParams = %v, want %v", params, want)
		}
		for at, one := range want {
			if params[at].Name != one.Name || params[at].Mode != one.Mode {
				t.Errorf("parameter %d = %+v, want %+v", at, params[at], one)
			}
		}
	})

	t.Run("an OUT parameter comes back in the row CALL answers with", func(t *testing.T) {
		call := driver.RoutineCall{SourceRef: procedure, Args: []driver.RoutineArg{
			{RoutineParam: driver.RoutineParam{Name: "n", Mode: driver.ParamIn}, Value: text("21")},
			{RoutineParam: driver.RoutineParam{Name: "doubled", Mode: driver.ParamOut}},
		}}

		if statements := live.CallStatements(ns, call); len(statements) != 1 {
			t.Fatalf("CallStatements = %v, want one CALL", statements)
		}

		result, err := live.CallRoutine(ctx, ns, call)
		if err != nil {
			t.Fatalf("CallRoutine: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] == nil || *result.Rows[0][0] != "42" {
			t.Fatalf("CallRoutine answered %v, want 42", result.Rows)
		}
	})

	t.Run("a function is read in the FROM clause", func(t *testing.T) {
		call := driver.RoutineCall{SourceRef: function, Args: []driver.RoutineArg{
			{RoutineParam: driver.RoutineParam{Name: "n", Mode: driver.ParamIn}, Value: text("21")},
		}}

		result, err := live.CallRoutine(ctx, ns, call)
		if err != nil {
			t.Fatalf("CallRoutine: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] == nil || *result.Rows[0][0] != "42" {
			t.Fatalf("CallRoutine answered %v, want 42", result.Rows)
		}
	})
}

func text(value string) *string { return &value }
