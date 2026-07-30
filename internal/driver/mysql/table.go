package mysql

import (
	"context"
	"database/sql"
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

	total, err := s.countRows(ctx, page.Namespace, page.Table)
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
	columns, err := s.columns(ctx, edit.Namespace, edit.Table)
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

func (s *session) columns(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Column, error) {
	const query = `
		SELECT column_name, data_type, column_type, is_nullable, column_key, column_default
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var name, dataType, fullType, nullable, key string
		var defaultValue sql.NullString

		if err := rows.Scan(&name, &dataType, &fullType, &nullable, &key, &defaultValue); err != nil {
			return nil, translateError(err)
		}

		columns = append(columns, driver.Column{
			Name:       name,
			Type:       dataType,
			FullType:   fullType,
			Nullable:   nullable == "YES",
			PrimaryKey: key == "PRI",
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
) (int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM " + dialect.Qualify(ns.Database, table)

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
	return query + " LIMIT ? OFFSET ?", []any{page.Limit, page.Offset}
}

func buildUpdate(edit driver.CellEdit, keys []string) (string, []any) {
	conditions := make([]string, len(keys))
	args := make([]any, 0, len(keys)+1)
	args = append(args, edit.Value)

	for i, key := range keys {
		conditions[i] = dialect.QuoteIdent(key) + " = ?"
		args = append(args, edit.Key[key])
	}

	query := "UPDATE " + dialect.Qualify(edit.Database, edit.Table) +
		" SET " + dialect.QuoteIdent(edit.Column) + " = ?" +
		" WHERE " + strings.Join(conditions, " AND ") +
		" LIMIT 1"

	return query, args
}
