package session

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// readOnly is the whole of what a read-only connection means, and it is **one place** on purpose:
// the session the manager hands out is already this, so every half of it the rest of the app reaches
// for — `Schema`, `Security`, `Monitor`, the rows, the query — is the guarded one, and there is no
// second seam a change could go round.
//
// It embeds the real session, so a read passes straight through and only what writes is written out
// here. What that costs is a method added to `driver.Session` later being carried silently: what
// answers for it is `readonly_test.go`, which walks the interface and asks of every method whether
// it is refused — a new one shows up there as unclassified and the test says so.
//
// Three kinds of method are deliberately *not* refused:
//
//   - **A speller.** `AlterStatements`, `SourceStatements`, `AccountStatements`, `ProcessStatements`,
//     `CallStatements`, `RowStatements`, `CreateStatements`, `CreateDatabaseSQL`, `NewSource` all run
//     nothing. A reader who cannot apply a change can still be shown the statements that would make
//     it, which is most of what a read-only connection is *for*.
//   - **A transaction.** Nothing inside one can write, so `Begin`, `Commit` and `Rollback` are left
//     alone: on the two engines that offer it, a transaction is how a reader holds a consistent view
//     across several reads.
//   - **A read**, however expensive.
type readOnly struct {
	driver.Session
	// text is the engine's own reviewer, and is what decides a typed statement. It is the same one
	// the editor is underlined from, so what the guard refuses and what the reader was warned about
	// are one answer.
	text driver.Text
}

// refuse names what was not sent, in the reader's own words for it. Nothing reached the server:
// that is the difference between this and every other failure the app reports.
func refuse(what string) error {
	return domain.ReadOnly(what)
}

// Query is the one method here that is neither plainly a read nor plainly a write, because what it
// does is whatever the reader typed. It is answered by the **engine's own reviewer** — the same
// `driver.Text` the editor underlines from — so the guard and the warning cannot disagree, and a
// script is refused whole the moment one of its statements would write.
func (s readOnly) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	if err := s.reviewed(query.SQL); err != nil {
		return driver.QueryResult{}, err
	}
	return s.Session.Query(ctx, query)
}

// reviewed refuses a script holding anything that writes, naming the first such statement by its own
// leading word. A session with no reviewer refuses everything rather than guessing: a guard that
// cannot read what it is guarding is not a guard.
func (s readOnly) reviewed(script string) error {
	if s.text == nil {
		return refuse("running a statement")
	}

	for _, one := range s.text.Review(script).Statements {
		if domain.AccessReadOnly.Refuses(one.Hazard, one.Keyword) {
			return refuse(statementName(one.Keyword))
		}
	}
	return nil
}

// statementName is what the reader is told was refused. It is the statement's own first word, which
// is the word they typed — and "that statement" for a chunk with no leading word at all.
func statementName(keyword string) string {
	if strings.TrimSpace(keyword) == "" {
		return "that statement"
	}
	return strings.ToUpper(keyword)
}

// The rows. These three are what the grid writes with, and are the ones a reader meets first.

func (s readOnly) UpdateCell(context.Context, driver.CellEdit) error {
	return refuse("changing a value")
}

func (s readOnly) InsertRow(context.Context, driver.RowInsert) error {
	return refuse("inserting a row")
}

func (s readOnly) DeleteRow(context.Context, driver.RowDelete) error {
	return refuse("deleting a row")
}

// The schema. Every one of these has a spelling beside it that is *not* refused, so a migration can
// still be previewed on a connection that will not run it.

func (s readOnly) CreateDatabase(context.Context, driver.DatabaseSpec) error {
	return refuse("CREATE DATABASE")
}

func (s readOnly) DropDatabase(context.Context, string) error {
	return refuse("DROP DATABASE")
}

func (s readOnly) CreateTable(context.Context, driver.TableSpec) error {
	return refuse("CREATE TABLE")
}

func (s readOnly) RenameTable(context.Context, driver.Namespace, string, string) error {
	return refuse("renaming a table")
}

func (s readOnly) DuplicateTable(context.Context, driver.Namespace, string, string, bool) error {
	return refuse("duplicating a table")
}

func (s readOnly) DropTable(context.Context, driver.Namespace, string) error {
	return refuse("DROP TABLE")
}

func (s readOnly) TruncateTable(context.Context, driver.Namespace, string) error {
	return refuse("TRUNCATE")
}

func (s readOnly) EmptyTable(context.Context, driver.Namespace, string) error {
	return refuse("emptying a table")
}

func (s readOnly) AddColumn(context.Context, driver.Namespace, string, driver.ColumnSpec) error {
	return refuse("adding a column")
}

func (s readOnly) AlterColumn(
	context.Context, driver.Namespace, string, string, driver.ColumnSpec,
) error {
	return refuse("altering a column")
}

func (s readOnly) RenameColumn(context.Context, driver.Namespace, string, string, string) error {
	return refuse("renaming a column")
}

func (s readOnly) DropColumn(context.Context, driver.Namespace, string, string) error {
	return refuse("dropping a column")
}

func (s readOnly) SetPrimaryKey(context.Context, driver.Namespace, string, []string) error {
	return refuse("changing a primary key")
}

func (s readOnly) CreateIndex(
	context.Context, driver.Namespace, string, driver.IndexSpec,
) error {
	return refuse("creating an index")
}

func (s readOnly) DropIndex(context.Context, driver.Namespace, string, string) error {
	return refuse("dropping an index")
}

func (s readOnly) CreateForeignKey(
	context.Context, driver.Namespace, string, driver.ForeignKeySpec,
) error {
	return refuse("creating a foreign key")
}

func (s readOnly) DropForeignKey(context.Context, driver.Namespace, string, string) error {
	return refuse("dropping a foreign key")
}

func (s readOnly) AddPartition(
	context.Context, driver.Namespace, string, driver.PartitionSpec,
) error {
	return refuse("adding a partition")
}

func (s readOnly) DropPartition(context.Context, driver.Namespace, string, string) error {
	return refuse("dropping a partition")
}

func (s readOnly) TruncatePartition(context.Context, driver.Namespace, string, string) error {
	return refuse("truncating a partition")
}

func (s readOnly) RefreshMaterializedView(context.Context, driver.Namespace, string) error {
	return refuse("refreshing a materialized view")
}

// A definition. Both of these are named by the *kind* they are about, since one machinery serves
// seven of them and "dropping a definition" would not tell the reader which one was refused.

func (s readOnly) DropSource(_ context.Context, _ driver.Namespace, ref driver.SourceRef) error {
	return refuse("DROP " + ref.Kind.Keyword())
}

func (s readOnly) SaveSource(_ context.Context, _ driver.Namespace, save driver.SourceSave) error {
	return refuse("saving a " + strings.ToLower(save.Replaces.Kind.Keyword()))
}

// A routine is the one declared object that is *called*, and a call is refused: its body is a
// program this side has not read and cannot vouch for, so it is a write whatever it turns out to do.
func (s readOnly) CallRoutine(
	context.Context, driver.Namespace, driver.RoutineCall,
) (driver.QueryResult, error) {
	return driver.QueryResult{}, refuse("calling a routine")
}

// The accounts and the server. Both lists are still read: what a read-only connection holds back is
// the change, never the answer.

func (s readOnly) ApplyAccountChanges(context.Context, []driver.AccountChange) error {
	return refuse("changing an account")
}

func (s readOnly) KillProcesses(context.Context, []driver.ProcessKill) error {
	return refuse("ending a server process")
}
