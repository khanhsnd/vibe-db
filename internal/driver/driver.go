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
type Routine struct {
	Name      string
	Procedure bool
	Arguments string
}

// Objects is everything a namespace holds that the tree draws a folder for. They are read
// together for the reason a table's shape is: expanding any one folder needs every folder's
// count, so one read answers all three.
type Objects struct {
	Tables   []string
	Views    []string
	Routines []Routine
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
	// InsertStatements spells rows as the statements that would put them back, which is what a
	// SQL export is. It is the driver's for the reason every other statement is: only it knows
	// how this engine quotes a name and a string.
	InsertStatements(ns Namespace, table string, columns []string, rows [][]*string) []string
	DropView(ctx context.Context, ns Namespace, view string) error
	// DropRoutine takes the whole routine rather than its name, because on PostgreSQL the
	// name alone does not identify one: two functions may share it and differ in arguments.
	DropRoutine(ctx context.Context, ns Namespace, routine Routine) error
}

type Session interface {
	Schema

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
