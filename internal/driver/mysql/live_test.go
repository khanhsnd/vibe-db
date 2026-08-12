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
