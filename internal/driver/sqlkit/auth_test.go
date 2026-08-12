package sqlkit

import (
	"errors"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The account half of the five grammars, checked where every difference between them lives: how an
// account is named, where its password goes, what a grant's `ON` clause looks like, and which of
// the nine changes an engine has no statement for at all.

// MySQL is the one engine whose account is a **pair**, written as two string literals rather than
// as a name — and the one whose grant names its object with a wildcard.
func TestAccountStatementsOnMySQL(t *testing.T) {
	app := driver.AccountRef{Name: "app", Host: "10.%"}

	changes := []driver.AccountChange{
		{
			Kind:       driver.ChangeCreateAccount,
			Account:    app,
			Password:   "s3cret",
			Attributes: map[string]string{"maxConnections": "20", "locked": "false"},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: app,
			Grant: driver.Grant{
				Level:     driver.GrantTable,
				Namespace: driver.Namespace{Database: "shop"},
				Object:    "orders",
				Privilege: "SELECT",
			},
		},
		// The same target, so it folds into the statement above rather than becoming a second one.
		{
			Kind:    driver.ChangeGrant,
			Account: app,
			Grant: driver.Grant{
				Level:     driver.GrantTable,
				Namespace: driver.Namespace{Database: "shop"},
				Object:    "orders",
				Privilege: "INSERT",
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: app,
			Grant: driver.Grant{
				Level:     driver.GrantDatabase,
				Namespace: driver.Namespace{Database: "shop"},
				Privilege: "SELECT",
				Grantable: true,
			},
		},
		{Kind: driver.ChangeGrantRole, Account: app, Role: "reader"},
		{Kind: driver.ChangeRenameAccount, Account: app, Name: "web"},
	}

	want := []string{
		"CREATE USER 'app'@'10.%' IDENTIFIED BY <password>" +
			" WITH MAX_USER_CONNECTIONS 20 ACCOUNT UNLOCK",
		"GRANT SELECT, INSERT ON `shop`.`orders` TO 'app'@'10.%'",
		"GRANT SELECT ON `shop`.* TO 'app'@'10.%' WITH GRANT OPTION",
		"GRANT 'reader'@'%' TO 'app'@'10.%'",
		"RENAME USER 'app'@'10.%' TO 'web'@'10.%'",
	}
	assertStatements(t, "MySQL", Auth{Dialect: MySQL}.AccountStatements(changes), want)
}

// PostgreSQL declares a **role** rather than a user, puts everything after one `WITH`, and names
// the class of the thing a privilege is granted on.
func TestAccountStatementsOnPostgres(t *testing.T) {
	alice := driver.AccountRef{Name: "alice"}

	changes := []driver.AccountChange{
		{
			Kind:     driver.ChangeCreateAccount,
			Account:  alice,
			Password: "s3cret",
			Attributes: map[string]string{
				"login": "true", "superuser": "false", "connectionLimit": "5",
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: alice,
			Grant: driver.Grant{
				Level:     driver.GrantDatabase,
				Namespace: driver.Namespace{Database: "shop"},
				Privilege: "CONNECT",
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: alice,
			Grant: driver.Grant{
				Level:     driver.GrantSchema,
				Namespace: driver.Namespace{Database: "shop", Schema: "public"},
				Privilege: "USAGE",
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: alice,
			Grant: driver.Grant{
				Level:     driver.GrantColumn,
				Namespace: driver.Namespace{Database: "shop", Schema: "public"},
				Object:    "orders",
				Columns:   []string{"total", "placed_at"},
				Privilege: "UPDATE",
			},
		},
		{
			Kind:    driver.ChangeRevoke,
			Account: alice,
			Grant: driver.Grant{
				Level:     driver.GrantTable,
				Namespace: driver.Namespace{Database: "shop", Schema: "public"},
				Object:    "orders",
				Privilege: "DELETE",
				// Ignored on a revoke: taking the privilege takes the option with it.
				Grantable: true,
			},
		},
		{Kind: driver.ChangeDropAccount, Account: alice},
	}

	want := []string{
		"CREATE ROLE \"alice\" WITH PASSWORD <password> LOGIN NOSUPERUSER CONNECTION LIMIT 5",
		"GRANT CONNECT ON DATABASE \"shop\" TO \"alice\"",
		"GRANT USAGE ON SCHEMA \"public\" TO \"alice\"",
		"GRANT UPDATE (\"total\", \"placed_at\") ON TABLE \"public\".\"orders\" TO \"alice\"",
		"REVOKE DELETE ON TABLE \"public\".\"orders\" FROM \"alice\"",
		"DROP ROLE \"alice\"",
	}
	assertStatements(t, "Postgres", Auth{Dialect: Postgres}.AccountStatements(changes), want)
}

// SQL Server is where the most differences meet at once: an account is two objects, a permission is
// granted on a *class*, the columns sit after the object rather than after the privilege, a role is
// altered instead of granted, and there is a third verb.
func TestAccountStatementsOnSQLServer(t *testing.T) {
	web := driver.AccountRef{Name: "web", Database: "shop"}

	changes := []driver.AccountChange{
		{
			Kind:     driver.ChangeCreateAccount,
			Account:  web,
			Password: "s3cret",
			Attributes: map[string]string{
				"defaultDatabase": "shop", "defaultSchema": "dbo", "disabled": "false",
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: web,
			Grant: driver.Grant{
				Level:     driver.GrantColumn,
				Namespace: driver.Namespace{Database: "shop", Schema: "dbo"},
				Object:    "orders",
				Columns:   []string{"total"},
				Privilege: "UPDATE",
			},
		},
		{
			Kind:    driver.ChangeDeny,
			Account: web,
			Grant: driver.Grant{
				Level:     driver.GrantSchema,
				Namespace: driver.Namespace{Database: "shop", Schema: "audit"},
				Privilege: "SELECT",
			},
		},
		{Kind: driver.ChangeGrantRole, Account: web, Role: "db_datareader"},
		{Kind: driver.ChangeDropAccount, Account: web},
	}

	want := []string{
		"CREATE LOGIN [web] WITH PASSWORD = <password> DEFAULT_DATABASE = [shop]",
		"ALTER LOGIN [web] ENABLE",
		"CREATE USER [web] FOR LOGIN [web] WITH DEFAULT_SCHEMA = [dbo]",
		"GRANT UPDATE ON OBJECT::[dbo].[orders] ([total]) TO [web]",
		"DENY SELECT ON SCHEMA::[audit] TO [web]",
		"ALTER ROLE [db_datareader] ADD MEMBER [web]",
		"DROP USER [web]",
		"DROP LOGIN [web]",
	}
	assertStatements(t, "SQLServer", Auth{Dialect: SQLServer}.AccountStatements(changes), want)
}

// Oracle quotes a password as a **name**, administers a privilege over the server rather than
// granting it, and drops an account with `CASCADE` — without which the statement is refused for
// any user that owns anything, which on this engine is every user with a table.
func TestAccountStatementsOnOracle(t *testing.T) {
	hr := driver.AccountRef{Name: "HR"}

	changes := []driver.AccountChange{
		{
			Kind:       driver.ChangeCreateAccount,
			Account:    hr,
			Password:   "s3cret",
			Attributes: map[string]string{"tablespace": "USERS", "locked": "false"},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: hr,
			Grant: driver.Grant{
				Level:     driver.GrantServer,
				Privilege: "CREATE SESSION",
				Grantable: true,
			},
		},
		{
			Kind:    driver.ChangeGrant,
			Account: hr,
			Grant: driver.Grant{
				Level:     driver.GrantTable,
				Namespace: driver.Namespace{Database: "SALES"},
				Object:    "ORDERS",
				Privilege: "SELECT",
				Grantable: true,
			},
		},
		{Kind: driver.ChangeDropAccount, Account: hr},
	}

	want := []string{
		"CREATE USER \"HR\" IDENTIFIED BY \"s3cret\" DEFAULT TABLESPACE \"USERS\" ACCOUNT UNLOCK",
		"GRANT CREATE SESSION TO \"HR\" WITH ADMIN OPTION",
		"GRANT SELECT ON \"SALES\".\"ORDERS\" TO \"HR\" WITH GRANT OPTION",
		"DROP USER \"HR\" CASCADE",
	}
	// Spelled with the password in it, which is what a run sends. Every other case here checks the
	// preview, where it is a placeholder — see TestPasswordIsNeverPreviewed.
	got := Auth{Dialect: Oracle}.spell(changes, true)
	statements := make([]string, 0, len(got))
	for _, one := range got {
		statements = append(statements, one.SQL)
	}
	assertStatements(t, "Oracle", statements, want)
}

// The rule that costs the most to get wrong: what the reader is shown, what the History Log keeps
// and what a saved history holds must never be the password itself.
func TestPasswordIsNeverPreviewed(t *testing.T) {
	changes := []driver.AccountChange{{
		Kind:     driver.ChangeCreateAccount,
		Account:  driver.AccountRef{Name: "alice"},
		Password: "hunter2",
	}}

	for name, dialect := range map[string]Dialect{
		"MySQL": MySQL, "Postgres": Postgres, "SQLServer": SQLServer, "Oracle": Oracle,
	} {
		for _, statement := range (Auth{Dialect: dialect}).AccountStatements(changes) {
			if strings.Contains(statement, "hunter2") {
				t.Errorf("%s previewed the password: %s", name, statement)
			}
			if strings.Contains(statement, "PASSWORD") &&
				!strings.Contains(statement, PasswordPlaceholder) {
				t.Errorf("%s previewed no placeholder: %s", name, statement)
			}
		}
	}
}

// An engine with no statement for a change says so, naming it, rather than leaving one for the
// server to refuse — and it says so **before** anything runs, so a list holding one is not applied
// halfway.
func TestUnsupportedChangesAreRefusedByName(t *testing.T) {
	cases := []struct {
		engine  string
		dialect Dialect
		change  driver.AccountChange
	}{
		{"Oracle", Oracle, driver.AccountChange{
			Kind:    driver.ChangeRenameAccount,
			Account: driver.AccountRef{Name: "HR"},
			Name:    "PEOPLE",
		}},
		{"MySQL", MySQL, driver.AccountChange{
			Kind:    driver.ChangeDeny,
			Account: driver.AccountRef{Name: "app"},
			Grant:   driver.Grant{Level: driver.GrantServer, Privilege: "SELECT"},
		}},
	}

	for _, one := range cases {
		err := Auth{Dialect: one.dialect}.ApplyAccountChanges(t.Context(), []driver.AccountChange{
			one.change,
		})
		if !errors.Is(err, domain.ErrUnsupported) {
			t.Errorf("%s answered %v, want an unsupported change", one.engine, err)
		}
	}
}

// A number that is not one contributes nothing rather than reaching the statement. DDL binds no
// arguments, so this is the one place text a reader typed would otherwise be pasted into one.
func TestAttributeNumberIsRefusedRatherThanPasted(t *testing.T) {
	changes := []driver.AccountChange{{
		Kind:       driver.ChangeAlterAccount,
		Account:    driver.AccountRef{Name: "alice"},
		Attributes: map[string]string{"connectionLimit": "5; DROP DATABASE shop"},
	}}

	statements := Auth{Dialect: Postgres}.AccountStatements(changes)
	if len(statements) != 0 {
		t.Fatalf("spelled %v, want nothing", statements)
	}
}
