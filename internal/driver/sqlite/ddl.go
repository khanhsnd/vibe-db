package sqlite

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// This file is what SQLite has not got, and one thing it has that nothing else does.
//
// A SQLite database is the text of the statements that made it, kept and re-parsed. That is why so
// much here is `domain.Unsupported`: there is no `ALTER TABLE … ALTER COLUMN`, no way to add or
// drop a constraint after the fact, and no second database to make or drop. Every one of them says
// which thing is missing rather than being left for the engine to refuse, and none of them is
// approximated by rebuilding the table behind the reader's back — a rebuild drops the triggers,
// the views and the constraints that pointed at it, which is not what "change this column" means.

// run is what sqlkit.DDL executes its statements through, and the one place a schema change meets
// this adapter's error map. There is one namespace, so nothing has to be selected for it.
func (s *session) run(ctx context.Context, _, statement string) error {
	if _, err := s.db.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

// CreateStatements asks SQLite for its own answer, because SQLite has one and it is exact: the
// file *stores* the statement that made each object. So a dump is the declaration the reader wrote,
// its constraints and its collations and its `WITHOUT ROWID` included, rather than a description
// put back together out of pragmas.
//
// The indexes are the same read: an index SQLite made for a `UNIQUE` column has no statement of its
// own — `sql` is NULL for it — and is left out, since the column's declaration already carries it.
//
// The **kinds are named** rather than everything under `tbl_name` being taken, because a trigger is
// filed under the table it fires on and is not part of that table's declaration: a dump is of
// tables, and a trigger is edited as [its own definition](../../../docs/features/definitions.md).
func (s *session) CreateStatements(
	ctx context.Context,
	_ driver.Namespace,
	table string,
) ([]string, error) {
	const query = `
		SELECT sql
		FROM sqlite_master
		WHERE tbl_name = ? AND type IN ('table', 'index') AND sql IS NOT NULL
		ORDER BY type = 'table' DESC, name`

	statements, err := s.names(ctx, query, table)
	if err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, domain.ErrUnknownTable
	}
	return statements, nil
}

// RenameTable and RenameColumn are the two changes SQLite does make, and both are ordinary ALTER
// TABLE statements — see sqlkit.DDL, which spells the second. This one is here because the target
// is never qualified: `main` is the only namespace and naming it is refused.
func (s *session) RenameTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" RENAME TO " + dialect.QuoteIdent(name)
	return s.run(ctx, ns.Database, statement)
}

// TruncateTable is not a faster DELETE here, it is nothing at all: SQLite has no such statement,
// and the optimisation that makes `DELETE FROM t` drop the pages wholesale happens by itself. So
// the two are one command and the engine says which it has — see `hasTruncate` on the frontend.
func (*session) TruncateTable(context.Context, driver.Namespace, string) error {
	return domain.Unsupported("truncating a table")
}

// A column's type, its nullability and its default are all in the text of the CREATE TABLE, and
// SQLite will not re-parse that text on request. That is a *dialect* fact rather than an override
// here — `sqlkit.Dialect.KeepsColumnAsDeclared` — so the change spells nothing, a migration shows
// an empty script rather than one that would fail, and the run of it answers Unsupported.

// A key is declared with the table and cannot be moved afterwards. It is read and shown — see
// `indexes` — and setting one is a rebuild this adapter will not do behind the reader's back.
func (*session) SetPrimaryKey(context.Context, driver.Namespace, string, []string) error {
	return domain.Unsupported("changing a primary key")
}

// A referential constraint is likewise part of the declaration. Both halves say so, so a reader
// who can *see* the foreign keys of a table is told plainly why they cannot add one.
func (*session) CreateForeignKey(
	context.Context,
	driver.Namespace,
	string,
	driver.ForeignKeySpec,
) error {
	return domain.Unsupported("adding a foreign key")
}

func (*session) DropForeignKey(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("dropping a foreign key")
}

// A database here is a file, and making one is a matter for the file system rather than for a
// statement. The frontend does not offer either — see `createsDatabases` — and these are what a
// caller that asked anyway is told.
func (*session) CreateDatabaseSQL(driver.DatabaseSpec) []driver.DatabaseStatement {
	return nil
}

func (*session) CreateDatabase(context.Context, driver.DatabaseSpec) error {
	return domain.Unsupported("making a database")
}

func (*session) DropDatabase(context.Context, string) error {
	return domain.Unsupported("dropping a database")
}

// DatabaseOptions answers with the empty lists it has rather than a failure: nothing asks this of
// an engine that makes no databases, and a caller that did is better given nothing to choose from
// than an error about a dialog it never opened.
func (*session) DatabaseOptions(context.Context) (driver.DatabaseOptions, error) {
	return driver.DatabaseOptions{}, nil
}

// A function in SQLite belongs to the program that opened the file — this one registers `regexp`,
// and it is gone the moment the file is closed. There is nothing stored to drop, so a routine is
// the one kind this refuses; the other three are objects of the file like any table.
func (s *session) DropSource(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) error {
	if ref.Kind == driver.SourceFunction || ref.Kind == driver.SourceProcedure {
		return domain.Unsupported("a stored routine")
	}
	return s.DDL.DropSource(ctx, ns, ref)
}

// RoutineParams has nothing to read for the reason DropSource has nothing to drop: a function here
// is the opening program's and is gone with it, so there is no stored routine to be told what it
// takes. The call itself spells nothing at all — see `sqlkit.CallNone`.
func (s *session) RoutineParams(
	context.Context,
	driver.Namespace,
	driver.SourceRef,
) ([]driver.RoutineParam, error) {
	return nil, domain.Unsupported("a stored routine")
}

// Source is the same read CreateStatements is, and is exact for the same reason: the file stores
// the statement that made the view or the trigger, so what a reader is shown is the declaration
// they wrote rather than a description of it.
//
// A routine is refused here rather than read as nothing, since there is none to read — see above.
func (s *session) Source(
	ctx context.Context,
	_ driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	// A SQLite file holds tables, views, indexes and triggers and nothing else, so every other kind
	// is named back rather than looked for: `sqlite_master` would simply answer with no row, which
	// reads as "it is not there" when the truth is that it could not be.
	if ref.Kind != driver.SourceView && ref.Kind != driver.SourceTrigger {
		return driver.Source{}, domain.Unsupported(string(ref.Kind))
	}

	const query = `
		SELECT sql
		FROM sqlite_master
		WHERE type = ? AND name = ? AND sql IS NOT NULL`

	// `sqlite_master.type` is spelled the way a kind already is — `view`, `trigger` — so the kind
	// goes into the read as itself rather than through a map of two entries.
	found, err := s.names(ctx, query, string(ref.Kind), ref.Name)
	if err != nil {
		return driver.Source{}, err
	}
	if len(found) == 0 {
		return driver.Source{}, domain.ErrUnknownDefinition
	}
	return driver.Source{SourceRef: ref, Definition: found[0]}, nil
}
