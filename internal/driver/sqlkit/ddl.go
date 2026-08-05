package sqlkit

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// DDL is the half of a schema change that does not vary by engine — the statements MySQL and
// PostgreSQL spell the same way. An adapter embeds it and hands it one way to run a statement;
// what is left to write per engine is the catalogue read and the four statements that really
// do differ: renaming a table, altering a column, and dropping an index or a constraint.
type DDL struct {
	Dialect Dialect
	// Run executes one statement against the named database, which is what an engine with a
	// pool per database needs and an engine with one pool ignores. It is also where the
	// adapter's own error translation happens, so nothing here has to know about it.
	Run func(ctx context.Context, database, statement string) error
}

func (x DDL) CreateTable(ctx context.Context, spec driver.TableSpec) error {
	return x.Run(ctx, spec.Database, x.Dialect.CreateTable(spec))
}

// RunAll executes statements in order, each in the database it names — which is what creating
// a database amounts to once its extensions and its comment are counted, since those can only
// be run from inside the database that has just been made.
func (x DDL) RunAll(ctx context.Context, statements []driver.DatabaseStatement) error {
	for _, statement := range statements {
		if err := x.Run(ctx, statement.In, statement.SQL); err != nil {
			return err
		}
	}
	return nil
}

func (x DDL) DropDatabase(ctx context.Context, name string) error {
	return x.Run(ctx, "", "DROP DATABASE "+x.Dialect.QuoteIdent(name))
}

// DuplicateTable declares the copy and then fills it, which is two statements because neither
// engine has one that does both: a table declared LIKE another takes its shape and none of its
// rows. Structure only stops after the first, so the two the menu offers are one method.
func (x DDL) DuplicateTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
	withRows bool,
) error {
	if err := x.Run(ctx, ns.Database, x.Dialect.CopyTable(ns, table, name)); err != nil {
		return err
	}
	if !withRows {
		return nil
	}
	return x.Run(ctx, ns.Database, x.Dialect.CopyRows(ns, table, name))
}

func (x DDL) DropTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.Run(ctx, ns.Database, "DROP TABLE "+x.Dialect.Table(ns, table))
}

func (x DDL) TruncateTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.Run(ctx, ns.Database, "TRUNCATE TABLE "+x.Dialect.Table(ns, table))
}

func (x DDL) EmptyTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.Run(ctx, ns.Database, "DELETE FROM "+x.Dialect.Table(ns, table))
}

func (x DDL) AddColumn(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ColumnSpec,
) error {
	statement := x.alter(ns, table) + " ADD COLUMN " + x.Dialect.ColumnClause(spec)
	return x.Run(ctx, ns.Database, statement)
}

func (x DDL) RenameColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column, name string,
) error {
	statement := x.alter(ns, table) +
		" RENAME COLUMN " + x.Dialect.QuoteIdent(column) + " TO " + x.Dialect.QuoteIdent(name)
	return x.Run(ctx, ns.Database, statement)
}

func (x DDL) DropColumn(ctx context.Context, ns driver.Namespace, table, column string) error {
	statement := x.alter(ns, table) + " DROP COLUMN " + x.Dialect.QuoteIdent(column)
	return x.Run(ctx, ns.Database, statement)
}

// CreateForeignKey is spelled the same by both engines once the quoting is the dialect's, and
// the referential actions reach the statement verbatim the way a column's type does.
func (x DDL) CreateForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ForeignKeySpec,
) error {
	statement := x.alter(ns, table) +
		" ADD CONSTRAINT " + x.Dialect.QuoteIdent(spec.Name) +
		" FOREIGN KEY (" + strings.Join(x.Dialect.QuoteAll(spec.Columns), ", ") + ")" +
		" REFERENCES " + x.Dialect.Referenced(ns, spec) +
		" (" + strings.Join(x.Dialect.QuoteAll(spec.ReferencedColumns), ", ") + ")"

	if action := strings.TrimSpace(spec.OnUpdate); action != "" {
		statement += " ON UPDATE " + action
	}
	if action := strings.TrimSpace(spec.OnDelete); action != "" {
		statement += " ON DELETE " + action
	}

	return x.Run(ctx, ns.Database, statement)
}

// Referenced names the table a key points at, which may sit outside the namespace holding the
// key: whichever part of a namespace this engine qualifies by is the part the spec filled in.
func (d Dialect) Referenced(ns driver.Namespace, spec driver.ForeignKeySpec) string {
	target := ns
	if spec.ReferencedSchema != "" {
		if d.NamespaceIsSchema {
			target.Schema = spec.ReferencedSchema
		} else {
			target.Database = spec.ReferencedSchema
		}
	}
	return d.Table(target, spec.ReferencedTable)
}

// InsertStatements spells rows as the statements that would put them back. A NULL travels as
// the keyword rather than as an empty string, so what is written out means what was read; every
// other value is quoted as a literal, which both engines read back into a column of any type.
func (x DDL) InsertStatements(
	ns driver.Namespace,
	table string,
	columns []string,
	rows [][]*string,
) []string {
	target := x.Dialect.Table(ns, table)
	names := strings.Join(x.Dialect.QuoteAll(columns), ", ")

	statements := make([]string, 0, len(rows))
	for _, row := range rows {
		values := make([]string, len(row))
		for at, value := range row {
			if value == nil {
				values[at] = "NULL"
				continue
			}
			values[at] = x.Dialect.QuoteString(*value)
		}

		statements = append(statements,
			"INSERT INTO "+target+" ("+names+") VALUES ("+strings.Join(values, ", ")+")")
	}

	return statements
}

func (x DDL) DropView(ctx context.Context, ns driver.Namespace, view string) error {
	return x.Run(ctx, ns.Database, "DROP VIEW "+x.Dialect.Table(ns, view))
}

// DropRoutine names the arguments only where the engine needs them to tell two routines of one
// name apart, which is PostgreSQL and not MySQL — and both spell the drop the same way once
// that difference travels in the routine itself.
func (x DDL) DropRoutine(ctx context.Context, ns driver.Namespace, routine driver.Routine) error {
	kind := "FUNCTION"
	if routine.Procedure {
		kind = "PROCEDURE"
	}

	statement := "DROP " + kind + " " + x.Dialect.Table(ns, routine.Name)
	if routine.Arguments != "" {
		statement += "(" + routine.Arguments + ")"
	}
	return x.Run(ctx, ns.Database, statement)
}

func (x DDL) alter(ns driver.Namespace, table string) string {
	return "ALTER TABLE " + x.Dialect.Table(ns, table)
}

// AppendIndexColumn folds the ordered rows of an index read into one entry per index, the way
// AppendColumn folds a schema read: only the last entry can still be growing.
func AppendIndexColumn(indexes []driver.Index, found driver.Index, column string) []driver.Index {
	last := len(indexes) - 1
	if last >= 0 && indexes[last].Name == found.Name {
		indexes[last].Columns = append(indexes[last].Columns, column)
		return indexes
	}
	found.Columns = []string{column}
	return append(indexes, found)
}

// AppendKeyColumn does the same for a foreign key, whose rows carry both a column it holds
// and the column that column points at.
func AppendKeyColumn(
	keys []driver.ForeignKey,
	found driver.ForeignKey,
	column, referenced string,
) []driver.ForeignKey {
	last := len(keys) - 1
	if last >= 0 && keys[last].Name == found.Name {
		keys[last].Columns = append(keys[last].Columns, column)
		keys[last].ReferencedColumns = append(keys[last].ReferencedColumns, referenced)
		return keys
	}
	found.Columns = []string{column}
	found.ReferencedColumns = []string{referenced}
	return append(keys, found)
}

// Table names a table the way the engine addresses it: inside its database on MySQL, inside
// its schema on PostgreSQL.
func (d Dialect) Table(ns driver.Namespace, table string) string {
	if d.NamespaceIsSchema {
		return d.Qualify(ns.Schema, table)
	}
	return d.Qualify(ns.Database, table)
}

// CreateTable renders the columns in the order they were given and collects the primary key
// into one clause, so a key spanning two columns is one constraint rather than two.
func (d Dialect) CreateTable(spec driver.TableSpec) string {
	clauses := make([]string, 0, len(spec.Columns)+1)
	keys := []string{}

	for _, column := range spec.Columns {
		clauses = append(clauses, d.ColumnClause(column))
		if column.PrimaryKey {
			keys = append(keys, column.Name)
		}
	}
	if len(keys) > 0 {
		clauses = append(clauses, "PRIMARY KEY ("+strings.Join(d.QuoteAll(keys), ", ")+")")
	}

	return "CREATE TABLE " + d.Table(spec.Namespace, spec.Table) +
		" (\n  " + strings.Join(clauses, ",\n  ") + "\n)"
}

// CopyTable declares a table shaped like another one. MySQL's LIKE is a clause of its own and
// brings the defaults and the indexes with it; PostgreSQL's stands where a column would and
// brings the columns alone until INCLUDING ALL asks for the rest.
func (d Dialect) CopyTable(ns driver.Namespace, table, name string) string {
	source := d.Table(ns, table)
	statement := "CREATE TABLE " + d.Table(ns, name)

	if d.LikeInColumnList {
		return statement + " (LIKE " + source + " INCLUDING ALL)"
	}
	return statement + " LIKE " + source
}

// CopyRows is every row of a table on its way into a copy of it. PostgreSQL's copy took the
// original's identity column with it and then refuses a value for that column, so the write says
// out loud that it is overriding one — a table with no identity column takes the same clause
// without complaint, which is what keeps this from needing to read the table first. A *generated*
// column is the one thing neither engine will be written, and says so itself.
func (d Dialect) CopyRows(ns driver.Namespace, table, name string) string {
	statement := "INSERT INTO " + d.Table(ns, name)
	if d.OverridesIdentity {
		statement += " OVERRIDING SYSTEM VALUE"
	}
	return statement + " SELECT * FROM " + d.Table(ns, table)
}

// ColumnClause is a column as it reads inside CREATE TABLE or ADD COLUMN. A column that is
// part of the primary key is written NOT NULL whatever it claimed, since neither engine will
// key a nullable column. The type and the default reach the statement verbatim — DDL takes no
// arguments, so there is nothing to bind them to.
func (d Dialect) ColumnClause(spec driver.ColumnSpec) string {
	clause := d.QuoteIdent(spec.Name) + " " + strings.TrimSpace(spec.Type)

	if !spec.Nullable || spec.PrimaryKey {
		clause += " NOT NULL"
	}
	if spec.Default != nil && strings.TrimSpace(*spec.Default) != "" {
		clause += " DEFAULT " + strings.TrimSpace(*spec.Default)
	}
	return clause
}
