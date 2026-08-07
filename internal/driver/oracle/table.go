package oracle

import (
	"context"
	"database/sql"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// fullType composes a column's declared type out of the pieces `all_tab_columns` keeps it in:
// Oracle stores the name in one column and the length, the precision and the scale in three more,
// so `NUMBER` and `(10,2)` are four facts rather than one string.
//
// The character length is used rather than the byte length, because that is what the declaration
// said — a `VARCHAR2(50 CHAR)` is fifty characters and up to two hundred bytes.
const fullType = `
	data_type || CASE
	    WHEN data_type IN ('VARCHAR2', 'NVARCHAR2', 'CHAR', 'NCHAR', 'RAW')
	        THEN '(' || TO_CHAR(char_length) || ')'
	    WHEN data_type = 'NUMBER' AND data_precision IS NOT NULL
	        THEN '(' || TO_CHAR(data_precision) ||
	             CASE WHEN NVL(data_scale, 0) > 0 THEN ',' || TO_CHAR(data_scale) END || ')'
	    ELSE ''
	END`

// TableStructure reads the three things the tree draws under a table. The columns come from the
// same read a data page uses, so a field's type reads there exactly as it does above the grid.
func (s *session) TableStructure(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (driver.TableStructure, error) {
	columns, err := s.columns(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	indexes, err := s.indexes(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	keys, err := s.foreignKeys(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	return driver.TableStructure{Columns: columns, Indexes: indexes, ForeignKeys: keys}, nil
}

// columns reads a table's declared shape. Two things about it are Oracle's own.
//
// Nullability is not a flag but a *constraint*, so `nullable` here is `Y` or `N` and a column in
// the primary key reads as `N` because the key put it there. And the default is a `LONG`, which is
// a type nothing may be done to in a query — it cannot be trimmed, cast or coalesced — so it is
// read as it lies and whatever whitespace Oracle stored with it comes back with it.
func (s *session) columns(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Column, error) {
	query := `
		SELECT c.column_name,
		       c.data_type,` + fullType + `,
		       c.nullable,
		       CASE WHEN k.column_name IS NULL THEN 0 ELSE 1 END,
		       c.data_default
		FROM all_tab_columns c
		LEFT JOIN (
		    SELECT cc.owner, cc.table_name, cc.column_name
		    FROM all_constraints t
		    JOIN all_cons_columns cc
		      ON cc.owner = t.owner AND cc.constraint_name = t.constraint_name
		    WHERE t.constraint_type = 'P'
		) k ON k.owner = c.owner AND k.table_name = c.table_name
		   AND k.column_name = c.column_name
		WHERE c.owner = :1 AND c.table_name = :2
		ORDER BY c.column_id`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	columns := []driver.Column{}
	for rows.Next() {
		var name, dataType, declared, nullable string
		var primaryKey int
		var defaultValue sql.NullString

		err := rows.Scan(&name, &dataType, &declared, &nullable, &primaryKey, &defaultValue)
		if err != nil {
			return nil, translateError(err)
		}

		columns = append(columns, driver.Column{
			Name:       name,
			Type:       dataType,
			FullType:   declared,
			Nullable:   nullable == "Y",
			PrimaryKey: primaryKey == 1,
			Default:    trimmed(defaultValue),
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

// trimmed is a stored default with the whitespace Oracle keeps around it taken off here, since it
// could not be taken off in the query — see `columns`. A default that was nothing but whitespace is
// no default at all, which is what an empty one becomes.
func trimmed(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	value.String = strings.TrimSpace(value.String)
	if value.String == "" {
		return nil
	}
	return &value.String
}

// indexes reads one row per index column, in index order, and folds them into one entry per index.
//
// The primary key is the part Oracle reports sideways: an index does not say it is behind one, the
// *constraint* says which index it uses. So the constraint is joined in to mark it, which is what
// keeps the key's index at the top of the list the way it is on every other engine.
//
// The method is the index's own kind — `NORMAL`, `BITMAP`, `FUNCTION-BASED NORMAL` — which is what
// Oracle has instead of an access method a CREATE INDEX names.
func (s *session) indexes(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Index, error) {
	const query = `
		SELECT i.index_name,
		       CASE WHEN i.uniqueness = 'UNIQUE' THEN 1 ELSE 0 END,
		       CASE WHEN p.constraint_name IS NULL THEN 0 ELSE 1 END,
		       i.index_type,
		       c.column_name
		FROM all_indexes i
		JOIN all_ind_columns c
		  ON c.index_owner = i.owner AND c.index_name = i.index_name
		LEFT JOIN all_constraints p
		  ON p.owner = i.table_owner AND p.table_name = i.table_name
		 AND p.index_name = i.index_name AND p.constraint_type = 'P'
		WHERE i.table_owner = :1 AND i.table_name = :2
		ORDER BY CASE WHEN p.constraint_name IS NULL THEN 1 ELSE 0 END,
		         i.index_name, c.column_position`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	indexes := []driver.Index{}
	for rows.Next() {
		var found driver.Index
		var unique, primary int
		var column string

		if err := rows.Scan(&found.Name, &unique, &primary, &found.Method, &column); err != nil {
			return nil, translateError(err)
		}

		found.Unique, found.Primary = unique == 1, primary == 1
		indexes = sqlkit.AppendIndexColumn(indexes, found, column)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return indexes, nil
}

// foreignKeys joins each constraint to the columns it holds and to the columns of the constraint it
// points at, which is how Oracle names the far side: a foreign key names the *unique constraint* it
// references rather than the table and columns directly.
//
// There is no update rule to read. Oracle has no `ON UPDATE` on a referential constraint at all —
// see ddl.go — so what is reported is what exists, and the field stays empty rather than being
// filled with a default that would be written back into a statement Oracle refuses.
func (s *session) foreignKeys(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.ForeignKey, error) {
	const query = `
		SELECT c.constraint_name,
		       r.owner,
		       r.table_name,
		       c.delete_rule,
		       held.column_name,
		       points.column_name
		FROM all_constraints c
		JOIN all_constraints r
		  ON r.owner = c.r_owner AND r.constraint_name = c.r_constraint_name
		JOIN all_cons_columns held
		  ON held.owner = c.owner AND held.constraint_name = c.constraint_name
		JOIN all_cons_columns points
		  ON points.owner = r.owner AND points.constraint_name = r.constraint_name
		 AND points.position = held.position
		WHERE c.owner = :1 AND c.table_name = :2 AND c.constraint_type = 'R'
		ORDER BY c.constraint_name, held.position`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
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
			&found.ReferencedSchema,
			&found.ReferencedTable,
			&found.OnDelete,
			&column,
			&referenced,
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

// TableSchemas reads every table and column of a schema in one query, for the editor's completion.
// Views are included, because completion cares about what can be selected from — and on Oracle
// `all_tab_columns` already describes both, the word "tab" there covering anything with columns.
func (s *session) TableSchemas(
	ctx context.Context,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	const query = `
		SELECT table_name, column_name
		FROM all_tab_columns
		WHERE owner = :1
		ORDER BY table_name, column_id`

	rows, err := s.db.QueryContext(ctx, query, ns.Database)
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
