package sqlkit

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Queryer is the part of *sql.DB and *sql.Conn a statement needs, so the same read serves
// the pool and a connection pinned to one namespace.
type Queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Exec runs a statement that answers with a count rather than rows.
func Exec(ctx context.Context, q Queryer, statement string, args ...any) (driver.QueryResult, error) {
	outcome, err := q.ExecContext(ctx, statement, args...)
	if err != nil {
		return driver.QueryResult{}, err
	}

	// A driver is allowed not to know the count; the statement still ran.
	affected, err := outcome.RowsAffected()
	if err != nil {
		return driver.QueryResult{Rows: [][]*string{}}, nil
	}
	return driver.QueryResult{Rows: [][]*string{}, RowsAffected: affected}, nil
}

const (
	// BatchRows is how many rows a streamed read hands over at a time: small enough that one
	// batch crosses a bridge comfortably, large enough that a million rows is a few hundred
	// batches.
	BatchRows = 5000

	// CountEvery is how often a streamed read says how far it has got. Twenty times a second is
	// faster than an eye reads a number, and it is a clock rather than a batch so the count is
	// whatever the read has truly reached — not the round number a batch last filled on.
	CountEvery = 50 * time.Millisecond
)

// Read runs a row-returning statement. A maxRows of zero or less reads every row, and a non-nil
// stream takes the rows as they arrive instead of the result carrying them — see ScanRows.
func Read(
	ctx context.Context,
	q Queryer,
	statement string,
	maxRows int,
	stream *driver.RowStream,
	args ...any,
) (driver.QueryResult, error) {
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer rows.Close()

	columns, err := ResultColumns(rows)
	if err != nil {
		return driver.QueryResult{}, err
	}

	values, truncated, err := ScanRows(rows, len(columns), maxRows, stream)
	if err != nil {
		return driver.QueryResult{}, err
	}

	return driver.QueryResult{Columns: columns, Rows: values, Truncated: truncated}, nil
}

// ResultColumns reads what the wire says about a result: a name and a type, but never the
// table a column came from.
func ResultColumns(rows *sql.Rows) ([]driver.Column, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columns := make([]driver.Column, 0, len(types))
	for _, columnType := range types {
		nullable, _ := columnType.Nullable()
		name := columnType.DatabaseTypeName()

		columns = append(columns, driver.Column{
			Name:     columnType.Name(),
			Type:     name,
			FullType: name,
			Nullable: nullable,
		})
	}
	return columns, nil
}

// ScanRows reads every column as text so one grid renders any type. A maxRows of
// zero or less reads to the end; otherwise it stops at the cap and reports that
// the server had more to give.
//
// A non-nil stream is handed the rows every BatchRows and the count every CountEvery, and the
// result then carries no rows at all: that is how a read of a million rows is delivered while it
// runs rather than in one piece at the end, and putting them back together is the caller's
// business.
//
// However the read ends — finished, capped, failed, or stopped — the part-filled batch and the
// exact count go over before it returns. **What was handed over is therefore always every row it
// counted**, so a read stopped at 60 031 rows delivers 60 031 and not the 60 000 its last full
// batch happened to end on.
func ScanRows(
	rows *sql.Rows,
	width, maxRows int,
	stream *driver.RowStream,
) ([][]*string, bool, error) {
	holders := make([]sql.NullString, width)
	targets := make([]any, width)
	for i := range holders {
		targets[i] = &holders[i]
	}

	out := [][]*string{}
	fetched := 0
	counted := time.Now()

	// A streamed read keeps none of what it hands over, so it costs one batch of memory here
	// rather than the whole table.
	hand := func() {
		if stream != nil && len(out) > 0 {
			stream.Rows(out)
			out = [][]*string{}
		}
	}

	// The count is its own errand, on its own clock, for the reason RowStream gives: handing it
	// over with a batch would step it in thousands instead of climbing it.
	count := func() {
		if stream != nil {
			stream.Counted(fetched)
			counted = time.Now()
		}
	}

	for rows.Next() {
		if maxRows > 0 && fetched == maxRows {
			hand()
			count()
			return out, true, nil
		}

		if err := rows.Scan(targets...); err != nil {
			hand()
			count()
			return nil, false, err
		}

		row := make([]*string, width)
		for i, holder := range holders {
			row[i] = TextOrNil(holder)
		}
		out = append(out, row)
		fetched++

		if len(out) == BatchRows {
			hand()
		}
		if stream != nil && time.Since(counted) >= CountEvery {
			count()
		}
	}
	// This is where a stopped read comes out, and it has produced every row it counted.
	if err := rows.Err(); err != nil {
		hand()
		count()
		return nil, false, err
	}

	hand()
	count()
	return out, false, nil
}

// MergeSchema reports whether the result carries every primary-key column exactly once,
// and if so copies the stored column facts — key flag, declared type, nullability — onto
// the matching result columns. An aliased or repeated key column fails the test, because
// the row it names can no longer be found.
func MergeSchema(result, stored []driver.Column, keys []string) bool {
	counts := make(map[string]int, len(result))
	for _, column := range result {
		counts[fold(column.Name)]++
	}
	for _, key := range keys {
		if counts[fold(key)] != 1 {
			return false
		}
	}

	byName := make(map[string]driver.Column, len(stored))
	for _, column := range stored {
		byName[fold(column.Name)] = column
	}
	for i := range result {
		if column, ok := byName[fold(result[i].Name)]; ok {
			result[i].FullType = column.FullType
			result[i].Nullable = column.Nullable
			result[i].PrimaryKey = column.PrimaryKey
			result[i].Default = column.Default
		}
	}

	return true
}

// OrderBy renders the ORDER BY clause of a table read: the keys the reader chose, in the order
// they were chosen, so the first decides and the rest break its ties. They take precedence over
// the primary key that otherwise makes a page stable, which is what a read carrying no key of its
// own falls back to. A name the table does not carry is dropped rather than quoted in, so no
// caller can write the clause itself.
func (d Dialect) OrderBy(sort []driver.TableSort, columns []driver.Column) string {
	keys := make([]string, 0, len(sort))
	for _, one := range sort {
		if !named(columns, one.Column) {
			continue
		}
		key := d.QuoteIdent(one.Column)
		if one.Descending {
			key += " DESC"
		}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		keys = d.QuoteAll(PrimaryKeyNames(columns))
	}
	if len(keys) == 0 {
		return ""
	}
	return " ORDER BY " + strings.Join(keys, ", ")
}

// FirstColumn flattens a one-column read into the values it returned, dropping a NULL: every read
// that uses this asks for values a reader picks from, and the absence of a value is not one.
func FirstColumn(rows [][]*string) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) > 0 && row[0] != nil {
			values = append(values, *row[0])
		}
	}
	return values
}

func named(columns []driver.Column, name string) bool {
	for _, column := range columns {
		if column.Name == name {
			return true
		}
	}
	return false
}

func PrimaryKeyNames(columns []driver.Column) []string {
	names := []string{}
	for _, column := range columns {
		if column.PrimaryKey {
			names = append(names, column.Name)
		}
	}
	return names
}

// EditableKeys returns the primary-key columns a write must pin its WHERE clause to, and
// refuses the write unless the caller carried a value for every one of them: a table with
// no key, or a key only partly supplied, names no single stored row.
func EditableKeys(columns []driver.Column, key map[string]string) ([]string, error) {
	keys := PrimaryKeyNames(columns)
	if len(keys) == 0 {
		return nil, domain.ErrTableNotEditable
	}
	for _, name := range keys {
		if _, ok := key[name]; !ok {
			return nil, domain.ErrTableNotEditable
		}
	}
	return keys, nil
}

// AppendColumn folds the flat (table, column) rows of a schema read into one entry per
// table. It only looks at the last entry, which an ordered read makes sufficient.
func AppendColumn(tables []driver.TableSchema, table, column string) []driver.TableSchema {
	last := len(tables) - 1
	if last >= 0 && tables[last].Name == table {
		tables[last].Columns = append(tables[last].Columns, column)
		return tables
	}
	return append(tables, driver.TableSchema{Name: table, Columns: []string{column}})
}

// fold normalises a column name for matching. Neither engine lets two columns of one
// table differ only in case, so lowering is enough to pair a result column with its
// stored definition whatever case the statement spelled it in.
func fold(name string) string {
	return strings.ToLower(name)
}

func TextOrNil(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}
