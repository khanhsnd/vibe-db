package mysql

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// CreateDatabaseSQL is one statement and always will be: MySQL's database has a character set
// and a collation and nothing else — no owner, no template, no comment of its own.
func (s *session) CreateDatabaseSQL(spec driver.DatabaseSpec) []driver.DatabaseStatement {
	statement := "CREATE DATABASE " + dialect.QuoteIdent(spec.Name)

	if spec.Encoding != "" {
		statement += " CHARACTER SET " + dialect.QuoteString(spec.Encoding)
	}
	if spec.Collation != "" {
		statement += " COLLATE " + dialect.QuoteString(spec.Collation)
	}

	return []driver.DatabaseStatement{{SQL: statement}}
}

func (s *session) CreateDatabase(ctx context.Context, spec driver.DatabaseSpec) error {
	return s.RunAll(ctx, s.CreateDatabaseSQL(spec))
}

// DatabaseOptions fills the two lists MySQL has. A collation is named after the character set
// it belongs to — `utf8mb4_general_ci` — which is how the dialog narrows the second list to
// the first without a second read.
func (s *session) DatabaseOptions(ctx context.Context) (driver.DatabaseOptions, error) {
	const charsets = `
		SELECT character_set_name
		FROM information_schema.character_sets
		ORDER BY character_set_name`

	const collations = `
		SELECT collation_name
		FROM information_schema.collations
		ORDER BY collation_name`

	encodings, err := s.names(ctx, charsets)
	if err != nil {
		return driver.DatabaseOptions{}, err
	}

	sorts, err := s.names(ctx, collations)
	if err != nil {
		return driver.DatabaseOptions{}, err
	}

	return driver.DatabaseOptions{Encodings: encodings, Collations: sorts}, nil
}
