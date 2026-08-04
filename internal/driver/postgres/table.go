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
	columns, err := s.columns(ctx, page.Namespace, page.Table)
	if err != nil {
		return driver.TableRows{}, err
	}

	where, err := dialect.Where(page.Filter, columns)
	if err != nil {
		return driver.TableRows{}, err
	}

	// The total is counted through the same filters the rows are read through, or the pager
	// would walk pages of a table the reader has narrowed away.
	total, err := s.countRows(ctx, page.Namespace, page.Table, where)
	if err != nil {
		return driver.TableRows{}, err
	}

	db, err := s.pool(page.Database)
	if err != nil {
		return driver.TableRows{}, err
	}

	query := buildSelect(page, columns, where)

	// The statement itself bounds the read — either through its LIMIT clause or because the
	// caller asked for every row — so no cap is applied here. A page carrying a stream takes its
	// rows as they arrive and the result then carries none.
	result, err := sqlkit.Read(ctx, db, query, 0, page.Stream)
	if err != nil {
		return driver.TableRows{}, translateError(err)
	}

	return driver.TableRows{
		Columns:   columns,
		Rows:      result.Rows,
		Total:     total,
		Statement: query,
	}, nil
}

func (s *session) ColumnValues(ctx context.Context, pick driver.ColumnPick) ([]string, error) {
	columns, err := s.columns(ctx, pick.Namespace, pick.Table)
	if err != nil {
		return nil, err
	}

	query, err := dialect.DistinctValues(
		dialect.Qualify(pick.Schema, pick.Table),
		pick.Column,
		columns,
		pick.Limit,
	)
	if err != nil {
		return nil, err
	}

	db, err := s.pool(pick.Database)
	if err != nil {
		return nil, err
	}

	// The statement's own LIMIT bounds the read, exactly as a page's does, so no cap is applied
	// here and nothing is streamed: the answer is a list somebody scrolls.
	result, err := sqlkit.Read(ctx, db, query, 0, nil)
	if err != nil {
		return nil, translateError(err)
	}
	return sqlkit.FirstColumn(result.Rows), nil
}

func (s *session) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	columns, err := s.columns(ctx, edit.Namespace, edit.Table)
	if err != nil {
		return err
	}

	keys, err := sqlkit.EditableKeys(columns, edit.Key)
	if err != nil {
		return err
	}

	db, err := s.pool(edit.Database)
	if err != nil {
		return err
	}

	query, args := buildUpdate(edit, keys)
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return translateError(err)
	}
	return nil
}

func (s *session) InsertRow(ctx context.Context, insert driver.RowInsert) error {
	columns, err := s.columns(ctx, insert.Namespace, insert.Table)
	if err != nil {
		return err
	}

	db, err := s.pool(insert.Database)
	if err != nil {
		return err
	}

	query, args := buildInsert(insert, columns)
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return translateError(err)
	}
	return nil
}

func (s *session) DeleteRow(ctx context.Context, removal driver.RowDelete) error {
	columns, err := s.columns(ctx, removal.Namespace, removal.Table)
	if err != nil {
		return err
	}

	keys, err := sqlkit.EditableKeys(columns, removal.Key)
	if err != nil {
		return err
	}

	db, err := s.pool(removal.Database)
	if err != nil {
		return err
	}

	query, args := buildDelete(removal, keys)
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return translateError(err)
	}
	return nil
}

// columns reads a table's shape from pg_catalog rather than information_schema, because
// format_type renders the declared type — `character varying(80)`, `numeric(10,2)` — that
// information_schema only offers in pieces.
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

func (s *session) countRows(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	where string,
) (int64, error) {
	db, err := s.pool(ns.Database)
	if err != nil {
		return 0, err
	}

	var total int64
	query := "SELECT COUNT(*) FROM " + dialect.Qualify(ns.Schema, table) + where

	if err := db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, translateError(err)
	}
	return total, nil
}

// buildSelect spells the whole read, its bounds written in rather than bound: a limit and an
// offset are numbers this side counts, never anything a reader typed, so one string is both what
// runs and what the reader is shown — the only arrangement in which the two cannot drift apart.
// A page that asks for every row carries no LIMIT clause at all, which makes its offset moot.
// The WHERE clause is built by the caller, which needs the same one for the count.
func buildSelect(page driver.TablePage, columns []driver.Column, where string) string {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = dialect.QuoteIdent(column.Name)
	}

	query := "SELECT " + strings.Join(names, ", ") +
		" FROM " + dialect.Qualify(page.Schema, page.Table) +
		where +
		dialect.OrderBy(page.Sort, columns)

	if page.Limit <= 0 {
		return query
	}
	return query + " LIMIT " + strconv.Itoa(page.Limit) + " OFFSET " + strconv.Itoa(page.Offset)
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

	query := "UPDATE " + dialect.Qualify(edit.Schema, edit.Table) +
		" SET " + dialect.QuoteIdent(edit.Column) + " = $1" +
		" WHERE " + strings.Join(conditions, " AND ")

	return query, args
}

// buildInsert names only the columns the new row carried, so an untouched column takes its
// server-side default. The column order is the table's own rather than the map's, which is
// unordered. A row that filled in nothing is PostgreSQL's DEFAULT VALUES form.
func buildInsert(insert driver.RowInsert, columns []driver.Column) (string, []any) {
	names := make([]string, 0, len(insert.Values))
	holders := make([]string, 0, len(insert.Values))
	args := make([]any, 0, len(insert.Values))

	for _, column := range columns {
		value, ok := insert.Values[column.Name]
		if !ok {
			continue
		}
		names = append(names, dialect.QuoteIdent(column.Name))
		holders = append(holders, placeholder(len(args)+1))
		args = append(args, value)
	}

	target := dialect.Qualify(insert.Schema, insert.Table)
	if len(names) == 0 {
		return "INSERT INTO " + target + " DEFAULT VALUES", nil
	}

	return "INSERT INTO " + target +
		" (" + strings.Join(names, ", ") + ")" +
		" VALUES (" + strings.Join(holders, ", ") + ")", args
}

// buildDelete needs no row limit of its own, for the reason buildUpdate does not.
func buildDelete(removal driver.RowDelete, keys []string) (string, []any) {
	conditions := make([]string, len(keys))
	args := make([]any, 0, len(keys))

	for i, key := range keys {
		conditions[i] = dialect.QuoteIdent(key) + " = " + placeholder(len(args)+1)
		args = append(args, removal.Key[key])
	}

	query := "DELETE FROM " + dialect.Qualify(removal.Schema, removal.Table) +
		" WHERE " + strings.Join(conditions, " AND ")

	return query, args
}

func placeholder(position int) string {
	return "$" + strconv.Itoa(position)
}
