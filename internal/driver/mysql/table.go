package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
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

	rows, err := s.selectRows(ctx, buildSelect(page, columns), page.Limit, page.Offset, len(columns))
	if err != nil {
		return driver.TableRows{}, err
	}

	return driver.TableRows{Columns: columns, Rows: rows, Total: total}, nil
}

func (s *session) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	columns, err := s.columns(ctx, edit.Database, edit.Table)
	if err != nil {
		return err
	}

	keys := primaryKeyNames(columns)
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

func (s *session) columns(ctx context.Context, database, table string) ([]driver.Column, error) {
	const query = `
		SELECT column_name, data_type, column_type, is_nullable, column_key, column_default
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`

	rows, err := s.db.QueryContext(ctx, query, database, table)
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
			Default:    toPointer(defaultValue),
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

func (s *session) countRows(ctx context.Context, database, table string) (int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM " + qualify(database, table)

	if err := s.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, translateError(err)
	}
	return total, nil
}

func (s *session) selectRows(
	ctx context.Context,
	query string,
	limit, offset, width int,
) ([][]*string, error) {
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	// The LIMIT clause already bounds the read, so no cap is needed here.
	out, _, err := scanRows(rows, width, 0)
	return out, err
}

func buildSelect(page driver.TablePage, columns []driver.Column) string {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = quoteIdent(column.Name)
	}

	query := "SELECT " + strings.Join(names, ", ") + " FROM " + qualify(page.Database, page.Table)
	if keys := primaryKeyNames(columns); len(keys) > 0 {
		query += " ORDER BY " + strings.Join(quoteAll(keys), ", ")
	}

	return query + " LIMIT ? OFFSET ?"
}

func buildUpdate(edit driver.CellEdit, keys []string) (string, []any) {
	conditions := make([]string, len(keys))
	args := make([]any, 0, len(keys)+1)
	args = append(args, edit.Value)

	for i, key := range keys {
		conditions[i] = quoteIdent(key) + " = ?"
		args = append(args, edit.Key[key])
	}

	query := "UPDATE " + qualify(edit.Database, edit.Table) +
		" SET " + quoteIdent(edit.Column) + " = ?" +
		" WHERE " + strings.Join(conditions, " AND ") +
		" LIMIT 1"

	return query, args
}

func primaryKeyNames(columns []driver.Column) []string {
	names := []string{}
	for _, column := range columns {
		if column.PrimaryKey {
			names = append(names, column.Name)
		}
	}
	return names
}

func quoteAll(names []string) []string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = quoteIdent(name)
	}
	return quoted
}

func qualify(database, table string) string {
	return quoteIdent(database) + "." + quoteIdent(table)
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func toPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}
