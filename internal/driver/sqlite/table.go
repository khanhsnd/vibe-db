package sqlite

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// TableStructure reads the three things the tree draws under a table. All three come from pragmas
// rather than from a catalogue table, which is what a file that keeps its schema as the text of the
// statements that made it has instead: SQLite parses its own declarations and answers about them.
func (s *session) TableStructure(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (driver.TableStructure, error) {
	columns, err := s.columns(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	indexes, err := s.indexes(ctx, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	keys, err := s.foreignKeys(ctx, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	return driver.TableStructure{Columns: columns, Indexes: indexes, ForeignKeys: keys}, nil
}

// columns reads the declared shape. A SQLite type is whatever the declaration said — the engine
// keeps the text and applies an affinity to it — so Type and FullType are the same string, there
// being no second, fuller spelling to report.
//
// `pk` is the column's position in the key rather than a flag, counting from one, which is how a
// key over two columns says which comes first.
func (s *session) columns(
	ctx context.Context,
	_ driver.Namespace,
	table string,
) ([]driver.Column, error) {
	const query = `
		SELECT name, type, "notnull", dflt_value, pk
		FROM pragma_table_info(?)
		ORDER BY cid`

	rows, err := s.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var name, declared string
		var notNull, key int
		var defaultValue sql.NullString

		if err := rows.Scan(&name, &declared, &notNull, &defaultValue, &key); err != nil {
			return nil, translateError(err)
		}

		columns = append(columns, driver.Column{
			Name:       name,
			Type:       declared,
			FullType:   declared,
			Nullable:   notNull == 0,
			PrimaryKey: key > 0,
			// A stored default is already the expression the declaration carried — `'x'` with its
			// quotes, `CURRENT_TIMESTAMP` without — so it travels verbatim the way PostgreSQL's does.
			Default: sqlkit.TextOrNil(defaultValue),
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

// indexes reads one row per index column, in index order, and folds them into one entry per index.
//
// The key is the part SQLite does not report here: an index of `origin` `pk` is the one behind a
// declared PRIMARY KEY, and an ordinary rowid table has *no* such index at all — the key is the
// rowid. So a table keyed the usual way reports no primary index, and the key is read off the
// columns instead, which is where `pragma_table_info` already put it.
func (s *session) indexes(ctx context.Context, table string) ([]driver.Index, error) {
	const query = `
		SELECT l.name, l."unique", l.origin = 'pk', i.name
		FROM pragma_index_list(?) l
		JOIN pragma_index_info(l.name) i
		ORDER BY l.origin = 'pk' DESC, l.seq, i.seqno`

	rows, err := s.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	indexes := []driver.Index{}
	for rows.Next() {
		var found driver.Index
		var column sql.NullString

		if err := rows.Scan(&found.Name, &found.Unique, &found.Primary, &column); err != nil {
			return nil, translateError(err)
		}
		// An index over an *expression* reports no column name. It is drawn as the blank it is
		// rather than dropped, so the index is still listed and still says how wide it is.
		indexes = sqlkit.AppendIndexColumn(indexes, found, column.String)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return indexes, nil
}

// foreignKeys reads the constraints as SQLite reports them: one row per column pair, grouped by an
// `id` rather than by a name, because a SQLite foreign key **has** no name of its own. The id is
// what a drop would need and there is no drop — see ddl.go — so it stands in as the name, which is
// what the reader is shown and what a comparison matches on.
//
// A key that names no column on the far side is one pointing at the other table's primary key, and
// SQLite reports the column as NULL there rather than resolving it.
func (s *session) foreignKeys(ctx context.Context, table string) ([]driver.ForeignKey, error) {
	const query = `
		SELECT 'fk_' || id, "table", "from", COALESCE("to", ''), on_update, on_delete
		FROM pragma_foreign_key_list(?)
		ORDER BY id, seq`

	rows, err := s.db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	keys := []driver.ForeignKey{}
	for rows.Next() {
		var found driver.ForeignKey
		var column, referenced string

		err := rows.Scan(
			&found.Name,
			&found.ReferencedTable,
			&column,
			&referenced,
			&found.OnUpdate,
			&found.OnDelete,
		)
		if err != nil {
			return nil, translateError(err)
		}
		keys = sqlkit.AppendKeyColumn(keys, found, column, referenced)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return keys, nil
}

// TableSchemas reads every table and column of the file in one query, for the editor's completion.
// Views are included, because completion cares about what can be selected from.
func (s *session) TableSchemas(
	ctx context.Context,
	_ driver.Namespace,
) ([]driver.TableSchema, error) {
	const query = `
		SELECT m.name, c.name
		FROM sqlite_master m
		JOIN pragma_table_info(m.name) c
		WHERE m.type IN ('table', 'view') AND m.name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY m.name, c.cid`

	rows, err := s.db.QueryContext(ctx, query)
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
