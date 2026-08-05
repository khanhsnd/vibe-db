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

	// A pooled connection is pinned for the call so USE applies to the statement.
	conn, err := s.pinned(ctx, query.Namespace)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer conn.Close()

	// A plan is read **before** the statement runs. Asking afterwards would ask about a server the
	// statement has already changed: what `DELETE FROM t` was going to do is not what the optimizer
	// says it would do once the rows are gone.
	var plan *driver.Plan
	if query.Explain {
		plan = explainPlan(ctx, conn, statement)
	}

	result, err := s.execute(ctx, conn, query, statement)
	if err != nil {
		return driver.QueryResult{}, err
	}

	result.Plan = plan
	return result, nil
}

// execute runs the statement down whichever of the two paths it belongs on: the one that answers
// with rows, or the one that answers with a count.
func (s *session) execute(
	ctx context.Context,
	conn *sql.Conn,
	query driver.Query,
	statement string,
) (driver.QueryResult, error) {
	if !rowReturning[sqlkit.LeadingKeyword(statement)] {
		result, err := sqlkit.Exec(ctx, conn, statement)
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return result, nil
	}

	result, err := sqlkit.Read(ctx, conn, statement, query.MaxRows, nil)
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}

	s.attachSource(ctx, query.Namespace, statement, &result)
	return result, nil
}

// attachSource marks a result writable. The grid needs the table to address a row and the
// primary key to identify it, and the wire format carries neither: database/sql reports a
// column's type but not the table it came from. Whatever cannot be proven stays read-only.
func (s *session) attachSource(
	ctx context.Context,
	ns driver.Namespace,
	statement string,
	result *driver.QueryResult,
) {
	found, ok := dialect.ParseSource(statement)
	if !ok {
		return
	}

	if found.Namespace != "" {
		ns.Database = found.Namespace
	}
	if ns.Database == "" {
		return
	}

	stored, err := s.columns(ctx, ns, found.Table)
	if err != nil {
		return
	}

	keys := sqlkit.PrimaryKeyNames(stored)
	if len(keys) == 0 || !sqlkit.MergeSchema(result.Columns, stored, keys) {
		return
	}

	result.Namespace, result.Table = ns, found.Table
}

func (s *session) pinned(ctx context.Context, ns driver.Namespace) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	if ns.Database == "" {
		return conn, nil
	}

	if _, err := conn.ExecContext(ctx, "USE "+dialect.QuoteIdent(ns.Database)); err != nil {
		conn.Close()
		return nil, translateError(err)
	}
	return conn, nil
}
