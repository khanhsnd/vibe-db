package postgres

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// TableSchemas reads every table and column of a schema in one query. Views, materialised
// views and foreign tables are included, because completion cares about what can be selected
// from, not about storage.
func (s *session) TableSchemas(
	ctx context.Context,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	const query = `
		SELECT c.relname, a.attname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
		  AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY c.relname, a.attnum`

	db, err := s.pool(ns.Database)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, ns.Schema)
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
