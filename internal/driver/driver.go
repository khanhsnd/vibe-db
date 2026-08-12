package driver

import (
	"context"
	"net"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

type Credentials struct {
	Password string
}

// Dialer opens the network connection a session is carried over. It is nil for a profile that
// reaches its server directly, and a tunnel's own dial for one that goes through a bastion —
// which is the whole reason neither adapter has ever heard of SSH: the tunnel is stood up above
// this layer and arrives as a function.
//
// It takes the address the *engine* asked for, resolved on the far side of whatever carries it,
// so a server bound to loopback on a bastion is still `127.0.0.1` in the profile.
type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

type ServerInfo struct {
	Version string
}

type Column struct {
	Name       string
	Type       string
	FullType   string
	Nullable   bool
	PrimaryKey bool
	Default    *string
}

// Namespace is where a table lives. MySQL has one level and fills Database alone;
// PostgreSQL has two and fills both, since a table is addressed by a schema inside the
// database the session is bound to.
type Namespace struct {
	Database string
	Schema   string
}

// TableSort orders a table read by one of its columns. A read carrying none leaves the primary
// key in force, which is what a page is cut from when the reader has chosen nothing.
type TableSort struct {
	Column     string
	Descending bool
}

// RowStream is how a read of every row is delivered while it runs, and watched while it is
// delivered. The two halves travel at different rates on purpose:
//
//   - Rows takes each batch as it fills, which is how a table too large to answer in one piece
//     is handed over — and how the read stays one batch of memory rather than a whole table.
//   - Counted says how far the read has got, on a clock rather than per batch. A batch fills on a
//     round number, so a count that moved only with one would step in thousands instead of
//     climbing, which is what a reader watching a long read is actually looking at.
//
// Both are called from the goroutine doing the scan, so whatever they do is time the read spends
// waiting. A read given no stream answers with its rows instead.
type RowStream struct {
	Rows    func(batch [][]*string)
	Counted func(fetched int)
}

// TableFilterOp is one comparison a filter makes, named by a fixed token rather than spelled
// as the operator itself: nothing a reader typed then reaches the statement as SQL, and an
// operator this side does not know is refused instead of pasted in.
type TableFilterOp string

const (
	FilterEqual        TableFilterOp = "eq"
	FilterNotEqual     TableFilterOp = "ne"
	FilterGreater      TableFilterOp = "gt"
	FilterGreaterEqual TableFilterOp = "ge"
	FilterLess         TableFilterOp = "lt"
	FilterLessEqual    TableFilterOp = "le"

	// The six text matches read the column as text whatever its own type is, so one condition
	// serves a name and a number alike, and a wildcard the reader typed is their own.
	FilterContains    TableFilterOp = "contains"
	FilterNotContains TableFilterOp = "notcontains"
	FilterBegins      TableFilterOp = "begins"
	FilterNotBegins   TableFilterOp = "notbegins"
	FilterEnds        TableFilterOp = "ends"
	FilterNotEnds     TableFilterOp = "notends"

	// The four that take no value at all. Empty is the empty string, which is a value a column
	// holds; Null is the absence of one, and the two are not the same question.
	FilterNull     TableFilterOp = "isnull"
	FilterNotNull  TableFilterOp = "notnull"
	FilterEmpty    TableFilterOp = "empty"
	FilterNotEmpty TableFilterOp = "notempty"

	// Between takes two values and the list operators take any number above none.
	FilterBetween    TableFilterOp = "between"
	FilterNotBetween TableFilterOp = "notbetween"
	FilterInList     TableFilterOp = "inlist"
	FilterNotInList  TableFilterOp = "notinlist"

	// The two the filter builder does not offer, because they are the shape of an engine that
	// matches text its own way rather than of a `WHERE`: Matches reads the value as a **pattern in
	// the engine's own notation** — a glob on a key space, which is the one narrowing a Redis server
	// will do — and ContainsFold is Contains with case folded away. An engine that has neither
	// refuses them like any other operator it does not know, and nothing writes them there: the
	// surface that does is the key space's own search bar.
	FilterMatches      TableFilterOp = "matches"
	FilterContainsFold TableFilterOp = "containsfold"
)

// TableConjunction joins the children of a group. There are two of them, and one this side does
// not know is refused rather than guessed at.
type TableConjunction string

const (
	ConjunctionAnd TableConjunction = "and"
	ConjunctionOr  TableConjunction = "or"
)

// TableCriterion is one node of the tree a table read is narrowed by: either a TableFilter on one
// column or a TableGroup of further nodes. A tree rather than a list because a reader who means
// `a AND (b OR c)` cannot say it in a list, and those parentheses are the whole point of a group.
type TableCriterion interface {
	criterion()
}

// TableFilter narrows a table read to the rows one column satisfies. Values is what the operator
// compares against, and how many it takes is the operator's own business: none for IS NULL, one
// for most, two for BETWEEN, and any number for IN.
type TableFilter struct {
	Column string
	Op     TableFilterOp
	Values []string
}

func (TableFilter) criterion() {}

// TableGroup is the parentheses: its children joined by one conjunction. A group of nothing
// narrows nothing, which is what an unfiltered read carries.
type TableGroup struct {
	Conjunction TableConjunction
	Children    []TableCriterion
}

func (TableGroup) criterion() {}

// TablePage is one window onto a table's rows. A Limit of zero or less reads every row and
// makes Offset meaningless, the same convention as Query.MaxRows.
type TablePage struct {
	Namespace
	Table string
	Limit int
	// Filter narrows the read, and narrows its total with it: a page is cut from the rows the
	// tree admits rather than from the table.
	Filter TableGroup
	Offset int
	// Sort orders the read, the first key deciding and the rest breaking its ties.
	Sort []TableSort
	// Stream, when set, takes the rows as they are read and the read then answers with none of
	// them. A page that leaves it nil is answered whole, which is what every bounded read is.
	Stream *RowStream
	// Whole asks for values as they *are* rather than for as much of one as a row can carry. It
	// means nothing on an engine that never samples a value, which is every engine but the key-value
	// one — a Redis container is drawn as its first fifty members precisely so that a page of a
	// hundred keys cannot be a million members (see `redis.sampled`).
	//
	// It is set by a read of **one** key that is going to be looked at whole, which is what the panel
	// under a key space is. A page never sets it: the bound is what makes the grid answerable, and
	// lifting it for a hundred rows at once would be lifting it for the reason it exists.
	Whole bool
}

// RowSearch is one table of a Find in Schema's *data* half: how many of its rows hold the reader's
// text in any of their columns. One table at a time, because that is what the answer is reported
// per — a search over a namespace is this asked once per table, and a table nobody may read is one
// row missing from the answer rather than the whole search failing.
//
// The other half of a search asks nothing of a driver: a field, an index and a constraint are names
// the catalogue reads already answer with, and matching one against the text is domain.TextMatch's.
type RowSearch struct {
	Namespace
	Table string
	Match domain.TextMatch
}

// RowMatches is what one table's data search found: how many of its rows hold the text, and the read
// that would return them.
//
// The two are spelled from **one condition**, which is the whole reason the statement is here rather
// than anywhere else: the number the reader is shown and the rows they open cannot be about different
// rows. It is the same arrangement TableRows.Statement has, and the same one a migration's preview has.
type RowMatches struct {
	Count     int64
	Statement string
}

// ColumnPick names the column a filter's value is picked from, rather than typed into. Limit caps
// the answer, because a column of a million rows is not a list anybody scrolls.
type ColumnPick struct {
	Namespace
	Table  string
	Column string
	Limit  int
}

// TableSchema is the completion-sized view of a table: names only, no types, because
// the editor needs every table of a database at once and nothing more.
type TableSchema struct {
	Name    string
	Columns []string
}

// Routine is one stored function or procedure. Arguments is the identity argument list, which
// is what PostgreSQL needs to tell two overloads of one name apart and what MySQL, whose
// routine names are unique, leaves empty.
//
// It is a *listing*, and what every statement about one is addressed by is a SourceRef built from
// it — which is the frontend's to build, the tree's row being where those facts are held.
type Routine struct {
	Name      string
	Procedure bool
	Arguments string
}

// Trigger is one trigger of a namespace, named beside the table it fires on. A trigger is the one
// declared object that is *about* another, so the table travels with the name everywhere the name
// goes: PostgreSQL will not drop one without being told which table it is on, and no engine will
// declare one without being told either.
type Trigger struct {
	Name  string
	Table string
}

// SourceKind is which of the four declared objects a definition belongs to. It is one value
// rather than a flag beside a flag because they differ in exactly one word — the one that
// drops them — and a caller that has the kind has the word.
type SourceKind string

const (
	SourceView      SourceKind = "view"
	SourceFunction  SourceKind = "function"
	SourceProcedure SourceKind = "procedure"
	SourceTrigger   SourceKind = "trigger"
)

// Keyword is the word the engines name this kind of object by, and is total on purpose: a kind
// that arrived as something else is a view rather than text on its way into a statement. What
// refuses an unknown kind outright is the API layer, before it ever reaches here.
func (k SourceKind) Keyword() string {
	switch k {
	case SourceProcedure:
		return "PROCEDURE"
	case SourceFunction:
		return "FUNCTION"
	case SourceTrigger:
		return "TRIGGER"
	default:
		return "VIEW"
	}
}

// SourceRef names one view, one routine or one trigger. The two fields beside the name are each
// one engine's way of saying which one: Arguments is PostgreSQL's overload discriminator, since
// two of its functions may share a name, and Table is the table a trigger fires on, since a
// trigger is declared and dropped *about* a table rather than on its own. Both are empty wherever
// the name already identifies the object.
type SourceRef struct {
	Kind      SourceKind
	Name      string
	Arguments string
	Table     string
}

// Source is one view or one routine as the statement that declares it: what a reader is shown,
// and — edited — what a save runs.
//
// The definition is the **server's own words** wherever the engine has them, for the reason
// CreateStatements is a read rather than a render: a declaration put back together out of a
// catalogue is only ever as complete as the catalogue's columns. Where an engine keeps the body
// alone — PostgreSQL and Oracle both answer for a view with its SELECT and nothing else — the
// adapter puts the CREATE in front of it, which is the driver's to spell like every other statement.
type Source struct {
	SourceRef
	Definition string
}

// SourceSave is a definition on its way back to a server. `Replaces` is what is there now, and a
// blank name in it means there is nothing there yet — which is the whole difference between
// declaring an object and altering one, a definition being replaced by being dropped and made again.
type SourceSave struct {
	Replaces   SourceRef
	Definition string
}

// Objects is everything a namespace holds that the tree draws a folder for. They are read
// together for the reason a table's shape is: expanding any one folder needs every folder's
// count, so one read answers all three.
type Objects struct {
	Tables   []string
	Views    []string
	Routines []Routine
	// Triggers is every trigger of the namespace rather than of one table, because that is the
	// level the catalogue answers at on all five engines and the level the tree draws a folder for.
	Triggers []Trigger
	// KeyCount is how many keys the namespace holds, on an engine whose namespace is a key space.
	// It is what "how much is in here" means there — the lists above are one fixed table and say
	// nothing — and is zero on every engine that answers the question by being listed.
	KeyCount int64
}

// TableList is one row per table of a namespace: its name in the first column, then whatever
// else this engine's catalogue keeps about it. The columns are the engine's own, for the reason
// DatabaseOptions' lists are — no two engines keep the same facts about a table, and a column
// neither could fill would say nothing — so a size or a date is already spelled the way the
// server spells it. Only the first column is promised, being what the list is acted on by.
type TableList struct {
	Columns []Column
	Rows    [][]*string
}

// Index is one index of a table, the one behind the primary key included, because that is
// what the catalogue reports and what the tree lists.
type Index struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
	Method  string
}

// ForeignKey is one referential constraint. The referenced table carries its own namespace
// part, since a key may point across a schema.
type ForeignKey struct {
	Name              string
	Columns           []string
	ReferencedSchema  string
	ReferencedTable   string
	ReferencedColumns []string
	OnUpdate          string
	OnDelete          string
}

// TableStructure is a table's shape rather than its rows: what the tree draws underneath it
// and what the designer shows.
type TableStructure struct {
	Columns     []Column
	Indexes     []Index
	ForeignKeys []ForeignKey
}

// ColumnSpec is a column as the user declares it, which is not what Column reports back.
// Type is the text the engine will parse — `varchar(255)`, `numeric(10,2)` — and Default is
// an expression rather than a value, so both travel into the statement verbatim: neither can
// be bound as an argument, because DDL takes no parameters.
type ColumnSpec struct {
	Name       string
	Type       string
	Nullable   bool
	Default    *string
	PrimaryKey bool
}

// TableSpec is a CREATE TABLE: a name and its columns, its primary key taken from whichever
// of them claim one.
type TableSpec struct {
	Namespace
	Table   string
	Columns []ColumnSpec
}

// IndexSpec is an index as the designer declares it. Method is the access method and reaches
// the statement verbatim, the way a column's type does; an empty one takes the engine's default.
type IndexSpec struct {
	Name    string
	Columns []string
	Unique  bool
	Method  string
}

// ForeignKeySpec is a referential constraint as the designer declares it. ReferencedSchema is
// whichever part of a namespace this engine qualifies a table by — a schema on PostgreSQL, a
// database on MySQL — and is empty for a table in the same namespace as the one holding the key.
type ForeignKeySpec struct {
	Name              string
	Columns           []string
	ReferencedSchema  string
	ReferencedTable   string
	ReferencedColumns []string
	OnUpdate          string
	OnDelete          string
}

// TableChangeKind is one thing that can be done to a table's shape. The list is closed and a kind
// this side does not know is refused rather than guessed at, exactly as a filter's operator is.
//
// There is no rename here, and no primary key. Both exist as methods on Schema and neither belongs
// in a *migration*: a rename cannot be told from a drop and an add by comparing two catalogues, and
// a primary key is a fact of the fields rather than a change of its own.
type TableChangeKind string

const (
	ChangeAddColumn        TableChangeKind = "addColumn"
	ChangeAlterColumn      TableChangeKind = "alterColumn"
	ChangeDropColumn       TableChangeKind = "dropColumn"
	ChangeCreateIndex      TableChangeKind = "createIndex"
	ChangeDropIndex        TableChangeKind = "dropIndex"
	ChangeCreateForeignKey TableChangeKind = "createForeignKey"
	ChangeDropForeignKey   TableChangeKind = "dropForeignKey"
)

// TableChange is one change to a table's shape, in the form both spelling it and running it take.
// Name is what the change is addressed to — the column, index or constraint — and only the field
// belonging to the kind is read.
type TableChange struct {
	Kind   TableChangeKind
	Name   string
	Column ColumnSpec
	Index  IndexSpec
	Key    ForeignKeySpec
}

// DatabaseSpec is a CREATE DATABASE as the dialog declares it. How much of it an engine reads
// is the engine's own business: MySQL knows only Encoding and Collation, PostgreSQL knows most of
// the rest, and MongoDB knows only the name and the collection.
type DatabaseSpec struct {
	Name string
	// FirstCollection is the collection a new database is made with, and is MongoDB's alone.
	// A document store has no CREATE DATABASE: a database exists exactly as long as it holds a
	// collection, so making one *is* making its first collection — which is why the one engine that
	// needs this asks for it rather than inventing a name the reader would then have to find.
	FirstCollection  string
	Owner            string
	Template         string
	Encoding         string
	Collation        string
	CType            string
	Tablespace       string
	ConnectionLimit  int
	AllowConnections bool
	IsTemplate       bool
	Comment          string
	Extensions       []string
}

// DatabaseStatement is one statement of a create and the database it has to run in. An empty
// In runs it wherever the session already is, which is the only place CREATE DATABASE can go —
// the database it makes cannot be connected to until it exists.
type DatabaseStatement struct {
	In  string
	SQL string
}

// DatabaseOptions is what the New Database dialog fills its lists from. An engine leaves empty
// whatever it has no such concept of.
type DatabaseOptions struct {
	Owners      []string
	Templates   []string
	Encodings   []string
	Collations  []string
	Tablespaces []string
	Extensions  []string
}

// TableRows is one window onto a table's rows. Statement is the read that produced them,
// spelled with its own bounds inline, so the reader can be shown exactly what ran.
type TableRows struct {
	Columns   []Column
	Rows      [][]*string
	Total     int64
	Statement string
}

type Query struct {
	Namespace
	SQL     string
	MaxRows int
	Explain bool
}

// QueryResult carries its Namespace and Table only when its rows map one-to-one onto the
// rows of a single table; both stay empty for a result that cannot be written back.
type QueryResult struct {
	Namespace
	Columns      []Column
	Rows         [][]*string
	RowsAffected int64
	Truncated    bool
	Table        string
	// Plan is what the server said it would do, and is filled only for a Query that asked to be
	// explained. Explain runs the statement like any other and asks about it as well, so the Rows
	// above are the statement's own and the plan is *about* that run rather than instead of it.
	Plan *Plan
}

// PlanNode is one step of a plan. Depth is how deep in the plan's tree the step sits and the steps
// arrive in the order the tree reads, so a flat list is the whole tree — which is what the grid that
// draws it needs, a grid having rows and not branches.
type PlanNode struct {
	// Type is the engine's own name for the step: `Seq Scan`, `ref`, `IXSCAN`. It is never
	// translated, because a reader who knows their engine knows these words and a reader who does
	// not would search for them.
	Type string
	// Detail is what the step works on, in one line: the relation, the index, the condition.
	Detail   string
	Depth    int
	Rows     float64
	Cost     float64
	SelfCost float64
}

// PlanFact is one thing the engine said about the plan as a whole. A list of pairs rather than a
// struct of fields because what a plan reports is the engine's own: PostgreSQL prices a plan,
// MySQL prices a query block, and a document store prices nothing at all — a field for a fact one
// engine has not got would only be filled with a zero that reads as an answer.
type PlanFact struct {
	Label string
	Value string
}

// Plan is a plan in the forms it is read in, and the first of them is the engine's own: Columns and
// Rows are exactly what an `EXPLAIN` typed by hand answers with — MySQL's grid of access types,
// PostgreSQL's plan text, MongoDB's one document. Everything else here is read out of that same
// answer, and a reader who knows their engine wants the answer itself.
type Plan struct {
	Columns []Column
	Rows    [][]*string
	Nodes   []PlanNode
	Summary []PlanFact
	JSON    string
	// Costed says the engine puts numbers on a step at all — a cost, and the rows it expects the step
	// to produce. A document store does neither, so its plan is drawn without those columns rather
	// than with columns full of zeroes.
	Costed bool
}

type CellEdit struct {
	Namespace
	Table  string
	Column string
	Value  *string
	Key    map[string]string
	// ValueType is what the value is to be written **as**, on an engine where a value carries a type
	// the reader picks rather than a column declaring one. It travels with the value and never on its
	// own, because on such an engine the two cannot be written apart: a Redis key's type *is* what its
	// value is, so `hash` and `{"a":"1"}` are one fact and two writes of it would each be refused by
	// the half the other was going to supply. Empty means "whatever the value is already", which is
	// every edit on every other engine, and a driver with no such notion ignores it.
	ValueType string
}

// RowWriteMode is what a row arriving from outside does to the table it is written into. The list
// is closed and a mode this side does not know is refused rather than guessed at, exactly as a
// filter's operator is.
type RowWriteMode string

const (
	// WriteAppend adds every row. A row the table already holds is a failure the *server* reports,
	// which is the honest outcome: only it knows what a key collision is.
	WriteAppend RowWriteMode = "append"
	// WriteUpdate changes the rows the key names and adds none.
	WriteUpdate RowWriteMode = "update"
	// WriteUpsert changes the rows the key names and adds the ones it does not.
	WriteUpsert RowWriteMode = "upsert"
	// WriteSkip adds a row the key does not already name and leaves the ones it does alone.
	WriteSkip RowWriteMode = "skip"
	// WriteDelete removes the rows the key names, and reads no other column at all.
	WriteDelete RowWriteMode = "delete"
)

// RowWrite is a batch of rows and what they are to do to a table.
//
// Keys names the columns that identify a stored row, and every mode but Append needs them: without
// one there is nothing to change, skip or remove. Batch is how many rows one statement may carry,
// for the modes whose statement can carry more than one — it is the caller's number because only
// the caller knows what the server will accept, which on MySQL is `max_allowed_packet`.
type RowWrite struct {
	Mode    RowWriteMode
	Columns []string
	Keys    []string
	Rows    [][]*string
	Batch   int
}

// RowInsert carries only the columns a new row actually filled in, so a column left alone
// takes the server's own default instead of an explicit NULL.
type RowInsert struct {
	Namespace
	Table  string
	Values map[string]*string
}

// RowDelete names one stored row the way CellEdit does, by its primary key.
type RowDelete struct {
	Namespace
	Table string
	Key   map[string]string
}

// ReviewedStatement is one statement of a script as the reviewer reads it, without running any
// of it: where it sits in the text the reader is looking at, and what it is going to do. Start
// and End are byte offsets into the script, so a warning can point back at the statement that
// earned it rather than repeating it.
type ReviewedStatement struct {
	Start   int
	End     int
	SQL     string
	Keyword string
	Hazard  domain.Hazard
}

// Problem is something wrong with a script that the lexer can *prove* without a server: a quote
// or a comment the text ends inside of, a parenthesis with no partner. It is deliberately not a
// parser — anything needing to know whether a name exists or a clause is in the right order is
// the server's answer to give, and guessing at it would cost the reader more than it saved.
type Problem struct {
	Start   int
	End     int
	Message string
}

// Review is what a script says about itself before anything runs it.
type Review struct {
	Statements []ReviewedStatement
	Problems   []Problem
}

// Text is the half of an engine that is purely lexical: where a statement ends, how a script is
// laid out, and what each of its statements will do. It is the driver's for the reason splitting
// always was — a semicolon inside a `$body$` ends nothing on PostgreSQL and a backslash escapes a
// quote only on MySQL — and it is its own interface so `sqlkit.Dialect` can supply the whole of it.
type Text interface {
	// Split cuts a script into the statements it holds.
	Split(script string) []string
	// Format lays a script out: one clause to a line, keywords in upper case, and the statement
	// it was handed back unchanged in every respect a server would notice.
	Format(script string) string
	// Review reads what a script is going to do, and what is already wrong with it.
	Review(script string) Review
}

type Driver interface {
	Engine() domain.Engine
	// Open dials the server. `dial` is nil for a direct connection and a tunnel's own dial for
	// one carried through a bastion; an adapter that cannot honour it must say so rather than
	// quietly connecting around it.
	Open(
		ctx context.Context,
		profile domain.ConnectionProfile,
		creds Credentials,
		dial Dialer,
	) (Session, error)
	// Text needs no connection, so it answers for the engine rather than for a session: a reader
	// lays out and is warned about a script whether or not anything is dialled.
	Text() Text
}

// Schema is the DDL half of a session: reading a table's shape and changing it. It is its own
// interface so an adapter can keep its schema work in its own file, and so `sqlkit.DDL` can
// supply the statements the two engines spell identically.
type Schema interface {
	TableStructure(ctx context.Context, ns Namespace, table string) (TableStructure, error)
	// CreateDatabaseSQL is exactly what CreateDatabase will run, in order, and is also what the
	// dialog previews: one method spells the statement, so what is shown and what runs cannot
	// drift apart.
	CreateDatabaseSQL(spec DatabaseSpec) []DatabaseStatement
	CreateDatabase(ctx context.Context, spec DatabaseSpec) error
	DropDatabase(ctx context.Context, name string) error
	// DatabaseOptions reads the lists a new database is declared from — roles, templates,
	// encodings. It sits with the DDL rather than with the catalogue reads because it exists
	// for CreateDatabase alone.
	DatabaseOptions(ctx context.Context) (DatabaseOptions, error)
	CreateTable(ctx context.Context, spec TableSpec) error
	RenameTable(ctx context.Context, ns Namespace, table, name string) error
	// DuplicateTable makes a second table shaped like the first, holding its rows only when
	// asked. What is copied is the table's own — its columns, their defaults and its indexes —
	// and never a foreign key: a constraint names two tables and neither engine copies one.
	DuplicateTable(ctx context.Context, ns Namespace, table, name string, withRows bool) error
	DropTable(ctx context.Context, ns Namespace, table string) error
	// TruncateTable drops every row at once; EmptyTable deletes them, which is the only one
	// of the two a table under a foreign key will accept.
	TruncateTable(ctx context.Context, ns Namespace, table string) error
	EmptyTable(ctx context.Context, ns Namespace, table string) error
	AddColumn(ctx context.Context, ns Namespace, table string, spec ColumnSpec) error
	AlterColumn(ctx context.Context, ns Namespace, table, column string, spec ColumnSpec) error
	RenameColumn(ctx context.Context, ns Namespace, table, column, name string) error
	DropColumn(ctx context.Context, ns Namespace, table, column string) error
	// SetPrimaryKey replaces whatever key the table has, and drops it outright for an empty
	// list. What the old key was called is the adapter's to find out, since only PostgreSQL
	// needs the name to drop it.
	SetPrimaryKey(ctx context.Context, ns Namespace, table string, columns []string) error
	// CreateIndex is the one statement of the four an engine spells its own way; the drop and
	// both halves of a foreign key are alike enough to be shared.
	CreateIndex(ctx context.Context, ns Namespace, table string, spec IndexSpec) error
	DropIndex(ctx context.Context, ns Namespace, table, index string) error
	CreateForeignKey(ctx context.Context, ns Namespace, table string, spec ForeignKeySpec) error
	DropForeignKey(ctx context.Context, ns Namespace, table, constraint string) error
	// AlterStatements spells changes to a table's shape as the statements that would make them, in
	// the order they are given, and **runs none of them**. It is what a migration shows the reader
	// before anything happens, and it is the driver's for the reason every other statement is:
	// `MODIFY COLUMN` against one `ALTER COLUMN` per fact, an index that is an object of the schema
	// against one that belongs to its table, and a document store's `$set` are one intention spelled
	// four ways.
	//
	// It is spelling and not running, so it needs no connection and answers no error: a change this
	// engine has no concept of contributes **nothing** rather than failing, and the run of that same
	// change is where `domain.Unsupported` is raised. A migration that shows an empty script is a
	// migration with nothing to run.
	//
	// Every kind here has a run beside it on this same interface, and both go through one speller,
	// so what the reader was shown and what happens cannot drift apart.
	AlterStatements(ns Namespace, table string, changes []TableChange) []string
	// RowStatements spells rows as the statements that write them into a table the way the caller
	// asked: appended, merged on a key, or removed by one. It is the driver's for the reason every
	// other statement is — only it knows how this engine quotes a name and a string, and MySQL's
	// `ON DUPLICATE KEY`, PostgreSQL's `ON CONFLICT` and a document store's upsert are three
	// statements for one intention.
	//
	// One method rather than two, because a SQL export, the data half of a dump and an import are
	// the same question asked with different modes: an export is `WriteAppend` with a batch of one.
	RowStatements(ns Namespace, table string, write RowWrite) []string
	// CreateStatements spells an object as the statements that would make it again: the table and
	// its key, then its indexes, then its constraints. It is the structure half of a dump and
	// RowStatements is the data half, and it is the driver's for the same reason — only the
	// engine knows how it declares one.
	//
	// It is a *read* rather than a render because an engine that can answer for itself is asked
	// rather than described: a declaration put back together out of a catalogue is only ever as
	// complete as the catalogue's columns.
	CreateStatements(ctx context.Context, ns Namespace, table string) ([]string, error)
	// DropSource removes one view, routine or trigger. It takes the whole reference rather than a
	// name because on three of the four kinds the name alone does not identify one: two PostgreSQL
	// functions may share it and differ in arguments, and a trigger is dropped about its table.
	//
	// One method for all four because they differ in exactly one word, which the kind supplies —
	// the same reason `Dialect.DropSource` is one speller.
	DropSource(ctx context.Context, ns Namespace, ref SourceRef) error
	// Source reads one view or one routine as the statement that declares it. It is the read a
	// designer opens on, and it is the driver's for the reason every other declaration is: only
	// the engine knows where it keeps one, and four of the five will answer for themselves.
	//
	// An engine that has no such object says `domain.Unsupported` naming which, rather than
	// answering with an empty definition a save would then wipe the object out with.
	Source(ctx context.Context, ns Namespace, ref SourceRef) (Source, error)
	// SourceStatements spells saving a definition as the statements that would make it, and
	// **runs none of them** — the arrangement AlterStatements has, for the same reason: the
	// preview is what protects a reader from a save that drops an object before it makes it.
	//
	// The definition itself travels **verbatim**. It is one statement however many semicolons are
	// inside it, so a routine body needs no DELIMITER and nothing here re-lexes what the reader
	// typed; where it is not the whole statement is where the adapter's own read already said so.
	SourceStatements(ns Namespace, save SourceSave) []string
	// SaveSource runs exactly what SourceStatements showed, in that order, stopping at the first
	// failure. There is no transaction around it, which is why the preview has to be exact.
	SaveSource(ctx context.Context, ns Namespace, save SourceSave) error
	// NewSource is the declaration a reader **starts from** when they are making one of these
	// rather than altering it: the shape of the statement with nothing in it yet. It is the
	// driver's for the reason every other statement is — a routine's body is grammar, and MySQL's
	// `BEGIN … END`, PostgreSQL's language-tagged string and Oracle's `IS` are one intention
	// spelled three ways.
	//
	// It takes the whole reference rather than a name because a trigger's skeleton addresses two
	// things: the trigger, and the table it fires on.
	//
	// It runs nothing and needs no connection. An engine with no such object answers with the
	// empty string, which is what the frontend reads as there being nothing to start.
	NewSource(ns Namespace, ref SourceRef) string
}

// Transaction is the manual half of a session's writing: the reader decides when what they have
// done becomes true, instead of every statement being true the moment it runs.
//
// It is a fact of the **connection** and therefore of the whole session, which is the only honest
// way to model it: uncommitted rows exist on one connection and nowhere else, so a grid that read
// them off a second one would show the reader a table that does not yet exist. While one is open
// every statement of the session runs inside it — its reads, its writes and its schema changes
// alike — and `sqlkit.Txn` is what supplies the whole of this for the five SQL engines.
//
// An engine with no such notion answers `domain.Unsupported` naming it, and the frontend never
// offers the button — see `EngineInfo.hasTransactions`.
type Transaction interface {
	// Begin opens one, in the namespace the reader is looking at, and refuses where one is already
	// open: `domain.ErrTransactionActive`.
	Begin(ctx context.Context, ns Namespace) error
	// Commit and Rollback both end it and both refuse where none is open
	// (`domain.ErrNoTransaction`). Either way the connection goes back to the pool, so the session
	// is autocommitting again the moment one of them returns — including when it returns a failure,
	// a transaction the server has thrown out not being one this side may keep holding.
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	// InTransaction is what the button draws itself from. It asks no server anything: whether this
	// session is holding a transaction open is this side's own fact.
	InTransaction() bool
}

type Session interface {
	Schema
	Transaction

	Ping(ctx context.Context) error
	ServerInfo(ctx context.Context) (ServerInfo, error)
	Databases(ctx context.Context) ([]string, error)
	// Schemas lists the level below a database, and returns nothing for an engine that has
	// none. It is read lazily, one database at a time, because PostgreSQL can only see the
	// schemas of a database something is connected to.
	Schemas(ctx context.Context, database string) ([]string, error)
	Objects(ctx context.Context, ns Namespace) (Objects, error)
	// TableList is the same tables Objects names, with what the catalogue keeps about each of
	// them. It is a read of its own rather than part of Objects because the tree needs the names
	// the moment a folder opens, and the numbers behind them are worth a second query.
	TableList(ctx context.Context, ns Namespace) (TableList, error)
	TableSchemas(ctx context.Context, ns Namespace) ([]TableSchema, error)
	TableRows(ctx context.Context, page TablePage) (TableRows, error)
	// ColumnValues lists the values one column actually holds, for a reader picking a filter's
	// value rather than typing it. Deduplicated, ordered and capped, and without NULL — that is
	// what the IS NULL operator is for.
	ColumnValues(ctx context.Context, pick ColumnPick) ([]string, error)
	// MatchingRows counts the rows of one table that hold the reader's text in any column, and spells
	// the read that would return them. It is the driver's for the reason every other read is: which
	// rows hold a piece of text is a *statement*, and every column of any type is read as text to
	// answer it — a search for `36` finds the number as readily as the name.
	MatchingRows(ctx context.Context, search RowSearch) (RowMatches, error)
	UpdateCell(ctx context.Context, edit CellEdit) error
	InsertRow(ctx context.Context, insert RowInsert) error
	DeleteRow(ctx context.Context, removal RowDelete) error
	Query(ctx context.Context, query Query) (QueryResult, error)
	Close() error
}

type Registry struct {
	drivers map[domain.Engine]Driver
}

func NewRegistry(drivers ...Driver) *Registry {
	r := &Registry{drivers: make(map[domain.Engine]Driver, len(drivers))}
	for _, d := range drivers {
		r.drivers[d.Engine()] = d
	}
	return r
}

func (r *Registry) For(engine domain.Engine) (Driver, error) {
	d, ok := r.drivers[engine]
	if !ok {
		return nil, domain.ValidationError{Field: "engine", Message: "is not supported"}
	}
	return d, nil
}
