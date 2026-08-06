package mysql

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

	query := buildSelect(page, columns, where)

	// The statement itself bounds the read — either through its LIMIT clause or because the
	// caller asked for every row — so no cap is applied here. A page carrying a stream takes its
	// rows as they arrive and the result then carries none.
	result, err := sqlkit.Read(ctx, s.db, query, 0, page.Stream)
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
		dialect.Qualify(pick.Database, pick.Table),
		pick.Column,
		columns,
		pick.Limit,
	)
	if err != nil {
		return nil, err
	}

	// The statement's own LIMIT bounds the read, exactly as a page's does, so no cap is applied
	// here and nothing is streamed: the answer is a list somebody scrolls.
	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
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

	query, args := buildUpdate(edit, keys)
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return translateError(err)
	}
	return nil
}

func (s *session) InsertRow(ctx context.Context, insert driver.RowInsert) error {
	columns, err := s.columns(ctx, insert.Namespace, insert.Table)
	if err != nil {
		return err
	}

	query, args := buildInsert(insert, columns)
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
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

	query, args := buildDelete(removal, keys)
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
	// `extra` is read for one word: MySQL reports a *literal* default as the bare value —
	// `x`, not `'x'` — and only marks an expression one DEFAULT_GENERATED. Everywhere else in the
	// app a `Column.Default` is an expression, because that is what goes back into a statement, so
	// the literal is quoted here rather than left to reach one as a bare identifier.
	const query = `
		SELECT column_name, data_type, column_type, is_nullable, column_key, column_default, extra
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
		var name, dataType, fullType, nullable, key, extra string
		var defaultValue sql.NullString

		err := rows.Scan(&name, &dataType, &fullType, &nullable, &key, &defaultValue, &extra)
		if err != nil {
			return nil, translateError(err)
		}

		columns = append(columns, driver.Column{
			Name:       name,
			Type:       dataType,
			FullType:   fullType,
			Nullable:   nullable == "YES",
			PrimaryKey: key == "PRI",
			Default:    defaultExpression(dataType, defaultValue, extra),
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

// generatedDefault is the word MySQL puts in a column's `extra` when its default is an expression
// rather than a value. It is the only way the two can be told apart: `information_schema` reports
// `now()` and `now` identically otherwise.
const generatedDefault = "DEFAULT_GENERATED"

// defaultExpression is a stored default read back as the expression the rest of the app takes it
// for — which is what has to go into an `ALTER TABLE`, a dump and a migration's script.
//
// MySQL reports a **literal** default as the bare value, so `DEFAULT 'x'` comes back as `x`; put
// straight back into a statement that is an identifier and the server refuses it. An expression
// default comes back already spelled and is marked DEFAULT_GENERATED, so it travels verbatim.
//
// A numeric or a temporal column's literal needs no quoting and gets none: `0` is the number and
// `CURRENT_TIMESTAMP` on a timestamp column is reported without the marker but is not a string.
func defaultExpression(dataType string, value sql.NullString, extra string) *string {
	if !value.Valid {
		return nil
	}
	if strings.Contains(extra, generatedDefault) || !quotesDefault(dataType) {
		return sqlkit.TextOrNil(value)
	}

	quoted := dialect.QuoteString(value.String)
	return &quoted
}

// quotesDefault says a literal default of this type is written as a string. It is the text and
// binary types and nothing else: a number, a date and a boolean are all written bare, and so is
// anything an engine keeps as an expression.
func quotesDefault(dataType string) bool {
	switch strings.ToLower(dataType) {
	case "char", "varchar", "text", "tinytext", "mediumtext", "longtext",
		"enum", "set", "json",
		"binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob":
		return true
	default:
		return false
	}
}

func (s *session) countRows(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	where string,
) (int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM " + dialect.Qualify(ns.Database, table) + where

	if err := s.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
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
		" FROM " + dialect.Qualify(page.Database, page.Table) +
		where +
		dialect.OrderBy(page.Sort, columns)

	if page.Limit <= 0 {
		return query
	}
	return query + " LIMIT " + strconv.Itoa(page.Limit) + " OFFSET " + strconv.Itoa(page.Offset)
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

// buildInsert names only the columns the new row carried, so an untouched column takes its
// server-side default. The column order is the table's own rather than the map's, which is
// unordered. A row that filled in nothing is MySQL's empty-column form.
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
		holders = append(holders, "?")
		args = append(args, value)
	}

	target := dialect.Qualify(insert.Database, insert.Table)
	if len(names) == 0 {
		return "INSERT INTO " + target + " () VALUES ()", nil
	}

	return "INSERT INTO " + target +
		" (" + strings.Join(names, ", ") + ")" +
		" VALUES (" + strings.Join(holders, ", ") + ")", args
}

func buildDelete(removal driver.RowDelete, keys []string) (string, []any) {
	conditions := make([]string, len(keys))
	args := make([]any, 0, len(keys))

	for i, key := range keys {
		conditions[i] = dialect.QuoteIdent(key) + " = ?"
		args = append(args, removal.Key[key])
	}

	query := "DELETE FROM " + dialect.Qualify(removal.Database, removal.Table) +
		" WHERE " + strings.Join(conditions, " AND ") +
		" LIMIT 1"

	return query, args
}
