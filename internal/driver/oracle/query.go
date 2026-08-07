package oracle

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// rowReturning lists the statements Oracle answers with a result set. There are two: it has no
// SHOW, no TABLE and no bare VALUES, and its EXPLAIN writes to a table rather than answering with
// anything — see plan.go.
var rowReturning = map[string]bool{
	"select": true,
	"with":   true,
}

func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	statement := sqlkit.TrimStatement(query.SQL)
	if statement == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}

	// A pooled connection is pinned for the call so the current schema applies to the statement,
	// and — for an explained one — so the plan is read back in the session that wrote it.
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

func (s *session) execute(
	ctx context.Context,
	conn *sql.Conn,
	query driver.Query,
	statement string,
) (driver.QueryResult, error) {
	if !returnsRows(statement) {
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

// returnsRows decides which of the two paths a statement takes. An INSERT, UPDATE or DELETE answers
// through a RETURNING clause only into *bound variables* here rather than as a result set, so
// unlike PostgreSQL the leading keyword settles it — which is MySQL's arrangement.
func returnsRows(statement string) bool {
	return rowReturning[sqlkit.LeadingKeyword(statement)]
}

// attachSource marks a result writable. The grid needs the table to address a row and the primary
// key to identify it, and the wire format carries neither: database/sql reports a column's type but
// not the table it came from. Whatever cannot be proven stays read-only.
//
// A statement can name its own schema, which is what a Namespace's Database holds here.
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

// pinned holds one pooled connection for the call and points its current schema at the one being
// browsed, so an unqualified name in the statement resolves there. It is what `SET search_path` is
// on PostgreSQL and `USE` on the other two, spelled Oracle's way.
func (s *session) pinned(ctx context.Context, ns driver.Namespace) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	if ns.Database == "" {
		return conn, nil
	}

	statement := "ALTER SESSION SET CURRENT_SCHEMA = " + dialect.QuoteIdent(ns.Database)
	if _, err := conn.ExecContext(ctx, statement); err != nil {
		conn.Close()
		return nil, translateError(err)
	}
	return conn, nil
}
