package app

import (
	"context"
	"time"

	"github.com/khanhsnd/vibe-db/internal/compare"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// compareTimeout covers the *whole* of a comparison rather than one read of it: a data compare is
// two shapes and two pages, and four deadlines that each looked reasonable would add up to a wait
// nobody agreed to. It is longer than a query's for the same reason — there are four calls in it.
const compareTimeout = 60 * time.Second

// DefaultCompareRows is how many rows each side of a data comparison reads when the caller names
// no count. A comparison is **bounded on purpose**: both sides are held in memory at once and
// crossing the bridge as one answer, so this is a read of two windows rather than of two tables,
// and the answer says which of them was cut short.
const DefaultCompareRows = 1000

// MigrationBatch is how many rows one statement of a migration carries, for the modes whose
// statement can carry more than one. It is this side's number rather than the caller's because
// what a server will accept is a fact about the server — MySQL's `max_allowed_packet` — and the
// rows here are the ones the reader was just shown rather than a file of unknown width.
const MigrationBatch = 100

// migrationTimeout is what applying a migration is given. It is longer than a schema change's
// because it is *many* of them — every ticked field, index and constraint in one trip — and a
// deadline that cut it half way through would leave the table in neither shape.
const migrationTimeout = 5 * time.Minute

// CompareSide names one side of a comparison. It carries its own session id because the two
// tables need not be on one server — a staging table against the production one is the whole point
// — and its own namespace for the same reason.
type CompareSide struct {
	SessionID string `json:"sessionId"`
	Namespace
	Table string `json:"table"`
}

// CompareStructure lines two tables' shapes up: one entry per field, index and constraint either
// side holds, matched by name and never by position.
func (a *ConnectionAPI) CompareStructure(left, right CompareSide) (StructureDiff, error) {
	if err := domain.RequireNames("table", left.Table, "table", right.Table); err != nil {
		return StructureDiff{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(compareTimeout)
	defer cancel()

	leftShape, err := a.readStructure(ctx, left)
	if err != nil {
		return StructureDiff{}, wrapError(err)
	}
	rightShape, err := a.readStructure(ctx, right)
	if err != nil {
		return StructureDiff{}, wrapError(err)
	}

	return toStructureDiffDTO(compare.CompareStructure(leftShape, rightShape)), nil
}

// CompareData lines two tables' rows up, matched on the key both sides declare — or, where they
// declare none in common, on every column they share, which the answer says so that a reader is
// never shown "no changed rows" as though it meant there were none.
//
// Both reads are **ordered by that key** before they are cut to `limit`, so a bounded compare
// compares comparable windows of the two tables rather than two arbitrary hundreds of rows.
func (a *ConnectionAPI) CompareData(left, right CompareSide, limit int) (DataDiff, error) {
	if err := domain.RequireNames("table", left.Table, "table", right.Table); err != nil {
		return DataDiff{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(compareTimeout)
	defer cancel()

	leftShape, err := a.readStructure(ctx, left)
	if err != nil {
		return DataDiff{}, wrapError(err)
	}
	rightShape, err := a.readStructure(ctx, right)
	if err != nil {
		return DataDiff{}, wrapError(err)
	}

	key := compare.KeyFor(leftShape, rightShape)
	rows := compareRows(limit)

	leftRows, err := a.readCompared(ctx, left, key, rows)
	if err != nil {
		return DataDiff{}, wrapError(err)
	}
	rightRows, err := a.readCompared(ctx, right, key, rows)
	if err != nil {
		return DataDiff{}, wrapError(err)
	}

	diff := compare.CompareRows(leftRows, rightRows, key)
	return toDataDiffDTO(diff, leftRows.Columns), nil
}

// LoadCreateStatements reads a table's own declaration — what would make it again — which is what
// the DDL Diff between two tables is read from. It is the engine's *answer* where the engine has
// one (MySQL's `SHOW CREATE TABLE`) and a description where it has not, so what is compared is as
// close to the truth as that server can be asked for.
func (a *ConnectionAPI) LoadCreateStatements(
	sessionID string,
	ns Namespace,
	table string,
) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}
	if err := domain.RequireName("table", table); err != nil {
		return nil, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	statements, err := schema.CreateStatements(ctx, ns.toDriver(), table)
	if err != nil {
		return nil, wrapError(err)
	}
	return names(statements), nil
}

// PreviewTableChanges is what ApplyTableChanges would run, spelled by the engine and run by
// nothing — the arrangement PreviewCreateDatabase already has, and for the same reason: one list of
// changes goes to both, so the script the reader is shown cannot drift from what happens.
//
// It needs no deadline and touches no server: spelling is lexical, and the session is looked up
// only to find out *which engine* is being spoken to.
func (a *ConnectionAPI) PreviewTableChanges(
	sessionID string,
	ns Namespace,
	table string,
	changes []TableChangeInput,
) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}
	if err := domain.RequireName("table", table); err != nil {
		return nil, wrapError(err)
	}

	return names(schema.AlterStatements(ns.toDriver(), table, toDriverChanges(changes))), nil
}

// ApplyTableChanges runs the changes in the order they were given, and stops at the first that
// fails. Stopping rather than carrying on is the honest outcome: the order is what makes a
// migration work — what is going goes first, what is arriving is made last — so a step that failed
// leaves every step after it meaningless, and the reader is told which one it was.
//
// There is no transaction around it, because neither engine can put one around DDL in a way that
// holds for all of it: MySQL commits implicitly at every statement. What protects the reader is the
// preview, which is why it exists.
func (a *ConnectionAPI) ApplyTableChanges(
	sessionID string,
	ns Namespace,
	table string,
	changes []TableChangeInput,
) error {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return wrapError(err)
	}
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(migrationTimeout)
	defer cancel()

	for _, change := range toDriverChanges(changes) {
		if err := runTableChange(ctx, schema, ns.toDriver(), table, change); err != nil {
			return wrapError(err)
		}
	}
	return nil
}

// runTableChange is the run beside each spelling, and the switch is deliberately the same shape the
// driver's own speller has: a kind that reaches neither is a kind nothing happens for.
func runTableChange(
	ctx context.Context,
	schema driver.Schema,
	ns driver.Namespace,
	table string,
	change driver.TableChange,
) error {
	switch change.Kind {
	case driver.ChangeAddColumn:
		return schema.AddColumn(ctx, ns, table, change.Column)
	case driver.ChangeAlterColumn:
		return schema.AlterColumn(ctx, ns, table, change.Name, change.Column)
	case driver.ChangeDropColumn:
		return schema.DropColumn(ctx, ns, table, change.Name)
	case driver.ChangeCreateIndex:
		return schema.CreateIndex(ctx, ns, table, change.Index)
	case driver.ChangeDropIndex:
		return schema.DropIndex(ctx, ns, table, change.Name)
	case driver.ChangeCreateForeignKey:
		return schema.CreateForeignKey(ctx, ns, table, change.Key)
	case driver.ChangeDropForeignKey:
		return schema.DropForeignKey(ctx, ns, table, change.Name)
	default:
		return domain.ValidationError{Field: "change", Message: "is not a kind this app knows"}
	}
}

// PreviewRowWrite is the data half of the same pair: the statements that would write these rows
// into that table, spelled by `RowStatements` and run by nothing. It is the one way a row is
// spelled anywhere in the app, which is why a migration's rows and an import's are the same
// statements — see driver.Schema.RowStatements.
func (a *ConnectionAPI) PreviewRowWrite(sessionID string, write RowWriteInput) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}
	if err := domain.RequireName("table", write.Table); err != nil {
		return nil, wrapError(err)
	}

	return names(schema.RowStatements(write.Namespace.toDriver(), write.Table, write.toDriver())), nil
}

// ApplyRowWrite runs those same statements, and stops at the first that fails for the reason
// ApplyTableChanges does.
func (a *ConnectionAPI) ApplyRowWrite(sessionID string, write RowWriteInput) error {
	statements, err := a.PreviewRowWrite(sessionID, write)
	if err != nil {
		return err
	}

	ctx, cancel := a.withTimeout(migrationTimeout)
	defer cancel()

	for _, statement := range statements {
		_, err := a.sessions.Query(ctx, sessionID, driver.Query{
			Namespace: write.Namespace.toDriver(),
			SQL:       statement,
		})
		if err != nil {
			return wrapError(err)
		}
	}
	return nil
}

// compareRows is how many rows one side of a comparison reads, and it is deliberately **not**
// driverRows: there is no unbounded answer here. AllRows is safe on a table tab because that read
// is streamed, and a comparison cannot be — it holds both sides at once and crosses the bridge as
// one string — so a count of nothing, or the negative one that means All, is the default rather
// than every row of two tables.
func compareRows(limit int) int {
	if limit <= 0 {
		return DefaultCompareRows
	}
	return limit
}

func (a *ConnectionAPI) readStructure(
	ctx context.Context,
	side CompareSide,
) (driver.TableStructure, error) {
	schema, err := a.sessions.Schema(side.SessionID)
	if err != nil {
		return driver.TableStructure{}, err
	}
	return schema.TableStructure(ctx, side.Namespace.toDriver(), side.Table)
}

// readCompared reads one side's window, ordered by the key so both windows hold the same stretch
// of the two tables. A table with more rows than were read says so, because a comparison of two
// windows must never read as a comparison of two tables.
func (a *ConnectionAPI) readCompared(
	ctx context.Context,
	side CompareSide,
	key compare.Key,
	limit int,
) (compare.Table, error) {
	sort := make([]driver.TableSort, 0, len(key.Columns))
	for _, column := range key.Columns {
		sort = append(sort, driver.TableSort{Column: column})
	}

	page, err := a.sessions.TableRows(ctx, side.SessionID, driver.TablePage{
		Namespace: side.Namespace.toDriver(),
		Table:     side.Table,
		Limit:     limit,
		Sort:      sort,
	})
	if err != nil {
		return compare.Table{}, err
	}

	return compare.Table{
		Columns:   page.Columns,
		Rows:      page.Rows,
		Truncated: page.Total > int64(len(page.Rows)),
	}, nil
}
