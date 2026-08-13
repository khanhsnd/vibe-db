package sqlkit

import (
	"context"
	"database/sql"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
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
	// Pin holds one pooled connection put *inside* a namespace — MySQL's and SQL Server's `USE`,
	// PostgreSQL's `search_path`, Oracle's `CURRENT_SCHEMA`. It is the connection a **definition**
	// is run on, and nothing else here needs one.
	//
	// Every other statement this app writes addresses its object by a qualified name; a definition
	// cannot, because its body is the reader's own text and an unqualified table inside a view, a
	// routine or a trigger resolves against whichever namespace the connection is in. Two engines
	// go further and refuse the statement outright rather than putting it in the wrong place: MySQL
	// will not have a trigger and its table in different schemas, and SQL Server's `CREATE TRIGGER`
	// has to be the first statement of a batch in the database it belongs to.
	//
	// An adapter that leaves it nil has one namespace to be in — SQLite — and runs its definitions
	// through Run like everything else.
	//
	// It hands back the way to give the connection back as well as the connection, because inside a
	// manual transaction there is nothing to give back: it is `Txn.Pinned` in all four adapters that
	// set it, so a definition saved while one is open joins that transaction like every other
	// statement.
	Pin func(ctx context.Context, ns driver.Namespace) (Queryer, func(), error)
	// Translate is the adapter's error map. `Run` applies its own; what goes through `Pin` or the
	// open transaction has to be given one too, since both are executed here rather than there.
	Translate func(error) error
	// Tx is the session's manual transaction, and nil is a session that cannot hold one. While one
	// is open every statement below runs *inside* it rather than through Run — see exec, and see
	// Txn for why a transaction is one connection and therefore the whole session's.
	Tx *Txn
}

// exec runs one statement wherever this session's statements currently go: inside the open
// transaction, or through the adapter's own pool. It is the one seam, so no method below has to
// know a transaction exists.
func (x DDL) exec(ctx context.Context, database, statement string) error {
	open := x.Tx.Queryer()
	if open == nil {
		return x.Run(ctx, database, statement)
	}
	if _, err := open.ExecContext(ctx, statement); err != nil {
		return x.Translate(err)
	}
	return nil
}

// CreateTable declares the table and then describes its columns, which on two of these engines is
// more than one statement: a description there is `COMMENT ON COLUMN`, not a clause. See
// CreateTableStatements, which is what a dump writes as well.
func (x DDL) CreateTable(ctx context.Context, spec driver.TableSpec) error {
	for _, statement := range x.Dialect.CreateTableStatements(spec) {
		if err := x.exec(ctx, spec.Database, statement); err != nil {
			return err
		}
	}
	return nil
}

// RunAll executes statements in order, each in the database it names — which is what creating
// a database amounts to once its extensions and its comment are counted, since those can only
// be run from inside the database that has just been made.
func (x DDL) RunAll(ctx context.Context, statements []driver.DatabaseStatement) error {
	for _, statement := range statements {
		if err := x.exec(ctx, statement.In, statement.SQL); err != nil {
			return err
		}
	}
	return nil
}

func (x DDL) DropDatabase(ctx context.Context, name string) error {
	return x.exec(ctx, "", "DROP DATABASE "+x.Dialect.QuoteIdent(name))
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
	if err := x.exec(ctx, ns.Database, x.Dialect.CopyTable(ns, table, name)); err != nil {
		return err
	}
	if !withRows {
		return nil
	}
	return x.exec(ctx, ns.Database, x.Dialect.CopyRows(ns, table, name))
}

func (x DDL) DropTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.exec(ctx, ns.Database, "DROP TABLE "+x.Dialect.Table(ns, table))
}

func (x DDL) TruncateTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.exec(ctx, ns.Database, "TRUNCATE TABLE "+x.Dialect.Table(ns, table))
}

func (x DDL) EmptyTable(ctx context.Context, ns driver.Namespace, table string) error {
	return x.exec(ctx, ns.Database, "DELETE FROM "+x.Dialect.Table(ns, table))
}

func (x DDL) AddColumn(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ColumnSpec,
) error {
	return x.runAll(ctx, ns, x.Dialect.AddColumn(ns, table, spec))
}

// AddPartition, DropPartition and TruncatePartition each run exactly what AlterStatements would
// have shown, from the same speller. An engine that spells nothing has no way to address one piece
// of a table, and says so rather than reporting a change that never happened.
func (x DDL) AddPartition(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.PartitionSpec,
) error {
	return x.runOne(ctx, ns, "a partition", x.Dialect.AddPartition(ns, table, spec))
}

func (x DDL) DropPartition(ctx context.Context, ns driver.Namespace, table, partition string) error {
	return x.runOne(ctx, ns, "a partition", x.Dialect.DropPartition(ns, table, partition))
}

func (x DDL) TruncatePartition(
	ctx context.Context,
	ns driver.Namespace,
	table, partition string,
) error {
	return x.runOne(ctx, ns, "a partition", x.Dialect.TruncatePartition(ns, table, partition))
}

// RefreshMaterializedView recomputes the rows such a view stores. An engine with none spells
// nothing and says so, rather than succeeding at having done nothing.
func (x DDL) RefreshMaterializedView(ctx context.Context, ns driver.Namespace, name string) error {
	return x.runOne(ctx, ns, "a materialized view", x.Dialect.Refresh(ns, name))
}

// runOne executes a statement this engine had a grammar for, and answers `domain.Unsupported`
// **naming the thing** where it had none — which is what an empty spelling means everywhere in this
// package, and the answer rule 9 asks for: a reader is told what their engine has not got rather
// than being shown a statement the server refused.
func (x DDL) runOne(ctx context.Context, ns driver.Namespace, what, statement string) error {
	if statement == "" {
		return domain.Unsupported(what)
	}
	return x.exec(ctx, ns.Database, statement)
}

// runAll executes statements in order, stopping at the first failure. It is what a change that is
// more than one statement on some engines runs through — a column and the description beside it.
func (x DDL) runAll(ctx context.Context, ns driver.Namespace, statements []string) error {
	for _, statement := range statements {
		if err := x.exec(ctx, ns.Database, statement); err != nil {
			return err
		}
	}
	return nil
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
			statements = append(statements, x.Dialect.AddColumn(ns, table, change.Column)...)
		case driver.ChangeAlterColumn:
			statements = append(statements,
				x.Dialect.AlterColumn(ns, table, change.Name, change.Column)...)
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
		case driver.ChangeAddPartition:
			statements = appendSpelled(statements,
				x.Dialect.AddPartition(ns, table, change.Partition))
		case driver.ChangeDropPartition:
			statements = appendSpelled(statements, x.Dialect.DropPartition(ns, table, change.Name))
		case driver.ChangeTruncatePartition:
			statements = appendSpelled(statements,
				x.Dialect.TruncatePartition(ns, table, change.Name))
		}
	}

	return statements
}

// AddColumn is that statement as text rather than as a run, the arrangement AddForeignKey already
// has and for the same reason: a migration writes the clause the designer would have executed
// instead of a second spelling of it.
//
// It answers with a list because a column's *description* is a statement of its own on two of these
// engines — see ColumnComment, which is the whole of that difference.
//
// A column being **added** with nothing written about it spells no description at all, where an
// altered one always spells its own: there is nothing on a new column to clear. That is
// ColumnComments' rule and it is asked for by name rather than repeated here.
func (d Dialect) AddColumn(ns driver.Namespace, table string, spec driver.ColumnSpec) []string {
	added := "ALTER TABLE " + d.Table(ns, table) + " ADD COLUMN " + d.ColumnClause(spec)
	return append([]string{added}, d.ColumnComments(ns, table, []driver.ColumnSpec{spec})...)
}

// ColumnComment is what somebody wrote about a column, as the statement that records it — and the
// empty string on the engine that writes it as a clause of the column instead, which has already
// said it by the time this is asked. See ColumnCommentInline.
//
// A blank description is still spelled, because clearing one is a change like any other: what says
// "there is nothing written here" is the empty literal, and leaving the statement out would leave
// whatever was there before.
//
// The text is quoted as a string literal, which is the only thing between a reader's own words and
// a statement — DDL binds no arguments.
func (d Dialect) ColumnComment(ns driver.Namespace, table, column, comment string) string {
	if !d.CommentsOnObject {
		return ""
	}
	return "COMMENT ON COLUMN " + d.Table(ns, table) + "." + d.QuoteIdent(column) +
		" IS " + d.QuoteString(comment)
}

// ColumnComments is every description of a table's columns as its own statement, which is what
// follows a CREATE TABLE on an engine that keeps them separately. A column with nothing written
// about it contributes nothing here — unlike a *change* to one, where the empty literal is how a
// description is cleared.
func (d Dialect) ColumnComments(
	ns driver.Namespace,
	table string,
	columns []driver.ColumnSpec,
) []string {
	statements := []string{}
	for _, column := range columns {
		if strings.TrimSpace(column.Comment) == "" {
			continue
		}
		statements = appendSpelled(statements, d.ColumnComment(ns, table, column.Name, column.Comment))
	}
	return statements
}

// Refresh recomputes the rows a materialized view stores. It is the one statement about such a view
// that an ordinary one has no counterpart for, and an engine with no materialized views spells
// nothing — which is what the run beside it answers `domain.Unsupported` for.
func (d Dialect) Refresh(ns driver.Namespace, name string) string {
	if d.RefreshViewSQL == "" {
		return ""
	}
	if d.RefreshNamesString {
		return strings.Replace(d.RefreshViewSQL, "%s", d.QuoteString(qualifiedName(ns, name)), 1)
	}
	return strings.Replace(d.RefreshViewSQL, "%s", d.Table(ns, name), 1)
}

// qualifiedName is a namespace and an object as one **unquoted** name, which is what an engine that
// takes an object as a string argument wants: `SHOP.SALES`, inside a literal. Nothing else here
// writes a name this way, and nothing else should — see RefreshNamesString.
func qualifiedName(ns driver.Namespace, name string) string {
	where := ns.Database
	if ns.Schema != "" {
		where = ns.Schema
	}
	if where == "" {
		return name
	}
	return where + "." + name
}

// AddPartition declares one more piece of a partitioned table. The bounds are the reader's own text
// and travel verbatim, for the reason a column's type does: what a piece may hold is a different
// grammar per engine *and* per partitioning method, and a clause built here would only ever cover
// the methods somebody thought of.
//
// The three shapes are PartitionStyle's whole reason for existing, and an engine with no partition
// to address spells nothing.
func (d Dialect) AddPartition(
	ns driver.Namespace,
	table string,
	spec driver.PartitionSpec,
) string {
	bounds := strings.TrimSpace(spec.Bounds)

	switch d.Partitions {
	case PartitionAsTable:
		// A partition here **is** a table, so it is declared as one — and the table it belongs to is
		// named in the middle of the statement rather than in front of it.
		return "CREATE TABLE " + d.Table(ns, spec.Name) +
			" PARTITION OF " + d.Table(ns, table) + " " + bounds
	case PartitionInList:
		return "ALTER TABLE " + d.Table(ns, table) +
			" ADD PARTITION (PARTITION " + d.QuoteIdent(spec.Name) + " " + bounds + ")"
	case PartitionAsClause:
		return "ALTER TABLE " + d.Table(ns, table) +
			" ADD PARTITION " + d.QuoteIdent(spec.Name) + " " + bounds
	default:
		return ""
	}
}

// DropPartition removes a piece of a table **and the rows in it**, which is why it is previewed like
// every other change rather than run from a menu.
func (d Dialect) DropPartition(ns driver.Namespace, table, partition string) string {
	switch d.Partitions {
	case PartitionAsTable:
		return "DROP TABLE " + d.Table(ns, partition)
	case PartitionInList, PartitionAsClause:
		return "ALTER TABLE " + d.Table(ns, table) + " DROP PARTITION " + d.QuoteIdent(partition)
	default:
		return ""
	}
}

// TruncatePartition empties a piece and leaves it there, which is the difference TruncateTable has
// from DropTable one level up.
func (d Dialect) TruncatePartition(ns driver.Namespace, table, partition string) string {
	switch d.Partitions {
	case PartitionAsTable:
		return "TRUNCATE TABLE " + d.Table(ns, partition)
	case PartitionInList, PartitionAsClause:
		return "ALTER TABLE " + d.Table(ns, table) + " TRUNCATE PARTITION " + d.QuoteIdent(partition)
	default:
		return ""
	}
}

// appendSpelled adds a statement unless there was none to add, which is what an engine that has no
// grammar for a change contributes — nothing, rather than an empty statement the server would then
// be asked to run.
func appendSpelled(statements []string, statement string) []string {
	if statement == "" {
		return statements
	}
	return append(statements, statement)
}

func (d Dialect) DropColumn(ns driver.Namespace, table, column string) string {
	return "ALTER TABLE " + d.Table(ns, table) + " DROP COLUMN " + d.QuoteIdent(column)
}

// AlterColumn is the one statement of these that the engines do not merely spell differently but
// *shape* differently, and it answers with a list because on one of them the shape is two
// statements. MySQL and Oracle restate the column whole, so a fact left out of the clause is a fact
// dropped; PostgreSQL has no such clause and takes one action per fact, all in one ALTER TABLE so
// the column is never left half-changed; and SQL Server restates the type and the nullability but
// not the default, a default there being a constraint with a name of its own.
//
// An engine that cannot alter a column at all contributes **nothing**, which is what an empty list
// means everywhere in AlterStatements — the run of that same change is where `domain.Unsupported`
// is answered.
//
// Every shape names the column **being changed** rather than the name in the spec, so renaming is
// RenameColumn's alone and never a side effect of an edit.
func (d Dialect) AlterColumn(
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) []string {
	if d.KeepsColumnAsDeclared {
		return nil
	}

	head := "ALTER TABLE " + d.Table(ns, table) + " "
	quoted := d.QuoteIdent(column)
	fallen := strings.TrimSpace(defaultOf(spec))
	// The description follows the change wherever it is a statement of its own, and is already in
	// the restated clause on the engine that writes it as one — see ColumnComment.
	described := d.ColumnComment(ns, table, column, spec.Comment)

	if d.RestatesColumn {
		restated := spec
		restated.Name = column
		if d.AltersColumnOnly {
			restated.Default = nil
		}

		clause := d.RestateClause
		if clause == "" {
			clause = "MODIFY COLUMN"
		}
		statements := []string{head + clause + " " + d.ColumnClause(restated)}

		// A default that could not travel in the clause is the second statement, and it is an *add*
		// rather than a change: what is already there has a name only a catalogue read knows, and a
		// spelling cannot read one. The server says so if there is one, which is the honest outcome.
		if d.SeparatesDefault && fallen != "" {
			statements = append(statements, head+"ADD DEFAULT "+fallen+" FOR "+quoted)
		}
		return appendSpelled(statements, described)
	}

	actions := []string{"ALTER COLUMN " + quoted + " TYPE " + strings.TrimSpace(spec.Type)}

	if spec.Nullable && !spec.PrimaryKey {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP NOT NULL")
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" SET NOT NULL")
	}

	if fallen != "" {
		actions = append(actions, "ALTER COLUMN "+quoted+" SET DEFAULT "+fallen)
	} else {
		actions = append(actions, "ALTER COLUMN "+quoted+" DROP DEFAULT")
	}

	return appendSpelled([]string{head + strings.Join(actions, ", ")}, described)
}

// defaultOf is the expression a column is to fall back to, and empty for a column that falls back
// to nothing. It is one function because "no default" reaches here two ways — a nil pointer and a
// pointer to nothing — and both mean the same thing to every statement that reads one.
func defaultOf(spec driver.ColumnSpec) string {
	if spec.Default == nil {
		return ""
	}
	return strings.TrimSpace(*spec.Default)
}

// CreateIndex differs between the engines only in where the access method goes: MySQL takes it
// between the name and the table, PostgreSQL after the table. An empty method takes the engine's
// own default, and reaches the statement verbatim the way a column's type does.
func (d Dialect) CreateIndex(ns driver.Namespace, table string, spec driver.IndexSpec) string {
	statement := "CREATE "
	if spec.Unique {
		statement += "UNIQUE "
	}
	statement += "INDEX " + d.indexName(ns, spec.Name)

	method := strings.TrimSpace(spec.Method)
	using := ""
	if method != "" {
		using = " USING " + method
	}

	target := d.Table(ns, table)
	if d.ObjectTableIsBare {
		target = d.QuoteIdent(table)
	}

	if d.MethodBeforeTable {
		statement += using + " ON " + target
	} else {
		statement += " ON " + target + using
	}

	return statement + " (" + strings.Join(d.QuoteAll(spec.Columns), ", ") + ")"
}

// DropIndex has three spellings, which is one more than most things here: an index is dropped by
// its own name where it is an object of the schema (PostgreSQL, Oracle), by its name *and* its
// table where it is an object belonging to one (SQL Server), and through an ALTER TABLE where it is
// part of one (MySQL).
func (d Dialect) DropIndex(ns driver.Namespace, table, index string) string {
	switch {
	case d.DropsIndexOnTable:
		return "DROP INDEX " + d.QuoteIdent(index) + " ON " + d.Table(ns, table)
	case d.IndexIsObject:
		return "DROP INDEX " + d.Table(ns, index)
	default:
		return "ALTER TABLE " + d.Table(ns, table) + " DROP INDEX " + d.QuoteIdent(index)
	}
}

// indexName is the name a create gives an index. PostgreSQL refuses a qualified one — the index
// goes wherever its table is — where Oracle takes it and would otherwise put the index in whichever
// schema the reader happened to connect as.
func (d Dialect) indexName(ns driver.Namespace, name string) string {
	if d.QualifiesIndexName {
		return d.Table(ns, name)
	}
	return d.QuoteIdent(name)
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
	return x.exec(ctx, ns.Database, statement)
}

func (x DDL) DropColumn(ctx context.Context, ns driver.Namespace, table, column string) error {
	return x.exec(ctx, ns.Database, x.Dialect.DropColumn(ns, table, column))
}

// AlterColumn, CreateIndex, DropIndex and DropForeignKey are here rather than in each adapter
// because the Dialect above now spells all four: what differs between the engines is a *flag* —
// whether a column is restated whole, where an index's method goes, whether an index is an object
// of the schema, and what the clause dropping a constraint is called — and a flag is the form every
// other difference between the two takes.
// AlterColumn runs what AlterStatements would have shown, in the same order and from the same
// speller. An engine that spells nothing has no way to change a column, and says so rather than
// reporting a change that never happened.
func (x DDL) AlterColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column string,
	spec driver.ColumnSpec,
) error {
	statements := x.Dialect.AlterColumn(ns, table, column, spec)
	if len(statements) == 0 {
		return domain.Unsupported("changing a column")
	}
	return x.runAll(ctx, ns, statements)
}

func (x DDL) CreateIndex(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.IndexSpec,
) error {
	return x.exec(ctx, ns.Database, x.Dialect.CreateIndex(ns, table, spec))
}

func (x DDL) DropIndex(ctx context.Context, ns driver.Namespace, table, index string) error {
	return x.exec(ctx, ns.Database, x.Dialect.DropIndex(ns, table, index))
}

func (x DDL) DropForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table, constraint string,
) error {
	return x.exec(ctx, ns.Database, x.Dialect.DropForeignKey(ns, table, constraint))
}

// CreateForeignKey is spelled the same by both engines once the quoting is the dialect's, and
// the referential actions reach the statement verbatim the way a column's type does.
func (x DDL) CreateForeignKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	spec driver.ForeignKeySpec,
) error {
	return x.exec(ctx, ns.Database, x.Dialect.AddForeignKey(ns, table, spec))
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
// they are one method — and all three carry `Batch` rows to a statement where the engine takes
// several tuples at once, which is every engine but Oracle.
//
// An engine with neither merge clause writes those two modes a row at a time instead: see
// rowsGuarded, which is what MergeByExists means.
func (x DDL) rowsAdded(target string, write driver.RowWrite) []string {
	if x.Dialect.MergeByExists && write.Mode != driver.WriteAppend {
		return x.rowsGuarded(target, write)
	}

	head := x.Dialect.InsertVerb(write.Mode) + " INTO " + target +
		" (" + strings.Join(x.Dialect.QuoteAll(write.Columns), ", ") + ") VALUES "
	tail := x.Dialect.MergeClause(write.Mode, write.Keys, write.Columns)

	batch := max(write.Batch, 1)
	if !x.Dialect.MultiRowValues {
		batch = 1
	}
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

// rowsGuarded is a merge on an engine that has no clause for one: an insert that looks first, and
// for an upsert an update in front of it. It is a row at a time because it has to be — the
// condition names *this* row's key — and the update comes first so a row already there is changed
// once and then not inserted.
//
// Neither engine's MERGE is used. A MERGE says the same thing in a page of syntax, and one of the
// two spends that page arguing about which source the rows came from.
func (x DDL) rowsGuarded(target string, write driver.RowWrite) []string {
	columns := strings.Join(x.Dialect.QuoteAll(write.Columns), ", ")

	statements := make([]string, 0, len(write.Rows)*2)
	for _, row := range write.Rows {
		match := x.match(write, row)
		if write.Mode == driver.WriteUpsert {
			if changed, ok := x.assign(write, row); ok {
				statements = append(statements,
					"UPDATE "+target+" SET "+changed+" WHERE "+match)
			}
		}
		statements = append(statements,
			"INSERT INTO "+target+" ("+columns+")"+
				" SELECT "+strings.Join(x.literals(row), ", ")+x.Dialect.From()+
				" WHERE NOT EXISTS (SELECT 1 FROM "+target+" WHERE "+match+")")
	}
	return statements
}

// rowsChanged is one UPDATE per row. It cannot be batched the way an insert can — every row sets
// different values on a different key — and a key column is left out of the SET, since changing the
// thing a row is found by is a rename rather than an update.
func (x DDL) rowsChanged(target string, write driver.RowWrite) []string {
	statements := make([]string, 0, len(write.Rows))
	for _, row := range write.Rows {
		changed, ok := x.assign(write, row)
		if !ok {
			continue
		}
		statements = append(statements,
			"UPDATE "+target+" SET "+changed+" WHERE "+x.match(write, row))
	}
	return statements
}

// assign is the SET of one row's update: every column that is not part of the key. It says whether
// there was anything to set, because a row that is entirely its own key has nothing to change and
// an `UPDATE … SET  WHERE …` is not a statement.
func (x DDL) assign(write driver.RowWrite, row []*string) (string, bool) {
	keys := set(write.Keys)

	assignments := []string{}
	for at, name := range write.Columns {
		if keys[name] || at >= len(row) {
			continue
		}
		assignments = append(assignments, x.Dialect.QuoteIdent(name)+" = "+x.literal(row[at]))
	}
	if len(assignments) == 0 {
		return "", false
	}
	return strings.Join(assignments, ", "), true
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
			Comment:    column.Comment,
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

// DropSource removes a view, a routine or a trigger, and is one statement for all four: what
// differs between them is the word naming the kind, which the kind itself supplies.
//
// The two fields beside the name are each one engine's way of saying *which* object, and each
// reaches the statement only where that engine needs it. The arguments tell two PostgreSQL
// functions of one name apart; the table is what PostgreSQL finds a trigger by, a trigger name
// there being unique per table rather than per schema.
func (d Dialect) DropSource(ns driver.Namespace, ref driver.SourceRef) string {
	if ref.Kind == driver.SourceTrigger {
		statement := "DROP TRIGGER " + d.TriggerName(ns, ref.Name)
		if d.TriggerOnTable {
			statement += " ON " + d.TriggerTable(ns, ref.Table)
		}
		return statement
	}

	statement := "DROP " + ref.Kind.Keyword() + " " + d.Table(ns, ref.Name)
	if ref.Arguments != "" {
		statement += "(" + ref.Arguments + ")"
	}
	return statement
}

// TriggerName is how a trigger is addressed, and is the one object name here that is not the one
// `Table` writes — for a different reason on each of the two engines that differ. A PostgreSQL
// trigger belongs to its *table*, so its own name carries no namespace at all and the table is
// named beside it; a SQL Server trigger belongs to its table's schema, and its statements take two
// parts where every other statement this adapter writes takes three.
func (d Dialect) TriggerName(ns driver.Namespace, name string) string {
	switch {
	case d.TriggerOnTable:
		return d.QuoteIdent(name)
	case d.NamespaceIsBoth:
		return d.Qualify(ns.Schema, name)
	default:
		return d.Table(ns, name)
	}
}

// TriggerTable is the table a trigger fires on, spelled the way a *trigger's* statements spell one
// — which is not how a read spells it on two engines. SQLite puts the schema on the trigger and
// refuses one here, exactly as it does for an index; SQL Server takes two parts where every other
// statement this app writes for it takes three, `CREATE TRIGGER` applying to the database the
// connection is already in.
func (d Dialect) TriggerTable(ns driver.Namespace, table string) string {
	switch {
	case d.ObjectTableIsBare:
		return d.QuoteIdent(table)
	case d.NamespaceIsBoth:
		return d.Qualify(ns.Schema, table)
	default:
		return d.Table(ns, table)
	}
}

func (x DDL) DropSource(ctx context.Context, ns driver.Namespace, ref driver.SourceRef) error {
	return x.runInside(ctx, ns, []string{x.Dialect.DropSource(ns, ref)})
}

// SourceStatements spells saving a definition: the drop of whatever is there now, and then the
// reader's own declaration. Two statements rather than a `CREATE OR REPLACE` because the second
// of them is the reader's text and nothing here rewrites it — an engine that can replace in place
// does so because the definition it answered with already said so, not because this added a word.
//
// A definition that is blank spells **nothing**, which is what the preview of an empty editor
// honestly is; what refuses to save one is the API layer, in front of this.
func (x DDL) SourceStatements(ns driver.Namespace, save driver.SourceSave) []string {
	definition := strings.TrimSpace(save.Definition)
	if definition == "" {
		return nil
	}
	if strings.TrimSpace(save.Replaces.Name) == "" {
		return []string{definition}
	}
	return []string{x.Dialect.DropSource(ns, save.Replaces), definition}
}

// NewSource is the skeleton this engine declares one of these with — see Dialect.Starters, which
// is where the four shapes live.
func (x DDL) NewSource(ns driver.Namespace, ref driver.SourceRef) string {
	return x.Dialect.NewSource(ns, ref)
}

// SaveSource runs what SourceStatements showed, in order and stopping at the first failure. A
// drop that succeeded in front of a create that did not leaves the object gone, which is why the
// preview shows both statements and why the reader sees the drop before pressing Save.
func (x DDL) SaveSource(ctx context.Context, ns driver.Namespace, save driver.SourceSave) error {
	return x.runInside(ctx, ns, x.SourceStatements(ns, save))
}

// runInside executes a definition's statements in order, stopping at the first failure, on a
// connection put inside the namespace where the adapter has one to give — see Pin, which is the
// whole reason a definition is not run the way every other statement here is.
//
// There is no transaction around them, DDL not being transactional on three of these five engines,
// which is why the preview has to be exact.
func (x DDL) runInside(ctx context.Context, ns driver.Namespace, statements []string) error {
	if len(statements) == 0 {
		return nil
	}

	if x.Pin == nil {
		for _, statement := range statements {
			if err := x.exec(ctx, ns.Database, statement); err != nil {
				return err
			}
		}
		return nil
	}

	conn, release, err := x.Pin(ctx, ns)
	if err != nil {
		return err
	}
	defer release()

	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return x.Translate(err)
		}
	}
	return nil
}

func (x DDL) alter(ns driver.Namespace, table string) string {
	return "ALTER TABLE " + x.Dialect.Table(ns, table)
}

// ScanTriggers folds a two-column catalogue read — a trigger's name, then the table it fires on —
// into the list `Objects` carries. It is here rather than in each adapter because what differs
// between the five engines is the *read*, and the fold does not differ at all.
//
// The table is scanned as a pointer because a trigger need not have one: Oracle's may be on the
// schema and SQL Server's on the database, and either is a trigger that exists and fires on no
// table. It is listed by the name it has rather than left out.
func ScanTriggers(rows *sql.Rows) ([]driver.Trigger, error) {
	defer rows.Close()

	triggers := []driver.Trigger{}
	for rows.Next() {
		var found driver.Trigger
		var table *string
		if err := rows.Scan(&found.Name, &table); err != nil {
			return nil, err
		}
		if table != nil {
			found.Table = *table
		}
		triggers = append(triggers, found)
	}
	return triggers, rows.Err()
}

// ScanPartitions folds a five-column catalogue read — a partition's name, the method the table is
// cut by, the expression it is cut on, what this piece holds, and how many rows the server thinks
// are in it — into the list a `TableStructure` carries. It is here rather than in each adapter
// because what differs between the engines is the *read*, and the fold does not differ at all.
//
// Every column but the name is scanned as a pointer, because every one of them is a fact a
// catalogue may not have: a hash partition is cut on no expression it will name, a piece of a
// default partition holds no bounds, and a row count is NULL until something has gathered
// statistics. A partition is listed by the name it has rather than left out.
func ScanPartitions(rows *sql.Rows) ([]driver.Partition, error) {
	defer rows.Close()

	partitions := []driver.Partition{}
	for rows.Next() {
		var found driver.Partition
		var method, expression, bounds *string
		if err := rows.Scan(&found.Name, &method, &expression, &bounds, &found.Rows); err != nil {
			return nil, err
		}
		found.Method = textOf(method)
		found.Expression = textOf(expression)
		found.Bounds = strings.TrimSpace(textOf(bounds))
		partitions = append(partitions, found)
	}
	return partitions, rows.Err()
}

// textOf is a catalogue column that may not be there at all, as the empty string every reader of
// one wants: a hash partition is cut on no expression the server will name, and a default piece
// holds no bounds.
func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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

// Table names a table the way the engine addresses it: inside its database on MySQL and Oracle,
// inside its schema on PostgreSQL, and inside both on SQL Server — which is what lets one pool
// there read every database on the server.
func (d Dialect) Table(ns driver.Namespace, table string) string {
	if d.NamespaceIsBoth && ns.Database != "" {
		return d.QuoteIdent(ns.Database) + "." + d.Qualify(ns.Schema, table)
	}
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

// CreateTableStatements is the whole of declaring a table: the table itself, and then what is
// written about its columns on an engine that keeps a description separately. It is one method so
// that a dump and a create run the same statements — the arrangement AlterStatements has.
func (d Dialect) CreateTableStatements(spec driver.TableSpec) []string {
	return append(
		[]string{d.CreateTable(spec)},
		d.ColumnComments(spec.Namespace, spec.Table, spec.Columns)...,
	)
}

// CopyTable declares a table shaped like another one, and the engines have four words for it.
// MySQL's LIKE is a clause of its own and brings the defaults and the indexes with it;
// PostgreSQL's stands where a column would and brings the columns alone until INCLUDING ALL asks
// for the rest; SQLite and Oracle have no LIKE and *read* the other table for its shape; and SQL
// Server reads it too but puts the new name in the middle of the read.
//
// The three that read take the columns and their types and nothing else — no key, no index, no
// default — which is the honest cost of an engine with no way to declare "like that one".
func (d Dialect) CopyTable(ns driver.Namespace, table, name string) string {
	source := d.Table(ns, table)
	target := d.Table(ns, name)

	switch {
	case d.SelectIntoCopy:
		return "SELECT * INTO " + target + " FROM " + source + " WHERE 1 = 0"
	case d.CopyAsSelect:
		return "CREATE TABLE " + target + " AS SELECT * FROM " + source + " WHERE 1 = 0"
	case d.LikeInColumnList:
		return "CREATE TABLE " + target + " (LIKE " + source + " INCLUDING ALL)"
	default:
		return "CREATE TABLE " + target + " LIKE " + source
	}
}

// CopyRows is every row of a table on its way into a copy of it. PostgreSQL's copy took the
// original's identity column with it and then refuses a value for that column, so the write says
// out loud that it is overriding one — a table with no identity column takes the same clause
// without complaint, which is what keeps this from needing to read the table first. A *generated*
// column is the one thing no engine will be written, and says so itself.
func (d Dialect) CopyRows(ns driver.Namespace, table, name string) string {
	statement := "INSERT INTO " + d.Table(ns, name)
	if d.OverridesIdentity {
		statement += " OVERRIDING SYSTEM VALUE"
	}
	return statement + " SELECT * FROM " + d.Table(ns, table)
}

// InsertVerb is how this engine opens an insert that is not to fail on a row already there. MySQL
// has a verb for it; PostgreSQL and SQLite say the same thing in a clause at the other end of the
// statement, so their verb never changes — see MergeClause — and the two with neither never reach
// here for that mode at all, an insert of theirs looking first instead (see rowsGuarded).
func (d Dialect) InsertVerb(mode driver.RowWriteMode) string {
	if mode == driver.WriteSkip && !d.MergesOnConflict && !d.MergeByExists {
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
	// A blank description writes no clause, and clears one all the same: the engine that keeps a
	// description here is the engine that restates a column whole, so a comment left out is a
	// comment dropped — which is exactly what clearing it means.
	if d.ColumnCommentInline && strings.TrimSpace(spec.Comment) != "" {
		clause += " COMMENT " + d.QuoteString(spec.Comment)
	}
	return clause
}
