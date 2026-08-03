package postgres

import (
	"context"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// CreateDatabaseSQL spells the whole of a create: the database itself, whatever extensions were
// asked for, and its comment. Only the extensions have to be run *inside* the database that has
// just been made — an extension is an object of that database. A comment on a database is held
// in a shared catalogue and can be set from anywhere, so it stays on the connection that is
// already open rather than costing a second pool.
func (s *session) CreateDatabaseSQL(spec driver.DatabaseSpec) []driver.DatabaseStatement {
	options := []string{}
	name := func(clause, value string) {
		if value != "" {
			options = append(options, clause+" = "+dialect.QuoteIdent(value))
		}
	}
	text := func(clause, value string) {
		if value != "" {
			options = append(options, clause+" = "+dialect.QuoteString(value))
		}
	}

	name("OWNER", spec.Owner)
	name("TEMPLATE", spec.Template)
	text("ENCODING", spec.Encoding)
	text("LC_COLLATE", spec.Collation)
	text("LC_CTYPE", spec.CType)
	name("TABLESPACE", spec.Tablespace)
	options = append(options,
		"ALLOW_CONNECTIONS = "+strconv.FormatBool(spec.AllowConnections),
		"IS_TEMPLATE = "+strconv.FormatBool(spec.IsTemplate),
		"CONNECTION LIMIT = "+strconv.Itoa(spec.ConnectionLimit),
	)

	statements := []driver.DatabaseStatement{{
		SQL: "CREATE DATABASE " + dialect.QuoteIdent(spec.Name) +
			"\n  WITH " + strings.Join(options, "\n       "),
	}}

	for _, extension := range spec.Extensions {
		statements = append(statements, driver.DatabaseStatement{
			In:  spec.Name,
			SQL: "CREATE EXTENSION IF NOT EXISTS " + dialect.QuoteIdent(extension),
		})
	}

	if strings.TrimSpace(spec.Comment) != "" {
		statements = append(statements, driver.DatabaseStatement{
			SQL: "COMMENT ON DATABASE " + dialect.QuoteIdent(spec.Name) +
				" IS " + dialect.QuoteString(spec.Comment),
		})
	}

	return statements
}

func (s *session) CreateDatabase(ctx context.Context, spec driver.DatabaseSpec) error {
	return s.RunAll(ctx, s.CreateDatabaseSQL(spec))
}

// DatabaseOptions reads the five lists the dialog offers. Encodings are asked of the server
// rather than listed here, so a build that knows one more knows it in the dialog too.
func (s *session) DatabaseOptions(ctx context.Context) (driver.DatabaseOptions, error) {
	const owners = `
		SELECT rolname
		FROM pg_catalog.pg_roles
		WHERE rolname NOT LIKE 'pg\_%'
		ORDER BY rolname`

	const templates = `SELECT datname FROM pg_catalog.pg_database ORDER BY datname`

	const encodings = `
		SELECT pg_catalog.pg_encoding_to_char(i) AS name
		FROM generate_series(0, 41) AS i
		WHERE pg_catalog.pg_encoding_to_char(i) <> ''
		ORDER BY name`

	const tablespaces = `SELECT spcname FROM pg_catalog.pg_tablespace ORDER BY spcname`

	const extensions = `SELECT name FROM pg_catalog.pg_available_extensions ORDER BY name`

	options := driver.DatabaseOptions{}
	reads := []struct {
		into  *[]string
		query string
	}{
		{&options.Owners, owners},
		{&options.Templates, templates},
		{&options.Encodings, encodings},
		{&options.Tablespaces, tablespaces},
		{&options.Extensions, extensions},
	}

	for _, read := range reads {
		names, err := s.names(ctx, "", read.query)
		if err != nil {
			return driver.DatabaseOptions{}, err
		}
		*read.into = names
	}

	return options, nil
}
