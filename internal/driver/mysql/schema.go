package mysql

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// TableSchemas reads every table and column of a database in one query. Views are included,
// because completion cares about what can be selected from, not about storage.
func (s *session) TableSchemas(
	ctx context.Context,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	const query = `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = ?
		ORDER BY table_name, ordinal_position`

	rows, err := s.db.QueryContext(ctx, query, ns.Database)
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
		tables = sqlkit.AppendColumn(tables, table, column)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return tables, nil
}
