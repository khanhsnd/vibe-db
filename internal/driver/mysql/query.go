package mysql

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// rowReturning lists the statements MySQL answers with a result set. Anything else
// goes through Exec so the affected-row count survives.
var rowReturning = map[string]bool{
	"analyze":  true,
	"call":     true,
	"check":    true,
	"desc":     true,
	"describe": true,
	"explain":  true,
	"select":   true,
	"show":     true,
	"table":    true,
	"values":   true,
	"with":     true,
}

func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	statement := strings.TrimRight(strings.TrimSpace(query.SQL), "; \t\r\n")
	if statement == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}
	if query.Explain {
		statement = "EXPLAIN " + statement
	}

	// A pooled connection is pinned for the call so USE applies to the statement.
	conn, err := s.pinned(ctx, query.Database)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer conn.Close()

	if !rowReturning[leadingKeyword(statement)] {
		return execStatement(ctx, conn, statement)
	}

	result, err := queryStatement(ctx, conn, statement, query.MaxRows)
	if err != nil {
		return driver.QueryResult{}, err
	}

	s.attachSource(ctx, query.Database, statement, &result)
	return result, nil
}

// attachSource marks a result writable. The grid needs the table to address a row and the
// primary key to identify it, and the wire format carries neither: database/sql reports a
// column's type but not the table it came from. Whatever cannot be proven stays read-only.
func (s *session) attachSource(
	ctx context.Context,
	fallbackDatabase, statement string,
	result *driver.QueryResult,
) {
	found, ok := parseSource(statement)
	if !ok {
		return
	}

	database := found.database
	if database == "" {
		database = fallbackDatabase
	}
	if database == "" {
		return
	}

	stored, err := s.columns(ctx, database, found.table)
	if err != nil {
		return
	}

	keys := primaryKeyNames(stored)
	if len(keys) == 0 || !mergeSchema(result.Columns, stored, keys) {
		return
	}

	result.Database, result.Table = database, found.table
}

// mergeSchema reports whether the result carries every primary-key column exactly once,
// and if so copies the stored column facts — key flag, declared type, nullability — onto
// the matching result columns. An aliased or repeated key column fails the test, because
// the row it names can no longer be found.
func mergeSchema(result, stored []driver.Column, keys []string) bool {
	counts := make(map[string]int, len(result))
	for _, column := range result {
		counts[strings.ToLower(column.Name)]++
	}
	for _, key := range keys {
		if counts[strings.ToLower(key)] != 1 {
			return false
		}
	}

	byName := make(map[string]driver.Column, len(stored))
	for _, column := range stored {
		byName[strings.ToLower(column.Name)] = column
	}
	for i := range result {
		if column, ok := byName[strings.ToLower(result[i].Name)]; ok {
			result[i].FullType = column.FullType
			result[i].Nullable = column.Nullable
			result[i].PrimaryKey = column.PrimaryKey
			result[i].Default = column.Default
		}
	}

	return true
}

func (s *session) pinned(ctx context.Context, database string) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	if database == "" {
		return conn, nil
	}

	if _, err := conn.ExecContext(ctx, "USE "+quoteIdent(database)); err != nil {
		conn.Close()
		return nil, translateError(err)
	}
	return conn, nil
}

func execStatement(ctx context.Context, conn *sql.Conn, statement string) (driver.QueryResult, error) {
	outcome, err := conn.ExecContext(ctx, statement)
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}

	affected, err := outcome.RowsAffected()
	if err != nil {
		return driver.QueryResult{Rows: [][]*string{}}, nil
	}
	return driver.QueryResult{Rows: [][]*string{}, RowsAffected: affected}, nil
}

func queryStatement(
	ctx context.Context,
	conn *sql.Conn,
	statement string,
	maxRows int,
) (driver.QueryResult, error) {
	rows, err := conn.QueryContext(ctx, statement)
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}
	defer rows.Close()

	columns, err := resultColumns(rows)
	if err != nil {
		return driver.QueryResult{}, err
	}

	values, truncated, err := scanRows(rows, len(columns), maxRows)
	if err != nil {
		return driver.QueryResult{}, err
	}

	return driver.QueryResult{Columns: columns, Rows: values, Truncated: truncated}, nil
}

func resultColumns(rows *sql.Rows) ([]driver.Column, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, translateError(err)
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

// leadingKeyword returns the first word of a statement in lower case, looking past
// leading comments and the parentheses of a wrapped SELECT.
func leadingKeyword(statement string) string {
	rest := statement
	for {
		rest = strings.TrimLeft(rest, " \t\r\n(")

		switch {
		case strings.HasPrefix(rest, "--"), strings.HasPrefix(rest, "#"):
			end := strings.IndexAny(rest, "\r\n")
			if end < 0 {
				return ""
			}
			rest = rest[end+1:]
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return ""
			}
			rest = rest[end+4:]
		default:
			end := strings.IndexFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
			if end < 0 {
				end = len(rest)
			}
			return strings.ToLower(rest[:end])
		}
	}
}
