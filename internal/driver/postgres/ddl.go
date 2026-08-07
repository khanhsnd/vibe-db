package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
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

// CreateStatements has to *describe* the table, because PostgreSQL cannot be asked: there is no
// SHOW CREATE TABLE, and `pg_dump` is another program. So the shape the tree already reads is
// spelled back out through the same dialect every other statement here goes through — the columns
// and the key as one CREATE TABLE, then an index each, then a constraint each.
//
// The index behind the primary key is left out: it arrives with the table's own PRIMARY KEY clause,
// and declaring it a second time is a statement the server refuses. What a description cannot carry
// is an index over an *expression*, which reports the expression where a column name would be — the
// honest cost of an engine that will not answer for itself.
func (s *session) CreateStatements(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]string, error) {
	structure, err := s.TableStructure(ctx, ns, table)
	if err != nil {
		return nil, err
	}

	spec := sqlkit.TableSpecOf(ns, table, declared(structure.Columns))
	statements := []string{dialect.CreateTable(spec)}
	for _, index := range structure.Indexes {
		if index.Primary {
			continue
		}
		statements = append(statements, dialect.CreateIndex(ns, table, sqlkit.IndexSpecOf(index)))
	}
	for _, key := range structure.ForeignKeys {
		statements = append(statements,
			dialect.AddForeignKey(ns, table, sqlkit.ForeignKeySpecOf(key)))
	}

	return statements, nil
}

// Source reads a definition the way PostgreSQL keeps one, which is two different ways.
//
// A **routine** is answered for whole: `pg_get_functiondef` writes the entire `CREATE OR REPLACE
// FUNCTION`, its arguments, its language, its volatility and its body inside dollar quotes, and
// that is the server's own words in the sense CreateStatements means. It is found by its identity
// arguments as well as its name, because on this engine two functions may share the name — the
// same reason a drop carries them.
//
// A **view** is not: `pg_get_viewdef` answers with the SELECT and nothing else, there being no
// catalogue function that writes the CREATE. So the CREATE is put in front of it here, which is
// the driver's to spell like every other statement — and it is spelled `OR REPLACE`, since that is
// what makes a save of an unaltered view a statement that changes nothing rather than a drop.
func (s *session) Source(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	if ref.Kind == driver.SourceView {
		return s.viewSource(ctx, ns, ref)
	}
	return s.routineSource(ctx, ns, ref)
}

func (s *session) viewSource(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	const query = `
		SELECT pg_catalog.pg_get_viewdef(c.oid, true)
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('v', 'm')`

	body, err := s.sourceText(ctx, ns.Database, query, ns.Schema, ref.Name)
	if err != nil {
		return driver.Source{}, err
	}

	definition := "CREATE OR REPLACE VIEW " + dialect.Table(ns, ref.Name) + " AS\n" +
		strings.TrimSpace(body)
	return driver.Source{SourceRef: ref, Definition: definition}, nil
}

func (s *session) routineSource(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	const query = `
		SELECT pg_catalog.pg_get_functiondef(p.oid)
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND p.proname = $2
		  AND pg_catalog.pg_get_function_identity_arguments(p.oid) = $3
		  AND p.prokind IN ('f', 'p')`

	definition, err := s.sourceText(ctx, ns.Database, query, ns.Schema, ref.Name, ref.Arguments)
	if err != nil {
		return driver.Source{}, err
	}
	return driver.Source{SourceRef: ref, Definition: strings.TrimSpace(definition)}, nil
}

// sourceText runs a read that answers with one definition, and reports an object that is not there
// as one rather than as an empty definition — which is what a save would then wipe it out with.
func (s *session) sourceText(
	ctx context.Context,
	database, query string,
	args ...any,
) (string, error) {
	db, err := s.pool(database)
	if err != nil {
		return "", err
	}

	var text string
	err = db.QueryRowContext(ctx, query, args...).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrUnknownDefinition
	}
	if err != nil {
		return "", translateError(err)
	}
	return text, nil
}

// sequenceDefault is how a column that reads from a sequence spells its default.
const sequenceDefault = "nextval("

// serialTypes is what such a column is declared as instead of its own type and that default.
var serialTypes = map[string]string{
	"smallint": "smallserial",
	"integer":  "serial",
	"bigint":   "bigserial",
}

// declared is the columns as a dump has to *declare* them, which is not in every case how the
// catalogue reports them.
//
// A `serial` is not a type PostgreSQL keeps. It is an `integer`, a sequence, and a default reading
// the next value from it — three objects — so a read reports the first and the third and says nothing
// about the second. Written back out as read, the column would carry `DEFAULT nextval('t_id_seq')`
// naming a sequence the dump never created, and the whole statement would fail. Declaring the
// shorthand instead makes the sequence, which is the whole of what the column needs.
func declared(columns []driver.Column) []driver.Column {
	out := make([]driver.Column, 0, len(columns))
	for _, column := range columns {
		shorthand, ok := serialTypes[column.FullType]
		if ok && column.Default != nil && strings.HasPrefix(*column.Default, sequenceDefault) {
			column.FullType = shorthand
			column.Default = nil
		}
		out = append(out, column)
	}
	return out
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
