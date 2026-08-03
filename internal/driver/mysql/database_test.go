package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestCreateDatabaseSQLCarriesTheCharacterSetAndCollation(t *testing.T) {
	s := &session{}

	statements := s.CreateDatabaseSQL(driver.DatabaseSpec{
		Name:      "shop",
		Encoding:  "utf8mb4",
		Collation: "utf8mb4_0900_ai_ci",
		// Everything else is PostgreSQL's, and reaches no MySQL statement.
		Owner:           "app",
		Template:        "template0",
		ConnectionLimit: 20,
		IsTemplate:      true,
	})

	if len(statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(statements))
	}

	got := statements[0].SQL
	want := "CREATE DATABASE `shop` CHARACTER SET 'utf8mb4' COLLATE 'utf8mb4_0900_ai_ci'"
	if got != want {
		t.Errorf("CreateDatabaseSQL = %s, want %s", got, want)
	}
}

func TestCreateDatabaseSQLWithoutACharacterSetIsJustTheName(t *testing.T) {
	s := &session{}

	got := s.CreateDatabaseSQL(driver.DatabaseSpec{Name: "shop"})[0].SQL
	want := "CREATE DATABASE `shop`"

	if got != want {
		t.Errorf("CreateDatabaseSQL = %s, want %s", got, want)
	}
}
