package mysql

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// rowReturning lists the statements MySQL answers with a result set. Anything else
// goes through Exec so the affected-row count survives.
var rowReturning = map[string]bool{
	"analyze":  true,
	"call":     true,
	"check":    true,
	"desc":     true,
	"describe": true,
	"explain":  true,
	"select":   true,
	"show":     true,
	"table":    true,
	"values":   true,
	"with":     true,
}

func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	statement := sqlkit.TrimStatement(query.SQL)
	if statement == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}
	if query.Explain {
		statement = "EXPLAIN " + statement
	}

	// A pooled connection is pinned for the call so USE applies to the statement.
	conn, err := s.pinned(ctx, query.Database)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer conn.Close()

	if !rowReturning[sqlkit.LeadingKeyword(statement)] {
		result, err := sqlkit.Exec(ctx, conn, statement)
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return result, nil
	}

	result, err := sqlkit.Read(ctx, conn, statement, query.MaxRows)
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}

	s.attachSource(ctx, query.Database, statement, &result)
	return result, nil
}

// attachSource marks a result writable. The grid needs the table to address a row and the
// primary key to identify it, and the wire format carries neither: database/sql reports a
// column's type but not the table it came from. Whatever cannot be proven stays read-only.
func (s *session) attachSource(
	ctx context.Context,
	fallbackDatabase, statement string,
	result *driver.QueryResult,
) {
	found, ok := dialect.ParseSource(statement)
	if !ok {
		return
	}

	database := found.Namespace
	if database == "" {
		database = fallbackDatabase
	}
	if database == "" {
		return
	}

	stored, err := s.columns(ctx, database, found.Table)
	if err != nil {
		return
	}

	keys := sqlkit.PrimaryKeyNames(stored)
	if len(keys) == 0 || !sqlkit.MergeSchema(result.Columns, stored, keys) {
		return
	}

	result.Database, result.Table = database, found.Table
}

func (s *session) pinned(ctx context.Context, database string) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	if database == "" {
		return conn, nil
	}

	if _, err := conn.ExecContext(ctx, "USE "+dialect.QuoteIdent(database)); err != nil {
		conn.Close()
		return nil, translateError(err)
	}
	return conn, nil
}
