package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

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

// RenameTable leaves the new name unqualified, because PostgreSQL reads a qualified one as an
// attempt to move the table to another schema and refuses it.
func (s *session) RenameTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" RENAME TO " + dialect.QuoteIdent(name)
	return s.run(ctx, ns.Database, statement)
}

// AlterColumn needs one action per fact changed: PostgreSQL has no single clause that
// restates a column the way MySQL's MODIFY COLUMN does. They travel in one ALTER TABLE so the
// column is never left half-changed.
func (s *session) AlterColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) error {
	quoted := dialect.QuoteIdent(column)
	actions := []string{
		"ALTER COLUMN " + quoted + " TYPE " + strings.TrimSpace(spec.Type),
	}

	if spec.Nullable && !spec.PrimaryKey {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP NOT NULL")
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" SET NOT NULL")
	}

	if spec.Default != nil && strings.TrimSpace(*spec.Default) != "" {
		actions = append(actions,
			"ALTER COLUMN "+quoted+" SET DEFAULT "+strings.TrimSpace(*spec.Default))
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP DEFAULT")
	}

	statement := "ALTER TABLE " + dialect.Table(ns, table) + " " + strings.Join(actions, ", ")
	return s.run(ctx, ns.Database, statement)
}

// SetPrimaryKey has to name the constraint it is replacing, which PostgreSQL keeps in
// pg_constraint rather than deriving from the table, so the old key is read before it is dropped.
func (s *session) SetPrimaryKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	columns []string,
) error {
	existing, err := s.primaryKeyName(ctx, ns, table)
	if err != nil {
		return err
	}

	actions := []string{}
	if existing != "" {
		actions = append(actions, "DROP CONSTRAINT "+dialect.QuoteIdent(existing))
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

func (s *session) primaryKeyName(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (string, error) {
	const query = `
		SELECT c.conname
		FROM pg_catalog.pg_constraint c
		JOIN pg_catalog.pg_class t ON t.oid = c.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND t.relname = $2 AND c.contype = 'p'`

	db, err := s.pool(ns.Database)
	if err != nil {
		return "", err
	}

	var name string
	err = db.QueryRowContext(ctx, query, ns.Schema, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", translateError(err)
	}
	return name, nil
}

// DropIndex names the index rather than the table it indexes: in PostgreSQL an index is an
// object of the schema in its own right.
func (s *session) DropIndex(ctx context.Context, ns driver.Namespace, _, index string) error {
	return s.run(ctx, ns.Database, "DROP INDEX "+dialect.Qualify(ns.Schema, index))
}

func (s *session) DropForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table, constraint string,
) error {
	statement := "ALTER TABLE " + dialect.Table(ns, table) +
		" DROP CONSTRAINT " + dialect.QuoteIdent(constraint)
	return s.run(ctx, ns.Database, statement)
}

// run is what sqlkit.DDL executes its statements through, and the one place a schema change
// meets this adapter's pool-per-database and its error map.
func (s *session) run(ctx context.Context, database, statement string) error {
	db, err := s.pool(database)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

// indexes reads one row per index column, in index order, and folds them into one entry per
// index. An index over an expression has no column of its own to report, so the expression
// stands in for the name.
func (s *session) indexes(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Index, error) {
	const query = `
		SELECT i.relname,
		       ix.indisunique,
		       ix.indisprimary,
		       am.amname,
		       COALESCE(a.attname, pg_catalog.pg_get_indexdef(ix.indexrelid, k.ord::int, true))
		FROM pg_catalog.pg_index ix
		JOIN pg_catalog.pg_class t ON t.oid = ix.indrelid
		JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_catalog.pg_am am ON am.oid = i.relam
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
		LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY ix.indisprimary DESC, i.relname, k.ord`

	db, err := s.pool(ns.Database)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, ns.Schema, table)
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

// foreignKeys unnests the two column lists of a constraint side by side, so each row carries
// a holding column and the column it points at.
func (s *session) foreignKeys(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.ForeignKey, error) {
	const query = `
		SELECT c.conname,
		       fn.nspname,
		       f.relname,
		       c.confupdtype,
		       c.confdeltype,
		       la.attname,
		       fa.attname
		FROM pg_catalog.pg_constraint c
		JOIN pg_catalog.pg_class t ON t.oid = c.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_catalog.pg_class f ON f.oid = c.confrelid
		JOIN pg_catalog.pg_namespace fn ON fn.oid = f.relnamespace
		JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS k(held, points, ord) ON true
		JOIN pg_catalog.pg_attribute la ON la.attrelid = c.conrelid AND la.attnum = k.held
		JOIN pg_catalog.pg_attribute fa ON fa.attrelid = c.confrelid AND fa.attnum = k.points
		WHERE n.nspname = $1 AND t.relname = $2 AND c.contype = 'f'
		ORDER BY c.conname, k.ord`

	db, err := s.pool(ns.Database)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, ns.Schema, table)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	keys := []driver.ForeignKey{}
	for rows.Next() {
		var found driver.ForeignKey
		var onUpdate, onDelete string
		var column, referenced string

		err := rows.Scan(
			&found.Name,
			&found.ReferencedSchema,
			&found.ReferencedTable,
			&onUpdate,
			&onDelete,
			&column,
			&referenced,
		)
		if err != nil {
			return nil, translateError(err)
		}

		found.OnUpdate = referentialAction(onUpdate)
		found.OnDelete = referentialAction(onDelete)
		keys = sqlkit.AppendKeyColumn(keys, found, column, referenced)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return keys, nil
}

// referentialAction spells out the single character pg_constraint stores.
func referentialAction(code string) string {
	switch code {
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
	}
}
