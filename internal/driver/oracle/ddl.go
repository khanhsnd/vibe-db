package oracle

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// run is what sqlkit.DDL executes its statements through, and the one place a schema change meets
// this adapter's error map. The schema is already in every statement's table name, so nothing has
// to be selected for it.
func (s *session) run(ctx context.Context, _, statement string) error {
	if _, err := s.db.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

// RenameTable leaves the new name unqualified, because Oracle reads a qualified one as an attempt
// to move the table into another schema and refuses it — which is PostgreSQL's rule as well, under
// a different word for the namespace.
func (s *session) RenameTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" RENAME TO " + dialect.QuoteIdent(name)
	return s.run(ctx, ns.Database, statement)
}

// SetPrimaryKey has to name the constraint it is replacing, which Oracle keeps in `all_constraints`
// rather than deriving from the table — the same arrangement PostgreSQL and SQL Server have, and
// for the same reason: a key here is a constraint with a name of its own.
//
// Dropping it takes its index with it unless told otherwise, which is what a reader means by
// replacing a key and is why nothing here says `KEEP INDEX`.
func (s *session) SetPrimaryKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	columns []string,
) error {
	existing, err := s.primaryKeyName(ctx, ns, table)
	if err != nil {
		return err
	}

	actions := []string{}
	if existing != "" {
		actions = append(actions, "DROP CONSTRAINT "+dialect.QuoteIdent(existing))
	}
	if len(columns) > 0 {
		actions = append(actions,
			"ADD PRIMARY KEY ("+strings.Join(dialect.QuoteAll(columns), ", ")+")")
	}

	// Oracle takes one action to an ALTER TABLE, so the drop and the add are two statements and the
	// order is what matters: the old key has to be gone before the new one can be declared.
	for _, action := range actions {
		statement := "ALTER TABLE " + dialect.Table(ns, table) + " " + action
		if err := s.run(ctx, ns.Database, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) primaryKeyName(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (string, error) {
	const query = `
		SELECT constraint_name
		FROM all_constraints
		WHERE owner = :1 AND table_name = :2 AND constraint_type = 'P'`

	var name string
	err := s.db.QueryRowContext(ctx, query, ns.Database, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", translateError(err)
	}
	return name, nil
}

// CreateStatements has to *describe* the table. Oracle can be asked — `DBMS_METADATA.GET_DDL`
// answers for any object — but what it answers with is the storage as well as the shape: the
// tablespace, the extents, the compression and the logging of the server it came from, none of
// which is the reader's and most of which the server they replay it on will not have. So the shape
// the tree already reads is spelled back out through the same dialect every other statement here
// goes through.
//
// The index behind the primary key is left out: it arrives with the table's own PRIMARY KEY clause,
// and declaring it a second time is a statement the server refuses. So is a function-based index,
// which reports Oracle's own hidden column name where a column would be and could not be replayed.
func (s *session) CreateStatements(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]string, error) {
	structure, err := s.TableStructure(ctx, ns, table)
	if err != nil {
		return nil, err
	}

	spec := sqlkit.TableSpecOf(ns, table, structure.Columns)
	statements := []string{dialect.CreateTable(spec)}
	for _, index := range structure.Indexes {
		if index.Primary || strings.Contains(index.Method, "FUNCTION-BASED") {
			continue
		}
		// The kind the catalogue reported is what the index *is*, and only `BITMAP` is a word a
		// CREATE INDEX takes — and it goes in front of the word INDEX rather than after the table,
		// which is not somewhere this app's speller puts anything. So the kind is dropped and what
		// is written is the ordinary index every one of these already is.
		declared := sqlkit.IndexSpecOf(index)
		declared.Method = ""
		statements = append(statements, dialect.CreateIndex(ns, table, declared))
	}
	for _, key := range structure.ForeignKeys {
		statements = append(statements,
			dialect.AddForeignKey(ns, table, sqlkit.ForeignKeySpecOf(key)))
	}

	return statements, nil
}

// A schema on Oracle is a *user*, and making one is `CREATE USER … IDENTIFIED BY …` — an account
// with a password, a quota and a set of privileges. That is administration rather than the New
// Database dialog's business, and there is nowhere in a DatabaseSpec for a password to go, so this
// engine says it makes none — see `createsDatabases` on the frontend, which is what stops the
// dialog being offered at all.
func (*session) CreateDatabaseSQL(driver.DatabaseSpec) []driver.DatabaseStatement {
	return nil
}

func (*session) CreateDatabase(context.Context, driver.DatabaseSpec) error {
	return domain.Unsupported("making a schema")
}

// Dropping one is `DROP USER … CASCADE`, which removes an account and everything it ever owned.
// The same answer, and rather more emphatically.
func (*session) DropDatabase(context.Context, string) error {
	return domain.Unsupported("dropping a schema")
}

// DatabaseOptions answers with the empty lists it has rather than a failure, for the reason SQLite's
// does: nothing asks this of an engine that makes no databases.
func (*session) DatabaseOptions(context.Context) (driver.DatabaseOptions, error) {
	return driver.DatabaseOptions{}, nil
}

// EmptyTable is `DELETE FROM` here as everywhere, and TruncateTable is Oracle's own — both come
// from sqlkit.DDL. What does not is the referential action on an update: Oracle has `ON DELETE` and
// nothing else, so a constraint asking for one is refused here rather than by the server, which
// would report it as a syntax error against a clause the reader did not know was missing.
func (s *session) CreateForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ForeignKeySpec,
) error {
	if strings.TrimSpace(spec.OnUpdate) != "" {
		return domain.Unsupported("a referential action on update")
	}
	return s.DDL.CreateForeignKey(ctx, ns, table, spec)
}
