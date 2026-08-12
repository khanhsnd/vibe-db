package sqlserver

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// rowReturning lists the statements SQL Server answers with a result set. `exec` is among them
// because a procedure usually selects something, and a procedure that does not simply answers with
// no rows — where sending one down the other path would lose whatever it did return.
var rowReturning = map[string]bool{
	"exec":    true,
	"execute": true,
	"select":  true,
	"values":  true,
	"with":    true,
}

func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	statement := sqlkit.TrimStatement(query.SQL)
	if statement == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}

	// A pooled connection is pinned for the call so USE applies to the statement.
	conn, release, err := s.Pinned(ctx, query.Namespace)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer release()

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
	conn sqlkit.Queryer,
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
// with rows only when it carries an OUTPUT clause, so the leading keyword alone cannot settle it —
// the same problem PostgreSQL's RETURNING poses, under another name.
func returnsRows(statement string) bool {
	return rowReturning[sqlkit.LeadingKeyword(statement)] ||
		dialect.HasClause(statement, "output")
}

// attachSource marks a result writable. The grid needs the table to address a row and the primary
// key to identify it, and the wire format carries neither: database/sql reports a column's type but
// not the table it came from. Whatever cannot be proven stays read-only.
//
// A statement here can name its own schema *and* its own database, but the second is ignored: the
// connection was pinned to a database above, and a read of another one is a read the pinned
// namespace cannot address a row in.
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
		ns.Schema = found.Namespace
	}
	if ns.Database == "" || ns.Schema == "" {
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

// pinned holds one pooled connection for the call and puts it in the chosen database, so an
// unqualified name in the statement resolves there. It is the one place this adapter needs a USE at
// all: every statement it writes itself names its database already.
//
// The default schema is left alone. SQL Server resolves an unqualified name against the login's own
// default schema, and changing that is a server-side setting rather than something a session says.
func (s *session) pinned(ctx context.Context, ns driver.Namespace) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}

	database := ns.Database
	if database == "" {
		database = s.home
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
