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
	return x.Run(ctx, ns.Database, x.Dialect.AddColumn(ns, table, spec))
}

// AlterStatements spells changes to a table as the statements that would make them, and runs none
// of them. Every one of them is spelled by the same Dialect method the run beside it uses, so what
// a migration shows the reader and what its Apply does cannot drift apart.
func (x DDL) AlterStatements(
	ns driver.Namespace,
	table string,
	changes []driver.TableChange,
) []string {
	statements := make([]string, 0, len(changes))

	for _, change := range changes {
		switch change.Kind {
		case driver.ChangeAddColumn:
			statements = append(statements, x.Dialect.AddColumn(ns, table, change.Column))
		case driver.ChangeAlterColumn:
			statements = append(statements,
				x.Dialect.AlterColumn(ns, table, change.Name, change.Column))
		case driver.ChangeDropColumn:
			statements = append(statements, x.Dialect.DropColumn(ns, table, change.Name))
		case driver.ChangeCreateIndex:
			statements = append(statements, x.Dialect.CreateIndex(ns, table, change.Index))
		case driver.ChangeDropIndex:
			statements = append(statements, x.Dialect.DropIndex(ns, table, change.Name))
		case driver.ChangeCreateForeignKey:
			statements = append(statements, x.Dialect.AddForeignKey(ns, table, change.Key))
		case driver.ChangeDropForeignKey:
			statements = append(statements, x.Dialect.DropForeignKey(ns, table, change.Name))
		}
	}

	return statements
}

// AddColumn is that statement as text rather than as a run, the arrangement AddForeignKey already
// has and for the same reason: a migration writes the clause the designer would have executed
// instead of a second spelling of it.
func (d Dialect) AddColumn(ns driver.Namespace, table string, spec driver.ColumnSpec) string {
	return "ALTER TABLE " + d.Table(ns, table) + " ADD COLUMN " + d.ColumnClause(spec)
}

func (d Dialect) DropColumn(ns driver.Namespace, table, column string) string {
	return "ALTER TABLE " + d.Table(ns, table) + " DROP COLUMN " + d.QuoteIdent(column)
}

// AlterColumn is the one statement of these that the two engines do not merely spell differently
// but *shape* differently. MySQL restates the column whole, so a fact left out of the clause is a
// fact dropped; PostgreSQL has no such clause and takes one action per fact, all in one ALTER TABLE
// so the column is never left half-changed.
//
// Either way the clause names the column **being changed** rather than the name in the spec, so
// renaming is RenameColumn's alone and never a side effect of an edit.
func (d Dialect) AlterColumn(
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) string {
	head := "ALTER TABLE " + d.Table(ns, table) + " "

	if d.RestatesColumn {
		restated := spec
		restated.Name = column
		return head + "MODIFY COLUMN " + d.ColumnClause(restated)
	}

	quoted := d.QuoteIdent(column)
	actions := []string{"ALTER COLUMN " + quoted + " TYPE " + strings.TrimSpace(spec.Type)}

	if spec.Nullable && !spec.PrimaryKey {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP NOT NULL")
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" SET NOT NULL")
	}

	if spec.Default != nil && strings.TrimSpace(*spec.Default) != "" {
		actions = append(actions, "ALTER COLUMN "+quoted+" SET DEFAULT "+strings.TrimSpace(*spec.Default))
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP DEFAULT")
	}

	return head + strings.Join(actions, ", ")
}

// CreateIndex differs between the engines only in where the access method goes: MySQL takes it
// between the name and the table, PostgreSQL after the table. An empty method takes the engine's
// own default, and reaches the statement verbatim the way a column's type does.
func (d Dialect) CreateIndex(ns driver.Namespace, table string, spec driver.IndexSpec) string {
	statement := "CREATE "
	if spec.Unique {
		statement += "UNIQUE "
	}
	statement += "INDEX " + d.QuoteIdent(spec.Name)

	method := strings.TrimSpace(spec.Method)
	using := ""
	if method != "" {
		using = " USING " + method
	}

	if d.MethodBeforeTable {
		statement += using + " ON " + d.Table(ns, table)
	} else {
		statement += " ON " + d.Table(ns, table) + using
	}

	return statement + " (" + strings.Join(d.QuoteAll(spec.Columns), ", ") + ")"
}

// DropIndex names the index alone where an index is an object of the schema in its own right, and
// goes through the table where it belongs to one.
func (d Dialect) DropIndex(ns driver.Namespace, table, index string) string {
	if d.IndexIsObject {
		return "DROP INDEX " + d.Qualify(ns.Schema, index)
	}
	return "ALTER TABLE " + d.Table(ns, table) + " DROP INDEX " + d.QuoteIdent(index)
}

func (d Dialect) DropForeignKey(ns driver.Namespace, table, constraint string) string {
	clause := " DROP FOREIGN KEY "
	if d.ForeignKeyIsConstraint {
		clause = " DROP CONSTRAINT "
	}
	return "ALTER TABLE " + d.Table(ns, table) + clause + d.QuoteIdent(constraint)
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
	return x.Run(ctx, ns.Database, x.Dialect.DropColumn(ns, table, column))
}

// AlterColumn, CreateIndex, DropIndex and DropForeignKey are here rather than in each adapter
// because the Dialect above now spells all four: what differs between the engines is a *flag* —
// whether a column is restated whole, where an index's method goes, whether an index is an object
// of the schema, and what the clause dropping a constraint is called — and a flag is the form every
// other difference between the two takes.
func (x DDL) AlterColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) error {
	return x.Run(ctx, ns.Database, x.Dialect.AlterColumn(ns, table, column, spec))
}

func (x DDL) CreateIndex(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.IndexSpec,
) error {
	return x.Run(ctx, ns.Database, x.Dialect.CreateIndex(ns, table, spec))
}

func (x DDL) DropIndex(ctx context.Context, ns driver.Namespace, table, index string) error {
	return x.Run(ctx, ns.Database, x.Dialect.DropIndex(ns, table, index))
}

func (x DDL) DropForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table, constraint string,
) error {
	return x.Run(ctx, ns.Database, x.Dialect.DropForeignKey(ns, table, constraint))
}

// CreateForeignKey is spelled the same by both engines once the quoting is the dialect's, and
// the referential actions reach the statement verbatim the way a column's type does.
func (x DDL) CreateForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ForeignKeySpec,
) error {
	return x.Run(ctx, ns.Database, x.Dialect.AddForeignKey(ns, table, spec))
}

// AddForeignKey is that statement as text rather than as a run, so a dump writes the same clause
// the designer would have executed instead of a second spelling of it.
func (d Dialect) AddForeignKey(
	ns driver.Namespace,
	table string,
	spec driver.ForeignKeySpec,
) string {
	statement := "ALTER TABLE " + d.Table(ns, table) +
		" ADD CONSTRAINT " + d.QuoteIdent(spec.Name) +
		" FOREIGN KEY (" + strings.Join(d.QuoteAll(spec.Columns), ", ") + ")" +
		" REFERENCES " + d.Referenced(ns, spec) +
		" (" + strings.Join(d.QuoteAll(spec.ReferencedColumns), ", ") + ")"

	if action := strings.TrimSpace(spec.OnUpdate); action != "" {
		statement += " ON UPDATE " + action
	}
	if action := strings.TrimSpace(spec.OnDelete); action != "" {
		statement += " ON DELETE " + action
	}

	return statement
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

// RowStatements spells rows as the statements that write them into a table.
//
// A NULL travels as the keyword rather than as an empty string, so what is written out means what
// was read; every other value is quoted as a literal, which both engines read back into a column of
// any type. Nothing is bound, for the reason no DDL here binds anything: these statements are also
// what a dump and a SQL export *are*, and a file of placeholders is a file nothing can run.
func (x DDL) RowStatements(
	ns driver.Namespace,
	table string,
	write driver.RowWrite,
) []string {
	target := x.Dialect.Table(ns, table)

	switch write.Mode {
	case driver.WriteDelete:
		return x.rowsRemoved(target, write)
	case driver.WriteUpdate:
		return x.rowsChanged(target, write)
	default:
		return x.rowsAdded(target, write)
	}
}

// rowsAdded is the three modes that are an INSERT: appending, merging on a key, and skipping what
// is already there. They differ only in the verb and in the clause after the values, which is why
// they are one method — and all three carry `Batch` rows to a statement, since an insert is the one
// write a server takes several rows of at once.
func (x DDL) rowsAdded(target string, write driver.RowWrite) []string {
	head := x.Dialect.InsertVerb(write.Mode) + " INTO " + target +
		" (" + strings.Join(x.Dialect.QuoteAll(write.Columns), ", ") + ") VALUES "
	tail := x.Dialect.MergeClause(write.Mode, write.Keys, write.Columns)

	batch := max(write.Batch, 1)
	statements := make([]string, 0, (len(write.Rows)+batch-1)/batch)

	for at := 0; at < len(write.Rows); at += batch {
		tuples := make([]string, 0, batch)
		for _, row := range write.Rows[at:min(at+batch, len(write.Rows))] {
			tuples = append(tuples, "("+strings.Join(x.literals(row), ", ")+")")
		}
		statements = append(statements, head+strings.Join(tuples, ", ")+tail)
	}
	return statements
}

// rowsChanged is one UPDATE per row. It cannot be batched the way an insert can — every row sets
// different values on a different key — and a key column is left out of the SET, since changing the
// thing a row is found by is a rename rather than an update.
func (x DDL) rowsChanged(target string, write driver.RowWrite) []string {
	keys := set(write.Keys)

	statements := make([]string, 0, len(write.Rows))
	for _, row := range write.Rows {
		assignments := []string{}
		for at, name := range write.Columns {
			if keys[name] || at >= len(row) {
				continue
			}
			assignments = append(assignments, x.Dialect.QuoteIdent(name)+" = "+x.literal(row[at]))
		}
		if len(assignments) == 0 {
			continue
		}
		statements = append(statements, "UPDATE "+target+" SET "+strings.Join(assignments, ", ")+
			" WHERE "+x.match(write, row))
	}
	return statements
}

func (x DDL) rowsRemoved(target string, write driver.RowWrite) []string {
	statements := make([]string, 0, len(write.Rows))
	for _, row := range write.Rows {
		statements = append(statements, "DELETE FROM "+target+" WHERE "+x.match(write, row))
	}
	return statements
}

// match is the WHERE that finds the one stored row this row stands for. A key holding NULL is
// compared with IS NULL, because `= NULL` is true of nothing at all and would silently match no row
// — which on a delete reads as "it worked" and on an update as "nothing needed changing".
func (x DDL) match(write driver.RowWrite, row []*string) string {
	at := map[string]int{}
	for index, name := range write.Columns {
		at[name] = index
	}

	conditions := make([]string, 0, len(write.Keys))
	for _, key := range write.Keys {
		index, known := at[key]
		if !known || index >= len(row) {
			continue
		}
		if row[index] == nil {
			conditions = append(conditions, x.Dialect.QuoteIdent(key)+" IS NULL")
			continue
		}
		conditions = append(conditions,
			x.Dialect.QuoteIdent(key)+" = "+x.Dialect.QuoteString(*row[index]))
	}

	// A row with nothing to find it by would otherwise become a statement that matched the whole
	// table, so it matches nothing instead: the run reports it, and no other row is touched.
	if len(conditions) == 0 {
		return "1 = 0"
	}
	return strings.Join(conditions, " AND ")
}

func (x DDL) literals(row []*string) []string {
	values := make([]string, len(row))
	for at, value := range row {
		values[at] = x.literal(value)
	}
	return values
}

func (x DDL) literal(value *string) string {
	if value == nil {
		return "NULL"
	}
	return x.Dialect.QuoteString(*value)
}

func set(names []string) map[string]bool {
	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[name] = true
	}
	return held
}

// TableSpecOf reads a table's stored shape back as the declaration that would make it, which is what
// a dump of an engine that will not answer for its own DDL has to be built from — see PostgreSQL's
// CreateStatements. FullType is the type that goes in: it is what the server will parse and what the
// reader is shown above the grid, `varchar(255)` rather than `varchar`. A stored default is already
// an expression, so it travels verbatim exactly as a designer's does.
func TableSpecOf(
	ns driver.Namespace,
	table string,
	columns []driver.Column,
) driver.TableSpec {
	specs := make([]driver.ColumnSpec, 0, len(columns))
	for _, column := range columns {
		kind := column.FullType
		if kind == "" {
			kind = column.Type
		}
		specs = append(specs, driver.ColumnSpec{
			Name:       column.Name,
			Type:       kind,
			Nullable:   column.Nullable,
			Default:    column.Default,
			PrimaryKey: column.PrimaryKey,
		})
	}
	return driver.TableSpec{Namespace: ns, Table: table, Columns: specs}
}

// IndexSpecOf is an index the catalogue reported as the declaration that would build it again.
func IndexSpecOf(index driver.Index) driver.IndexSpec {
	return driver.IndexSpec{
		Name:    index.Name,
		Columns: index.Columns,
		Unique:  index.Unique,
		Method:  index.Method,
	}
}

// ForeignKeySpecOf is a constraint the catalogue reported as the declaration that would add it again.
func ForeignKeySpecOf(key driver.ForeignKey) driver.ForeignKeySpec {
	return driver.ForeignKeySpec{
		Name:              key.Name,
		Columns:           key.Columns,
		ReferencedSchema:  key.ReferencedSchema,
		ReferencedTable:   key.ReferencedTable,
		ReferencedColumns: key.ReferencedColumns,
		OnUpdate:          key.OnUpdate,
		OnDelete:          key.OnDelete,
	}
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

// InsertVerb is how this engine opens an insert that is not to fail on a row already there. MySQL
// has a verb for it; PostgreSQL says the same thing in a clause at the other end of the statement,
// so its verb never changes — see MergeClause.
func (d Dialect) InsertVerb(mode driver.RowWriteMode) string {
	if mode == driver.WriteSkip && !d.MergesOnConflict {
		return "INSERT IGNORE"
	}
	return "INSERT"
}

// MergeClause is what follows the values to say what a row already in the table should do, and is
// empty for a plain append — which is what leaves the collision to be reported as the failure it is.
//
// The two engines disagree about *what is already there*. PostgreSQL has to be told which columns
// to watch, and they must carry a unique constraint or it refuses the statement; MySQL is told
// nothing and applies whichever unique key the row broke. So the key list reaches one and not the
// other, and a reader who names the wrong columns is answered by the server rather than here.
func (d Dialect) MergeClause(mode driver.RowWriteMode, keys, columns []string) string {
	if mode != driver.WriteUpsert && mode != driver.WriteSkip {
		return ""
	}

	if d.MergesOnConflict {
		target := ""
		if len(keys) > 0 {
			target = " (" + strings.Join(d.QuoteAll(keys), ", ") + ")"
		}
		if mode == driver.WriteSkip {
			return " ON CONFLICT" + target + " DO NOTHING"
		}
		return " ON CONFLICT" + target + " DO UPDATE SET " +
			strings.Join(d.assignments(keys, columns, "EXCLUDED."), ", ")
	}

	if mode == driver.WriteSkip {
		// Already said in the verb. See InsertVerb.
		return ""
	}
	return " ON DUPLICATE KEY UPDATE " + strings.Join(d.assignments(keys, columns, ""), ", ")
}

// assignments is the SET of a merge: every column that is not part of the key, taking the value the
// row *would* have been inserted with. A key column is left out because it is what the collision was
// found by, and setting it to itself is at best noise and on PostgreSQL is refused outright.
//
// `from` is how each engine names that would-be value: PostgreSQL has a pseudo-table, and MySQL has
// a function that reads the same way in every version from 5.7 onwards.
func (d Dialect) assignments(keys, columns []string, from string) []string {
	key := set(keys)

	out := make([]string, 0, len(columns))
	for _, name := range columns {
		if key[name] {
			continue
		}
		quoted := d.QuoteIdent(name)
		if from == "" {
			out = append(out, quoted+" = VALUES("+quoted+")")
			continue
		}
		out = append(out, quoted+" = "+from+quoted)
	}

	// Every column being part of the key leaves nothing to change, so the merge becomes the skip it
	// already is — a row that is entirely its own key is the row that is already there.
	if len(out) == 0 && len(columns) > 0 {
		first := d.QuoteIdent(columns[0])
		if from == "" {
			return []string{first + " = VALUES(" + first + ")"}
		}
		return []string{first + " = " + from + first}
	}
	return out
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
