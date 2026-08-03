package postgres

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestCreateDatabaseSQLNamesOnlyWhatWasFilledIn(t *testing.T) {
	s := &session{}

	statements := s.CreateDatabaseSQL(driver.DatabaseSpec{
		Name:             "shop",
		Owner:            "app",
		Encoding:         "UTF8",
		ConnectionLimit:  -1,
		AllowConnections: true,
	})

	if len(statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(statements))
	}

	got := statements[0].SQL
	for _, want := range []string{
		`CREATE DATABASE "shop"`,
		`OWNER = "app"`,
		`ENCODING = 'UTF8'`,
		`ALLOW_CONNECTIONS = true`,
		`IS_TEMPLATE = false`,
		`CONNECTION LIMIT = -1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("statement is missing %s:\n%s", want, got)
		}
	}

	// A clause nobody filled in is left out, so the server applies its own default rather
	// than this side guessing at one.
	for _, absent := range []string{`TEMPLATE = "`, "LC_COLLATE", "LC_CTYPE", "TABLESPACE"} {
		if strings.Contains(got, absent) {
			t.Errorf("statement carries an empty %s:\n%s", absent, got)
		}
	}
}

// An extension is an object of the database it is installed in, so it has to be run inside the
// database that has just been made. A comment is held in a shared catalogue and is not.
func TestCreateDatabaseSQLRunsExtensionsInsideTheNewDatabase(t *testing.T) {
	s := &session{}

	statements := s.CreateDatabaseSQL(driver.DatabaseSpec{
		Name:       "shop",
		Extensions: []string{"pgcrypto"},
		Comment:    "the shop's own",
	})

	if len(statements) != 3 {
		t.Fatalf("got %d statements, want 3", len(statements))
	}
	if statements[0].In != "" {
		t.Errorf("the create runs in %q, want the session's own database", statements[0].In)
	}
	if statements[1].In != "shop" {
		t.Errorf("the extension runs in %q, want shop", statements[1].In)
	}
	if statements[2].In != "" {
		t.Errorf("the comment runs in %q, want the session's own database", statements[2].In)
	}

	if want := `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`; statements[1].SQL != want {
		t.Errorf("extension = %s, want %s", statements[1].SQL, want)
	}
	if want := `COMMENT ON DATABASE "shop" IS 'the shop''s own'`; statements[2].SQL != want {
		t.Errorf("comment = %s, want %s", statements[2].SQL, want)
	}
}
