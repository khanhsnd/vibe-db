package mysql

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Schema reads every table and column of a database in one query. Views are included,
// because completion cares about what can be selected from, not about storage.
func (s *session) Schema(ctx context.Context, database string) ([]driver.TableSchema, error) {
	const query = `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = ?
		ORDER BY table_name, ordinal_position`

	rows, err := s.db.QueryContext(ctx, query, database)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	tables := []driver.TableSchema{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return nil, translateError(err)
		}
		tables = appendColumn(tables, table, column)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return tables, nil
}

// appendColumn folds the flat (table, column) rows into one entry per table. It only
// looks at the last entry, which the query's ORDER BY makes sufficient.
func appendColumn(tables []driver.TableSchema, table, column string) []driver.TableSchema {
	last := len(tables) - 1
	if last >= 0 && tables[last].Name == table {
		tables[last].Columns = append(tables[last].Columns, column)
		return tables
	}
	return append(tables, driver.TableSchema{Name: table, Columns: []string{column}})
}
