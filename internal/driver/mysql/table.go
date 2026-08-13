package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// columns reads a table's declared shape, which is what every read and every write of a row
// begins with — see `sqlkit.Data`, which is where those statements are spelled.
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
		SELECT column_name, data_type, column_type, is_nullable, column_key, column_default,
		       extra, column_comment
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
		var name, dataType, fullType, nullable, key, extra, comment string
		var defaultValue sql.NullString

		err := rows.Scan(
			&name, &dataType, &fullType, &nullable, &key, &defaultValue, &extra, &comment,
		)
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
			Comment:    comment,
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
