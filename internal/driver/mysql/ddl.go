package mysql

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
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

	pieces, err := s.partitions(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	return driver.TableStructure{
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: keys,
		Partitions:  pieces,
	}, nil
}

// partitions reads the pieces a table's rows are stored in. A table that is not partitioned still
// has a row here — one whose `partition_name` is NULL, which is how information_schema says "the
// whole table" — so that row is left out and such a table answers with no partitions at all.
//
// `partition_description` is what the piece holds without the words in front of it: `2027` for a
// range and `1, 3` for a list. The clause is put back together here, because what a reader is shown
// and what an ALTER takes are the same text — see `driver.Partition.Bounds`.
func (s *session) partitions(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Partition, error) {
	const query = `
		SELECT partition_name,
		       partition_method,
		       partition_expression,
		       partition_description,
		       table_rows
		FROM information_schema.partitions
		WHERE table_schema = ? AND table_name = ? AND partition_name IS NOT NULL
		ORDER BY partition_ordinal_position`

	rows, err := s.db.QueryContext(ctx, query, ns.Database, table)
	if err != nil {
		return nil, translateError(err)
	}

	found, err := sqlkit.ScanPartitions(rows)
	if err != nil {
		return nil, translateError(err)
	}

	for at, piece := range found {
		found[at].Bounds = partitionBounds(piece.Method, piece.Bounds)
	}
	return found, nil
}

// partitionBounds is the `VALUES` clause a piece was declared with, put back together from the
// description information_schema keeps. `MAXVALUE` is a keyword rather than a value and so is
// written bare; a hash or a key partition has no bounds at all, its piece being decided by
// arithmetic on the expression rather than by anything written down.
func partitionBounds(method, description string) string {
	switch {
	case description == "":
		return ""
	case strings.HasPrefix(method, "RANGE"):
		if strings.EqualFold(description, "MAXVALUE") {
			return "VALUES LESS THAN MAXVALUE"
		}
		return "VALUES LESS THAN (" + description + ")"
	case strings.HasPrefix(method, "LIST"):
		return "VALUES IN (" + description + ")"
	default:
		return ""
	}
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
	return []string{qualified(ns, "CREATE TABLE", table, declaration)}, nil
}

// Source asks MySQL for its own words, exactly as CreateStatements does and for the same reason:
// `SHOW CREATE VIEW`, `SHOW CREATE PROCEDURE` / `FUNCTION` and `SHOW CREATE TRIGGER` report the
// whole declaration — the algorithm, the definer, the SQL security, the parameter list and the body
// — and no read of information_schema puts all of that back together.
//
// **The answer's columns differ by statement and by version**, so the row is scanned into as many
// strings as it turned out to have and the declaration is taken by the name of its column — see
// `declaration`, which is where the one statement that does not call it `Create …` is named.
func (s *session) Source(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	// A kind this engine has not got is refused here rather than sent, because `SHOW CREATE` would
	// otherwise be spelled with words no MySQL server knows and the reader would be shown a syntax
	// error instead of an answer.
	if ref.Kind == driver.SourceMaterializedView || ref.Kind == driver.SourceSequence {
		return driver.Source{}, domain.Unsupported(string(ref.Kind))
	}

	query := "SHOW CREATE " + ref.Kind.Keyword() + " " + dialect.Table(ns, ref.Name)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return driver.Source{}, translateError(err)
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return driver.Source{}, translateError(err)
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return driver.Source{}, translateError(err)
		}
		return driver.Source{}, domain.ErrUnknownDefinition
	}

	// A definition the account may not read comes back NULL rather than missing, which is why
	// every column is scanned as a pointer: the answer is then "there is one and you cannot see
	// it" rather than a crash on the way past it.
	cells := make([]*string, len(names))
	into := make([]any, len(names))
	for at := range cells {
		into[at] = &cells[at]
	}
	if err := rows.Scan(into...); err != nil {
		return driver.Source{}, translateError(err)
	}

	for at, column := range names {
		if !declaration(column) || cells[at] == nil {
			continue
		}
		return driver.Source{
			SourceRef:  ref,
			Definition: qualified(ns, ref.Kind.Keyword(), ref.Name, *cells[at]),
		}, nil
	}
	return driver.Source{}, domain.Unsupported("reading this definition: the server did not answer with one")
}

// declaration says which column of a SHOW CREATE answer holds the statement. Every one of them
// calls it `Create <something>` except the trigger's, which calls it what it is — MySQL keeps a
// trigger as the text that was submitted rather than as a declaration it rebuilds, so the rest of
// that text is the reader's own spelling and the table inside it is usually bare. What makes such a
// definition land where it was read from is that a save is run *inside* its database — see
// `sqlkit.DDL.Pin`.
func declaration(column string) bool {
	return strings.HasPrefix(column, "Create ") || column == "SQL Original Statement"
}

// qualified names the database in a declaration the server answered with, which names the object
// alone — `CREATE TABLE` and then the bare name — where everything written after it is qualified.
// Left as it came, a replayed dump would make the table in whatever database the connection was on
// and put the rows in the one they came from, which is the one arrangement a reader could not have
// meant; and a routine put back would land wherever the pool happened to be pointed rather than
// where it was read from.
//
// A prefix the server ever stopped spelling this way simply is not found, and the answer then
// travels unchanged: this can leave the declaration as it was but never make it something else. A
// view already arrives qualified, so its own keyword finds nothing and nothing is done.
func qualified(ns driver.Namespace, keyword, name, declaration string) string {
	return strings.Replace(declaration,
		keyword+" "+dialect.QuoteIdent(name),
		keyword+" "+dialect.Table(ns, name), 1)
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
