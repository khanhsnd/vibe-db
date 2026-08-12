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
