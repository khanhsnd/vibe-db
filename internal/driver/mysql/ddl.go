package mysql

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// primaryKeyName is what MySQL calls the index behind a primary key; it has no other name.
const primaryKeyName = "PRIMARY"

// TableStructure reads the three things the tree draws under a table. The columns come from
// the same read a data page uses, so a field's type reads there exactly as it does above the
// grid.
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

// RenameTable qualifies the new name with the same database as the old one, because MySQL
// reads an unqualified target as a table of whatever database the connection is on — which is
// not necessarily the one being browsed.
func (s *session) RenameTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" RENAME TO " + dialect.Table(ns, name)
	return s.run(ctx, ns.Database, statement)
}

// AlterColumn restates the whole column, which is the only form MODIFY COLUMN takes: a fact
// left out of it is dropped rather than kept.
func (s *session) AlterColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) error {
	// The clause names the column being changed, not the name in the spec, so renaming is
	// RenameColumn's job alone and never a side effect of an edit.
	restated := spec
	restated.Name = column

	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" MODIFY COLUMN " + dialect.ColumnClause(restated)
	return s.run(ctx, ns.Database, statement)
}

// SetPrimaryKey needs no name for the key it replaces: MySQL calls every primary key PRIMARY and
// drops it by that. It does need to know whether there is one, since dropping none is an error.
func (s *session) SetPrimaryKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	columns []string,
) error {
	stored, err := s.columns(ctx, ns, table)
	if err != nil {
		return err
	}

	actions := []string{}
	if len(sqlkit.PrimaryKeyNames(stored)) > 0 {
		actions = append(actions, "DROP PRIMARY KEY")
	}
	if len(columns) > 0 {
		actions = append(actions,
			"ADD PRIMARY KEY ("+strings.Join(dialect.QuoteAll(columns), ", ")+")")
	}
	if len(actions) == 0 {
		return nil
	}

	statement := "ALTER TABLE " + dialect.Table(ns, table) + " " + strings.Join(actions, ", ")
	return s.run(ctx, ns.Database, statement)
}

func (s *session) DropIndex(ctx context.Context, ns driver.Namespace, table, index string) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" DROP INDEX " + dialect.QuoteIdent(index)
	return s.run(ctx, ns.Database, statement)
}

func (s *session) DropForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table, constraint string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" DROP FOREIGN KEY " + dialect.QuoteIdent(constraint)
	return s.run(ctx, ns.Database, statement)
}

// run is what sqlkit.DDL executes its statements through, and the one place a schema change
// meets this adapter's error map. The database is already in every statement's table name, so
// nothing has to be selected for it.
func (s *session) run(ctx context.Context, _, statement string) error {
	if _, err := s.db.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

// indexes reads one row per index column, in index order, and folds them into one entry per
// index. The primary key comes back as an index named PRIMARY, which is how MySQL stores it.
func (s *session) indexes(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Index, error) {
	const query = `
		SELECT index_name, non_unique, index_type, column_name
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		ORDER BY index_name = 'PRIMARY' DESC, index_name, seq_in_index`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	indexes := []driver.Index{}
	for rows.Next() {
		var found driver.Index
		var nonUnique int
		var column string

		if err := rows.Scan(&found.Name, &nonUnique, &found.Method, &column); err != nil {
			return nil, translateError(err)
		}

		found.Unique = nonUnique == 0
		found.Primary = found.Name == primaryKeyName
		indexes = sqlkit.AppendIndexColumn(indexes, found, column)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return indexes, nil
}

// foreignKeys joins the columns of a constraint to the rules it acts by, which
// information_schema keeps in two different tables.
func (s *session) foreignKeys(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.ForeignKey, error) {
	const query = `
		SELECT k.constraint_name,
		       k.referenced_table_schema,
		       k.referenced_table_name,
		       r.update_rule,
		       r.delete_rule,
		       k.column_name,
		       k.referenced_column_name
		FROM information_schema.key_column_usage k
		JOIN information_schema.referential_constraints r
		  ON r.constraint_schema = k.constraint_schema
		 AND r.constraint_name = k.constraint_name
		 AND r.table_name = k.table_name
		WHERE k.table_schema = ? AND k.table_name = ?
		  AND k.referenced_table_name IS NOT NULL
		ORDER BY k.constraint_name, k.ordinal_position`

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
