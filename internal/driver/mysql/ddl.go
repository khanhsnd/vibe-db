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

// CreateStatements asks MySQL for its own answer, because MySQL has one. `SHOW CREATE TABLE`
// reports the whole declaration — the storage engine, the character set, the auto_increment, every
// index and every constraint — and no read of information_schema puts all of that back together, so
// a dump is the server's own words rather than a description of them.
//
// It answers with one statement, indexes and constraints included, which is why nothing here
// spells either: they are already in it.
func (s *session) CreateStatements(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]string, error) {
	var name, declaration string

	query := "SHOW CREATE TABLE " + dialect.Table(ns, table)
	if err := s.db.QueryRowContext(ctx, query).Scan(&name, &declaration); err != nil {
		return nil, translateError(err)
	}
	return []string{qualified(ns, table, declaration)}, nil
}

// qualified names the database in the declaration the server answered with, which names the table
// alone — CREATE TABLE and then the bare name — where the rows written after it are qualified. Left
// as it came, a replayed dump would make the table in whatever database the connection was on and
// put the rows in the one they came from, which is the one arrangement a reader could not have meant.
//
// A prefix the server ever stopped spelling this way simply is not found, and the answer then
// travels unchanged: this can leave the declaration as it was but never make it something else.
func qualified(ns driver.Namespace, table, declaration string) string {
	return strings.Replace(declaration,
		"CREATE TABLE "+dialect.QuoteIdent(table),
		"CREATE TABLE "+dialect.Table(ns, table), 1)
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
