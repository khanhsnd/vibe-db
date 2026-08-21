package mysql

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

// A live run against a real server, because the one half of the account work that cannot be checked
// any other way is the half that reads: `mysql.user`, the four grant tables and `SHOW GRANTS` are
// the *server's* answers, and only a server can say whether the statements this app spelled put
// there what it thinks they did.
//
// The spelling is checked without one — see `sqlkit/auth_test.go`, which holds every dialect to the
// statement it writes. What is here is the round trip: make an account, grant it something, read it
// back, and drop it.
//
// It is skipped unless `VIBE_MYSQL_DSN` names a server — see docs/development.md — and it works on an
// account of its own name, which it drops first and last, so nothing it does can be mistaken for
// something the reader put there.
//
//	VIBE_MYSQL_DSN=root:mysql@127.0.0.1:3306 go test ./internal/driver/mysql -run TestLive -v
const liveEnv = "VIBE_MYSQL_DSN"

// liveAccount is the account the run makes. The host is spelled out rather than left to default, so
// a failure names the same pair the statements did.
var liveAccount = driver.AccountRef{Name: "vibe_db_live_test", Host: "%"}

func openLive(t *testing.T) driver.Session {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv(liveEnv))
	if dsn == "" {
		t.Skipf("set %s to run this against a real server", liveEnv)
	}

	profile := domain.ConnectionProfile{
		Name:   "live",
		Engine: domain.EngineMySQL,
		Auth:   domain.AuthPassword,
		TLS:    domain.TLSOptions{Mode: domain.TLSPreferred},
	}
	credentials := driver.Credentials{}

	address := dsn
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		profile.User, credentials.Password, _ = strings.Cut(dsn[:at], ":")
		address = dsn[at+1:]
	}

	host, port, found := strings.Cut(address, ":")
	if !found {
		t.Fatalf("%s = %q, want [user:password@]host:port", liveEnv, dsn)
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

	// Whatever a previous run left behind. A drop of an account that is not there fails, and that
	// failure is not this test's business, so it is run and ignored.
	_ = live.ApplyAccountChanges(ctx, []driver.AccountChange{
		{Kind: driver.ChangeDropAccount, Account: liveAccount},
	})
	t.Cleanup(func() {
		_ = live.ApplyAccountChanges(context.Background(), []driver.AccountChange{
			{Kind: driver.ChangeDropAccount, Account: liveAccount},
		})
	})

	t.Run("options are read from the server", func(t *testing.T) {
		options, err := live.AccountOptions(ctx)
		if err != nil {
			t.Fatalf("AccountOptions: %v", err)
		}
		// `SHOW PRIVILEGES` is what fills these, so an empty list means the read failed rather than
		// that the server understands nothing.
		for _, set := range options.Privileges {
			if len(set.Names) == 0 {
				t.Errorf("%s has no privileges", set.Level)
			}
		}
		if !holds(privilegesAt(options, driver.GrantTable), "SELECT") {
			t.Error("SELECT is not offered at table level")
		}
	})

	t.Run("an account that is not there is named as missing", func(t *testing.T) {
		_, err := live.Account(ctx, driver.AccountRef{Name: "vibe_db_no_such_user", Host: "%"})
		if !errors.Is(err, domain.ErrUnknownAccount) {
			t.Errorf("Account of a missing user = %v, want ErrUnknownAccount", err)
		}
	})

	t.Run("made, granted, read back and dropped", func(t *testing.T) {
		changes := []driver.AccountChange{
			{
				Kind:       driver.ChangeCreateAccount,
				Account:    liveAccount,
				Password:   "vibe-db-live-test-Passw0rd!",
				Attributes: map[string]string{"maxConnections": "3"},
			},
			{
				Kind:    driver.ChangeGrant,
				Account: liveAccount,
				Grant: driver.Grant{
					Level:     driver.GrantDatabase,
					Namespace: driver.Namespace{Database: "mysql"},
					Privilege: "SELECT",
				},
			},
		}

		// The preview is checked before the run, because the one thing that must never be true of it
		// is that the password is in it.
		for _, statement := range live.AccountStatements(changes) {
			if strings.Contains(statement, "Passw0rd") {
				t.Fatalf("the preview holds the password: %s", statement)
			}
		}

		if err := live.ApplyAccountChanges(ctx, changes); err != nil {
			t.Fatalf("ApplyAccountChanges: %v", err)
		}

		detail, err := live.Account(ctx, liveAccount)
		if err != nil {
			t.Fatalf("Account: %v", err)
		}
		if detail.Attributes["maxConnections"] != "3" {
			t.Errorf("maxConnections = %q, want 3", detail.Attributes["maxConnections"])
		}
		if !granted(detail.Grants, driver.GrantDatabase, "mysql", "SELECT") {
			t.Errorf("SELECT on mysql.* is not in %v", detail.Grants)
		}
		// The server's own words, which is the half of this a catalogue read cannot give.
		if len(detail.Source) == 0 {
			t.Error("SHOW GRANTS answered with nothing")
		}

		list, err := live.Accounts(ctx)
		if err != nil {
			t.Fatalf("Accounts: %v", err)
		}
		if !listed(list, liveAccount) {
			t.Errorf("the account is not in the list of %d", len(list.Refs))
		}

		revoke := []driver.AccountChange{{
			Kind:    driver.ChangeRevoke,
			Account: liveAccount,
			Grant: driver.Grant{
				Level:     driver.GrantDatabase,
				Namespace: driver.Namespace{Database: "mysql"},
				Privilege: "SELECT",
			},
		}}
		if err := live.ApplyAccountChanges(ctx, revoke); err != nil {
			t.Fatalf("revoke: %v", err)
		}

		after, err := live.Account(ctx, liveAccount)
		if err != nil {
			t.Fatalf("Account after revoke: %v", err)
		}
		if granted(after.Grants, driver.GrantDatabase, "mysql", "SELECT") {
			t.Error("SELECT on mysql.* survived the revoke")
		}
	})
}

func privilegesAt(options driver.AccountOptions, level driver.GrantLevel) []string {
	for _, set := range options.Privileges {
		if set.Level == level {
			return set.Names
		}
	}
	return nil
}

func holds(names []string, wanted string) bool {
	for _, name := range names {
		if name == wanted {
			return true
		}
	}
	return false
}

func granted(grants []driver.Grant, level driver.GrantLevel, database, privilege string) bool {
	for _, grant := range grants {
		if grant.Level == level && grant.Database == database && grant.Privilege == privilege {
			return true
		}
	}
	return false
}

func listed(list driver.AccountList, wanted driver.AccountRef) bool {
	for _, ref := range list.Refs {
		if ref.Name == wanted.Name && ref.Host == wanted.Host {
			return true
		}
	}
	return false
}

// The monitoring half against a real server. The reads are what only a server can check — an alias
// this app spelled wrongly is a column the grid draws under the wrong name, and nothing without a
// server would notice — and the kill is checked against a thread that is **not there**: the point is
// that the statement parsed and reached the server, which its own "unknown thread id" proves without
// this test ending anybody's connection.
func TestLiveMonitor(t *testing.T) {
	live := openLive(t)
	ctx := context.Background()

	t.Run("processes", func(t *testing.T) {
		list, err := live.Processes(ctx)
		if err != nil {
			t.Fatalf("Processes: %v", err)
		}
		if len(list.Rows) == 0 {
			t.Fatal("no processes: this connection is one of them")
		}
		if len(list.Refs) != len(list.Rows) {
			t.Fatalf("%d references for %d rows", len(list.Refs), len(list.Rows))
		}
		if columnAt(list.Columns, "ID") < 0 || columnAt(list.Columns, "Statement") < 0 {
			t.Errorf("columns = %v", names(list.Columns))
		}

		// One row is the connection this read came back on, which is what the hazard rule turns on.
		ours := 0
		for _, ref := range list.Refs {
			if ref.Own {
				ours++
			}
			if ref.ID == "" {
				t.Error("a process with no id")
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
		if len(status.Rows) == 0 || len(status.Columns) != 2 {
			t.Errorf("status: %d rows in %d columns", len(status.Rows), len(status.Columns))
		}

		variables, err := live.ServerVariables(ctx)
		if err != nil {
			t.Fatalf("ServerVariables: %v", err)
		}
		if len(variables.Rows) == 0 {
			t.Error("the server reported no variables")
		}
	})

	t.Run("a kill reaches the server", func(t *testing.T) {
		// A thread id no server hands out, so what comes back is the server's own refusal rather than
		// somebody's connection ending.
		gone := driver.ProcessRef{ID: "2147483647"}

		if err := live.KillProcesses(ctx, []driver.ProcessKill{
			{Process: gone, Action: domain.ProcessClose},
		}); !errors.Is(err, domain.ErrStatementFailed) {
			t.Errorf("KillProcesses on a thread that is gone = %v, want the server's own refusal", err)
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

func columnAt(columns []driver.Column, wanted string) int {
	for at, column := range columns {
		if column.Name == wanted {
			return at
		}
	}
	return -1
}

func names(columns []driver.Column) []string {
	found := make([]string, 0, len(columns))
	for _, column := range columns {
		found = append(found, column.Name)
	}
	return found
}

// A routine round trip: declare one, read what it takes, and run it with its arguments filled in.
// It is here rather than beside the speller for the reason the account round trip is — `sqlkit`
// holds every dialect to the statement it writes, and only a server can say whether that statement
// does what this app thinks it does. The OUT half is the reason this cannot be checked any other
// way: the value comes back through a **user variable**, and a variable is the connection's.
func TestLiveRoutineCall(t *testing.T) {
	live := openLive(t)
	ctx := context.Background()
	ns := driver.Namespace{Database: "mysql"}

	// A schema of this test's own, so nothing it declares can be mistaken for something the reader
	// put there. It is made in `mysql` because that database is on every server there is.
	procedure := driver.SourceRef{Kind: driver.SourceProcedure, Name: "vibe_db_live_total"}
	function := driver.SourceRef{Kind: driver.SourceFunction, Name: "vibe_db_live_twice"}

	drop := func() {
		_ = live.DropSource(context.Background(), ns, procedure)
		_ = live.DropSource(context.Background(), ns, function)
	}
	drop()
	t.Cleanup(drop)

	declare := []driver.SourceSave{
		{Definition: "CREATE PROCEDURE `mysql`.`vibe_db_live_total`" +
			"(IN n INT, OUT doubled INT)\nBEGIN\n  SET doubled = n * 2;\nEND"},
		{Definition: "CREATE FUNCTION `mysql`.`vibe_db_live_twice`(n INT)\n" +
			"RETURNS INT\nDETERMINISTIC\nBEGIN\n  RETURN n * 2;\nEND"},
	}
	for _, save := range declare {
		if err := live.SaveSource(ctx, ns, save); err != nil {
			t.Fatalf("SaveSource: %v", err)
		}
	}

	t.Run("what it takes is read from the catalogue", func(t *testing.T) {
		params, err := live.RoutineParams(ctx, ns, procedure)
		if err != nil {
			t.Fatalf("RoutineParams: %v", err)
		}
		want := []driver.RoutineParam{
			{Name: "n", Mode: driver.ParamIn, Type: "int"},
			{Name: "doubled", Mode: driver.ParamOut, Type: "int"},
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

	t.Run("an OUT parameter comes back through its variable", func(t *testing.T) {
		call := driver.RoutineCall{SourceRef: procedure, Args: []driver.RoutineArg{
			{RoutineParam: driver.RoutineParam{Name: "n", Mode: driver.ParamIn}, Value: text("21")},
			{RoutineParam: driver.RoutineParam{Name: "doubled", Mode: driver.ParamOut}},
		}}

		// The preview is what the reader was shown; the run has to be those very statements.
		if statements := live.CallStatements(ns, call); len(statements) != 3 {
			t.Fatalf("CallStatements = %v, want a set, a call and a read", statements)
		}

		result, err := live.CallRoutine(ctx, ns, call)
		if err != nil {
			t.Fatalf("CallRoutine: %v", err)
		}
		if len(result.Rows) != 1 || len(result.Rows[0]) != 1 || result.Rows[0][0] == nil {
			t.Fatalf("CallRoutine answered %v, want one value", result.Rows)
		}
		if *result.Rows[0][0] != "42" {
			t.Errorf("doubled = %s, want 42", *result.Rows[0][0])
		}
	})

	t.Run("a function is read", func(t *testing.T) {
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
