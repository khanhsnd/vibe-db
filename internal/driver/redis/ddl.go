package redis

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A key space has no shape, so most of `driver.Schema` has nothing to do here — and says so.
//
// **Every one of these is `domain.Unsupported` rather than a statement left for the server to
// refuse**, which is the rule the MongoDB adapter follows for a foreign key and for the same reason:
// a reader answers "this engine has not got one" by not asking again, while a server's rejection
// reads as something that might work next time. What is missing is not missing by accident: a Redis
// database is one of a fixed set the server was started with, it holds exactly one key space, and a
// key space has no columns, no indexes, no constraints and no routines to declare.
//
// Two things here do run. `EmptyTable` is `FLUSHDB` — the one destructive thing a key space *can* be
// asked, and what every other client calls emptying a database — and `AlterStatements` answers with
// nothing at all, which is what a migration between two key spaces correctly amounts to.

// CreateDatabaseSQL previews nothing, because there is nothing to run: see CreateDatabase.
func (*session) CreateDatabaseSQL(driver.DatabaseSpec) []driver.DatabaseStatement {
	return nil
}

// CreateDatabase has nothing to make. A Redis server is started with a fixed number of databases and
// they all exist from that moment, empty or not — the opposite of MongoDB, where a database exists
// only as long as something is in it.
func (*session) CreateDatabase(context.Context, driver.DatabaseSpec) error {
	return domain.Unsupported("creating a database: a Redis server's databases are fixed at start-up")
}

func (*session) DropDatabase(context.Context, string) error {
	return domain.Unsupported("dropping a database: a Redis database cannot stop existing, only be emptied")
}

// DatabaseOptions has no lists to fill, there being no dialog for it to fill.
func (*session) DatabaseOptions(context.Context) (driver.DatabaseOptions, error) {
	return driver.DatabaseOptions{}, nil
}

func (*session) CreateTable(context.Context, driver.TableSpec) error {
	return domain.Unsupported("creating a table: a Redis database holds one key space and nothing else")
}

func (*session) RenameTable(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("renaming a key space")
}

func (*session) DuplicateTable(context.Context, driver.Namespace, string, string, bool) error {
	return domain.Unsupported("duplicating a key space")
}

func (*session) DropTable(context.Context, driver.Namespace, string) error {
	return domain.Unsupported("dropping a key space: emptying it is the only thing there is")
}

// TruncateTable is the other half of a distinction a key space has not got. Emptying and truncating
// are two things on a table store — one deletes the rows and the other resets the storage — and
// `FLUSHDB` is both at once, so offering it twice under two names would be offering the same
// command and pretending they differed.
func (*session) TruncateTable(context.Context, driver.Namespace, string) error {
	return domain.Unsupported("truncating a key space: emptying it is the same command")
}

// EmptyTable is `FLUSHDB`: every key of this database, gone. It is the one destructive thing a key
// space can be asked and it is exactly what every other Redis client calls emptying one.
func (s *session) EmptyTable(ctx context.Context, ns driver.Namespace, table string) error {
	client, err := s.client(ns)
	if err != nil {
		return err
	}
	if !isKeySpace(table) {
		return domain.ErrUnknownTable
	}
	return translateError(client.FlushDB(ctx).Err())
}

// The five column changes. A column of a key space is not declared anywhere — it is one of the same
// five every Redis server has — so there is nothing to add to, alter or drop.
func (*session) AddColumn(context.Context, driver.Namespace, string, driver.ColumnSpec) error {
	return domain.Unsupported("adding a column: a key space has the five columns every one has")
}

func (*session) AlterColumn(
	context.Context,
	driver.Namespace,
	string,
	string,
	driver.ColumnSpec,
) error {
	return domain.Unsupported("altering a column: a key space has the five columns every one has")
}

func (*session) RenameColumn(context.Context, driver.Namespace, string, string, string) error {
	return domain.Unsupported("renaming a column: a key space has the five columns every one has")
}

func (*session) DropColumn(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("dropping a column: a key space has the five columns every one has")
}

// SetPrimaryKey has nothing to set. The key *is* the primary key and cannot be anything else, which
// is the one thing a key space and a MongoDB collection have in common.
func (*session) SetPrimaryKey(context.Context, driver.Namespace, string, []string) error {
	return domain.Unsupported("a primary key other than the key itself")
}

func (*session) CreateIndex(context.Context, driver.Namespace, string, driver.IndexSpec) error {
	return domain.Unsupported("an index: a Redis key is the index")
}

func (*session) DropIndex(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("an index: a Redis key is the index")
}

func (*session) CreateForeignKey(
	context.Context,
	driver.Namespace,
	string,
	driver.ForeignKeySpec,
) error {
	return domain.Unsupported("a foreign key")
}

func (*session) DropForeignKey(context.Context, driver.Namespace, string, string) error {
	return domain.Unsupported("a foreign key")
}

// AlterStatements spells nothing, because none of the seven kinds of change exists here. A migration
// that shows an empty script is a migration with nothing to run, which is the honest answer for two
// key spaces: what differs between them is their rows, and that is `RowStatements`' half.
func (*session) AlterStatements(driver.Namespace, string, []driver.TableChange) []string {
	return nil
}

// DropSource has nothing to drop, whichever kind is asked for. A Lua script is sent with the call
// that runs it and a function library is loaded by name rather than kept as an object of a database,
// so there is no folder for the tree to draw and nothing in it to remove — and a key space has
// neither a view nor a trigger over it.
func (*session) DropSource(context.Context, driver.Namespace, driver.SourceRef) error {
	return domain.Unsupported("a declared object: a Redis database holds keys and nothing else")
}

// A key space declares nothing, so there is no definition to read and none to put back. Both halves
// say so for the reason every other refusal in this file does: a reader answers "this engine has not
// got one" by not asking again.
func (*session) Source(
	context.Context,
	driver.Namespace,
	driver.SourceRef,
) (driver.Source, error) {
	return driver.Source{}, domain.Unsupported("a definition: a Redis key space is not declared")
}

func (*session) SourceStatements(driver.Namespace, driver.SourceSave) []string {
	return nil
}

func (*session) SaveSource(context.Context, driver.Namespace, driver.SourceSave) error {
	return domain.Unsupported("a definition: a Redis key space is not declared")
}

// NewSource has no skeleton to give, a key space being declared by nothing at all.
func (*session) NewSource(driver.Namespace, driver.SourceRef) string {
	return ""
}
