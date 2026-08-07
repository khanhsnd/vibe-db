package postgres

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// columns reads a table's shape from pg_catalog rather than information_schema, because
// format_type renders the declared type — `character varying(80)`, `numeric(10,2)` — that
// information_schema only offers in pieces. It is what every read and every write of a row begins
// with; see `sqlkit.Data`, which is where those statements are spelled.
func (s *session) columns(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Column, error) {
	const query = `
		SELECT a.attname,
		       t.typname,
		       pg_catalog.format_type(a.atttypid, a.atttypmod),
		       NOT a.attnotnull,
		       COALESCE(a.attnum = ANY (i.indkey), false),
		       pg_catalog.pg_get_expr(d.adbin, d.adrelid)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
		LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		LEFT JOIN pg_catalog.pg_index i ON i.indrelid = a.attrelid AND i.indisprimary
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`

	db, err := s.pool(ns.Database)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, ns.Schema, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var name, dataType, fullType string
		var nullable, primaryKey bool
		var defaultValue sql.NullString

		err := rows.Scan(&name, &dataType, &fullType, &nullable, &primaryKey, &defaultValue)
		if err != nil {
			return nil, translateError(err)
		}

		columns = append(columns, driver.Column{
			Name:       name,
			Type:       dataType,
			FullType:   fullType,
			Nullable:   nullable,
			PrimaryKey: primaryKey,
			Default:    sqlkit.TextOrNil(defaultValue),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	if len(columns) == 0 {
		return nil, domain.ErrUnknownTable
	}

	return columns, nil
}
