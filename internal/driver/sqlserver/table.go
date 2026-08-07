package sqlserver

import (
	"context"
	"database/sql"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

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

// columns reads a table's declared shape. The full type is composed here rather than read, because
// SQL Server keeps a type's name in one catalogue and its length, precision and scale in another —
// `nvarchar` and `(50)` are two facts, and `varchar(max)` is the length reported as `-1`.
//
// A default arrives as the constraint's own expression, parentheses included: SQL Server stores
// `('none')` for `DEFAULT 'none'`, and that is already the expression a statement takes back.
func (s *session) columns(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Column, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT c.name,
		       y.name,
		       y.name + CASE
		           WHEN y.name IN ('decimal', 'numeric')
		               THEN '(' + CAST(c.precision AS NVARCHAR(8)) + ',' +
		                    CAST(c.scale AS NVARCHAR(8)) + ')'
		           WHEN y.name IN ('nvarchar', 'nchar') AND c.max_length = -1 THEN '(max)'
		           WHEN y.name IN ('nvarchar', 'nchar')
		               THEN '(' + CAST(c.max_length / 2 AS NVARCHAR(8)) + ')'
		           WHEN y.name IN ('varchar', 'char', 'varbinary', 'binary') AND c.max_length = -1
		               THEN '(max)'
		           WHEN y.name IN ('varchar', 'char', 'varbinary', 'binary')
		               THEN '(' + CAST(c.max_length AS NVARCHAR(8)) + ')'
		           ELSE ''
		       END,
		       c.is_nullable,
		       CASE WHEN k.column_id IS NULL THEN 0 ELSE 1 END,
		       d.definition
		FROM ` + catalogue + `.sys.columns c
		JOIN ` + catalogue + `.sys.tables t ON t.object_id = c.object_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		JOIN ` + catalogue + `.sys.types y ON y.user_type_id = c.user_type_id
		LEFT JOIN ` + catalogue + `.sys.default_constraints d ON d.object_id = c.default_object_id
		LEFT JOIN (
		    SELECT ic.object_id, ic.column_id
		    FROM ` + catalogue + `.sys.index_columns ic
		    JOIN ` + catalogue + `.sys.indexes i
		      ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		    WHERE i.is_primary_key = 1
		) k ON k.object_id = c.object_id AND k.column_id = c.column_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY c.column_id`

	rows, err := s.db.QueryContext(ctx, query, ns.Schema, table)
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

// indexes reads one row per index column, in index order, and folds them into one entry per index.
//
// The method is what the index *is* rather than an access method the reader chose — `CLUSTERED` or
// `NONCLUSTERED`, which is SQL Server's own distinction and the one thing worth knowing about an
// index here. A heap has no index at all and reports the row it keeps for one, `index_id = 0`,
// which is left out because it is not an index.
func (s *session) indexes(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Index, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT i.name, i.is_unique, i.is_primary_key, i.type_desc, c.name
		FROM ` + catalogue + `.sys.indexes i
		JOIN ` + catalogue + `.sys.tables t ON t.object_id = i.object_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		JOIN ` + catalogue + `.sys.index_columns ic
		  ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		JOIN ` + catalogue + `.sys.columns c
		  ON c.object_id = ic.object_id AND c.column_id = ic.column_id
		WHERE s.name = @p1 AND t.name = @p2 AND i.index_id > 0
		ORDER BY i.is_primary_key DESC, i.name, ic.key_ordinal`

	rows, err := s.db.QueryContext(ctx, query, ns.Schema, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	indexes := []driver.Index{}
	for rows.Next() {
		var found driver.Index
		var column string

		err := rows.Scan(&found.Name, &found.Unique, &found.Primary, &found.Method, &column)
		if err != nil {
			return nil, translateError(err)
		}
		indexes = sqlkit.AppendIndexColumn(indexes, found, column)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return indexes, nil
}

// foreignKeys joins each constraint to the column pairs it holds. The referential actions are
// spelled out by the catalogue itself, so nothing here has to translate a code the way PostgreSQL's
// single characters have to be translated.
func (s *session) foreignKeys(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.ForeignKey, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT f.name,
		       rs.name,
		       rt.name,
		       REPLACE(f.update_referential_action_desc, '_', ' '),
		       REPLACE(f.delete_referential_action_desc, '_', ' '),
		       c.name,
		       rc.name
		FROM ` + catalogue + `.sys.foreign_keys f
		JOIN ` + catalogue + `.sys.tables t ON t.object_id = f.parent_object_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		JOIN ` + catalogue + `.sys.tables rt ON rt.object_id = f.referenced_object_id
		JOIN ` + catalogue + `.sys.schemas rs ON rs.schema_id = rt.schema_id
		JOIN ` + catalogue + `.sys.foreign_key_columns k
		  ON k.constraint_object_id = f.object_id
		JOIN ` + catalogue + `.sys.columns c
		  ON c.object_id = k.parent_object_id AND c.column_id = k.parent_column_id
		JOIN ` + catalogue + `.sys.columns rc
		  ON rc.object_id = k.referenced_object_id AND rc.column_id = k.referenced_column_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY f.name, k.constraint_column_id`

	rows, err := s.db.QueryContext(ctx, query, ns.Schema, table)
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
			&found.OnUpdate,
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
// Views are included, because completion cares about what can be selected from.
func (s *session) TableSchemas(
	ctx context.Context,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT o.name, c.name
		FROM ` + catalogue + `.sys.objects o
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = o.schema_id
		JOIN ` + catalogue + `.sys.columns c ON c.object_id = o.object_id
		WHERE s.name = @p1 AND o.type IN ('U', 'V')
		ORDER BY o.name, c.column_id`

	rows, err := s.db.QueryContext(ctx, query, ns.Schema)
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
