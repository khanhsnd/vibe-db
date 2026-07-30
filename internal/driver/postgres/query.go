package postgres

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// rowReturning lists the statements PostgreSQL answers with a result set whatever else they
// say. Anything else goes through Exec so the affected-row count survives.
var rowReturning = map[string]bool{
	"explain": true,
	"select":  true,
	"show":    true,
	"table":   true,
	"values":  true,
	"with":    true,
}

func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	statement := sqlkit.TrimStatement(query.SQL)
	if statement == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}
	if query.Explain {
		statement = "EXPLAIN " + statement
	}

	// A pooled connection is pinned for the call so the search path applies to the statement.
	conn, err := s.pinned(ctx, query.Database)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer conn.Close()

	if !returnsRows(statement) {
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

// returnsRows decides which of the two paths a statement takes. An INSERT, UPDATE or DELETE
// answers with rows only when it carries a RETURNING clause, so the leading keyword alone
// cannot settle it the way it can for MySQL.
func returnsRows(statement string) bool {
	return rowReturning[sqlkit.LeadingKeyword(statement)] ||
		dialect.HasClause(statement, "returning")
}

// attachSource marks a result writable. The grid needs the table to address a row and the
// primary key to identify it, and the wire format carries neither: database/sql reports a
// column's type but not the table it came from. Whatever cannot be proven stays read-only.
func (s *session) attachSource(
	ctx context.Context,
	fallbackSchema, statement string,
	result *driver.QueryResult,
) {
	found, ok := dialect.ParseSource(statement)
	if !ok {
		return
	}

	schema := found.Namespace
	if schema == "" {
		schema = fallbackSchema
	}
	if schema == "" {
		return
	}

	stored, err := s.columns(ctx, schema, found.Table)
	if err != nil {
		return
	}

	keys := sqlkit.PrimaryKeyNames(stored)
	if len(keys) == 0 || !sqlkit.MergeSchema(result.Columns, stored, keys) {
		return
	}

	result.Database, result.Table = schema, found.Table
}

// pinned holds one pooled connection for the call and points its search path at the chosen
// schema, so an unqualified table name in the statement resolves there. pg_catalog stays
// searchable whatever the path says, so built-in functions still resolve.
func (s *session) pinned(ctx context.Context, schema string) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	if schema == "" {
		return conn, nil
	}

	if _, err := conn.ExecContext(ctx, "SET search_path TO "+dialect.QuoteIdent(schema)); err != nil {
		conn.Close()
		return nil, translateError(err)
	}
	return conn, nil
}
