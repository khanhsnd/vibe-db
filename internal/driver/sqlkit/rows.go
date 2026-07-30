package sqlkit

import (
	"context"
	"database/sql"
	"strings"

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

// Read runs a row-returning statement. A maxRows of zero or less reads every row.
func Read(
	ctx context.Context,
	q Queryer,
	statement string,
	maxRows int,
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

	values, truncated, err := ScanRows(rows, len(columns), maxRows)
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
func ScanRows(rows *sql.Rows, width, maxRows int) ([][]*string, bool, error) {
	holders := make([]sql.NullString, width)
	targets := make([]any, width)
	for i := range holders {
		targets[i] = &holders[i]
	}

	out := [][]*string{}
	for rows.Next() {
		if maxRows > 0 && len(out) == maxRows {
			return out, true, nil
		}

		if err := rows.Scan(targets...); err != nil {
			return nil, false, err
		}

		row := make([]*string, width)
		for i, holder := range holders {
			row[i] = TextOrNil(holder)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

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

func PrimaryKeyNames(columns []driver.Column) []string {
	names := []string{}
	for _, column := range columns {
		if column.PrimaryKey {
			names = append(names, column.Name)
		}
	}
	return names
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
