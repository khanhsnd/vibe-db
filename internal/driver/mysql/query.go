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
	return queryStatement(ctx, conn, statement, query.MaxRows)
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
