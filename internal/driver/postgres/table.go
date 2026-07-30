package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

func (s *session) TableRows(ctx context.Context, page driver.TablePage) (driver.TableRows, error) {
	columns, err := s.columns(ctx, page.Database, page.Table)
	if err != nil {
		return driver.TableRows{}, err
	}

	total, err := s.countRows(ctx, page.Database, page.Table)
	if err != nil {
		return driver.TableRows{}, err
	}

	query, args := buildSelect(page, columns)

	// The statement itself bounds the read — either through its LIMIT clause or because the
	// caller asked for every row — so no cap is applied here.
	result, err := sqlkit.Read(ctx, s.db, query, 0, args...)
	if err != nil {
		return driver.TableRows{}, translateError(err)
	}

	return driver.TableRows{Columns: columns, Rows: result.Rows, Total: total}, nil
}

func (s *session) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	columns, err := s.columns(ctx, edit.Database, edit.Table)
	if err != nil {
		return err
	}

	keys := sqlkit.PrimaryKeyNames(columns)
	if len(keys) == 0 {
		return domain.ErrTableNotEditable
	}
	for _, key := range keys {
		if _, ok := edit.Key[key]; !ok {
			return domain.ErrTableNotEditable
		}
	}

	query, args := buildUpdate(edit, keys)
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return translateError(err)
	}
	return nil
}

// columns reads a table's shape from pg_catalog rather than information_schema, because
// format_type renders the declared type — `character varying(80)`, `numeric(10,2)` — that
// information_schema only offers in pieces.
func (s *session) columns(ctx context.Context, schema, table string) ([]driver.Column, error) {
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

	rows, err := s.db.QueryContext(ctx, query, schema, table)
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

func (s *session) countRows(ctx context.Context, schema, table string) (int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM " + dialect.Qualify(schema, table)

	if err := s.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, translateError(err)
	}
	return total, nil
}

// buildSelect returns the statement and its arguments together, because a page that asks for
// every row carries no LIMIT clause and therefore no arguments to bind.
func buildSelect(page driver.TablePage, columns []driver.Column) (string, []any) {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = dialect.QuoteIdent(column.Name)
	}

	query := "SELECT " + strings.Join(names, ", ") +
		" FROM " + dialect.Qualify(page.Database, page.Table)
	if keys := sqlkit.PrimaryKeyNames(columns); len(keys) > 0 {
		query += " ORDER BY " + strings.Join(dialect.QuoteAll(keys), ", ")
	}

	if page.Limit <= 0 {
		return query, nil
	}
	return query + " LIMIT $1 OFFSET $2", []any{page.Limit, page.Offset}
}

// buildUpdate needs no row limit of its own: PostgreSQL has no LIMIT on UPDATE, and the
// primary key in the WHERE clause already names at most one row.
func buildUpdate(edit driver.CellEdit, keys []string) (string, []any) {
	conditions := make([]string, len(keys))
	args := make([]any, 0, len(keys)+1)
	args = append(args, edit.Value)

	for i, key := range keys {
		conditions[i] = dialect.QuoteIdent(key) + " = " + placeholder(len(args)+1)
		args = append(args, edit.Key[key])
	}

	query := "UPDATE " + dialect.Qualify(edit.Database, edit.Table) +
		" SET " + dialect.QuoteIdent(edit.Column) + " = $1" +
		" WHERE " + strings.Join(conditions, " AND ")

	return query, args
}

func placeholder(position int) string {
	return "$" + strconv.Itoa(position)
}
