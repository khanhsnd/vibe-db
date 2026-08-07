package sqlserver

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// CreateDatabaseSQL is one statement and very nearly always will be: a SQL Server database has a
// collation and nothing else the dialog asks for. Everything that makes one interesting — where its
// files go, how big they start, what recovery model it takes — is storage rather than a namespace,
// and a dialog that offered a file path would be offering a path on the *server's* disk.
func (s *session) CreateDatabaseSQL(spec driver.DatabaseSpec) []driver.DatabaseStatement {
	statement := "CREATE DATABASE " + dialect.QuoteIdent(spec.Name)

	// A collation is a name here rather than a string: `COLLATE Latin1_General_CI_AS` takes an
	// identifier, and a quoted literal is refused.
	if spec.Collation != "" {
		statement += " COLLATE " + spec.Collation
	}

	statements := []driver.DatabaseStatement{{SQL: statement}}

	// A comment is an extended property, and it can only be set from inside the database it
	// describes — which is what DatabaseStatement.In is for.
	if spec.Comment != "" {
		statements = append(statements, driver.DatabaseStatement{
			In: spec.Name,
			SQL: "EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = " +
				dialect.QuoteString(spec.Comment),
		})
	}

	return statements
}

func (s *session) CreateDatabase(ctx context.Context, spec driver.DatabaseSpec) error {
	return s.RunAll(ctx, s.CreateDatabaseSQL(spec))
}

// DropDatabase closes whatever is still connected before it drops, because SQL Server refuses to
// drop a database anything is using — including, on some builds, the connection that just listed
// it. Rolling the other sessions back is what every client does here and what the reader means by
// asking for the database to be gone.
func (s *session) DropDatabase(ctx context.Context, name string) error {
	quoted := dialect.QuoteIdent(name)

	alone := "ALTER DATABASE " + quoted + " SET SINGLE_USER WITH ROLLBACK IMMEDIATE"
	if err := s.run(ctx, "", alone); err != nil {
		return err
	}
	return s.run(ctx, "", "DROP DATABASE "+quoted)
}

// DatabaseOptions fills the one list SQL Server has. There are no owners to choose from — a
// database is owned by whoever made it — and no templates, tablespaces or extensions of any kind.
func (s *session) DatabaseOptions(ctx context.Context) (driver.DatabaseOptions, error) {
	const collations = `SELECT name FROM sys.fn_helpcollations() ORDER BY name`

	sorts, err := s.names(ctx, collations)
	if err != nil {
		return driver.DatabaseOptions{}, err
	}
	return driver.DatabaseOptions{Collations: sorts}, nil
}
