// Package sqlkit holds the parts of a database/sql adapter that do not vary by engine:
// row scanning, result-column reflection, statement classification, and reading the table
// a result can be written back to. Engines differ only in the lexical details a Dialect
// carries, which leaves an adapter its DSN, its error map and its catalogue queries.
package sqlkit

import (
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Dialect is the lexical difference between engines: the character that quotes an
// identifier, whether `#` starts a line comment, and which part of a Namespace qualifies a
// table name.
//
// **Lex is not only SQL's.** Where a statement ends, what a string is and what a comment is are
// the same questions in a document store's shell, so the MongoDB adapter carries a Dialect of its
// own for the lexer alone — see `Shell` — and nothing else in this file has anything to say to it.
type Dialect struct {
	IdentQuote byte
	// IdentClose is the character that *ends* a quoted name where it is not the one that opened
	// it — SQL Server's `[name]`. Zero means the two are the same, which is every other engine.
	IdentClose   byte
	HashComments bool
	// SlashComments says `//` starts a line comment, which is what a shell script uses and
	// neither SQL engine has.
	SlashComments bool
	// NamespaceIsSchema says a table is qualified by the schema inside its database rather
	// than by the database itself, which is the one structural difference between the two
	// engines' names for the same table.
	NamespaceIsSchema bool
	// NamespaceIsBoth says a name carries the database *and* the schema — SQL Server's
	// `[shop].[dbo].[users]`. It is what lets one pool read every database on the server, where
	// PostgreSQL, whose names stop at the schema, needs a pool per database to reach one.
	NamespaceIsBoth bool
	// ParamPrefix is how this engine numbers a bound argument: empty for `?`, and otherwise the
	// text in front of the position — `$` for `$1`, `@p` for `@p1`, `:` for `:1`.
	ParamPrefix string
	// FetchPaging says a page is cut with `OFFSET n ROWS FETCH NEXT m ROWS ONLY` rather than with
	// `LIMIT m OFFSET n`, which is the standard spelling and the only one SQL Server and Oracle have.
	FetchPaging bool
	// PagingNeedsOrder says that spelling is refused without an ORDER BY, which SQL Server alone
	// insists on — so a read with nothing to order by is given an order that orders nothing.
	PagingNeedsOrder bool
	// LimitsWrite says an UPDATE or a DELETE may be bounded by a row limit, which is MySQL's alone
	// and is belt and braces: the primary key in the WHERE already names at most one row.
	LimitsWrite bool
	// DualTable is the table a SELECT with no table of its own reads from — Oracle's `dual`. Empty
	// where a bare SELECT is a statement in its own right, which is everywhere else.
	DualTable string
	// MultiRowValues says one INSERT may carry several tuples after its VALUES. Oracle cannot, so
	// a batch there is one statement per row rather than a longer statement.
	MultiRowValues bool
	// EmptyInsert is how a new row that filled nothing in is written: MySQL's empty column list,
	// or the `DEFAULT VALUES` the standard gives. Empty means the engine has neither and the row
	// names every column with the keyword instead — see everyDefault.
	EmptyInsert string
	// BackslashEscapes says the engine reads `\` inside a string literal as an escape, which
	// MySQL does unless told otherwise and PostgreSQL does not.
	BackslashEscapes bool
	// DollarQuotes says `$tag$ … $tag$` is a string, which is how a PostgreSQL function body
	// carries semicolons without ending the statement that declares it.
	DollarQuotes bool
	// LikeInColumnList says a table copied from another declares the copy inside its column
	// list — `CREATE TABLE new (LIKE old INCLUDING ALL)` — rather than as a clause after the
	// name, and has to be asked for the defaults and indexes the other engine's LIKE takes
	// unbidden. See CopyTable.
	LikeInColumnList bool
	// CopyAsSelect says a table is copied by *reading* the other one — `CREATE TABLE new AS
	// SELECT * FROM old` — because this engine has no LIKE at all. What that copies is the columns
	// and their types and nothing else: no key, no index, no default.
	CopyAsSelect bool
	// SelectIntoCopy is the same idea spelled the other way round, the new table's name sitting in
	// the middle of the read — SQL Server's `SELECT * INTO new FROM old`.
	SelectIntoCopy bool
	// OverridesIdentity says a write has to declare it is supplying a value for an identity
	// column, which PostgreSQL refuses the row without and MySQL has no clause for. See CopyRows.
	OverridesIdentity bool
	// MergesOnConflict says an insert that meets a row already there says so in a clause *after*
	// the values, naming the columns it conflicts on — PostgreSQL's `ON CONFLICT`. MySQL says it
	// in the verb (`INSERT IGNORE`) and in a clause that names no columns at all, leaving the
	// server to find whichever key was broken. See InsertVerb and MergeClause.
	MergesOnConflict bool
	// MergeByExists says this engine has neither of those clauses, so a row that may already be
	// there is written one row at a time: an insert guarded by `WHERE NOT EXISTS`, and an update
	// in front of it for an upsert. SQL Server and Oracle both have a MERGE statement instead, and
	// it is not used — a MERGE is a page of syntax to say what two ordinary statements already say.
	MergeByExists bool
	// KeepsColumnAsDeclared says a column cannot be changed at all once it is made, because this
	// engine keeps its schema as the *text* of the statements that made it and will not re-parse
	// that text on request — SQLite. A change to one therefore spells nothing, which is what an
	// empty list means everywhere in AlterStatements, and the run of it answers Unsupported.
	KeepsColumnAsDeclared bool
	// RestatesColumn says an altered column is restated *whole* — MySQL's `MODIFY COLUMN`, where a
	// fact left out is a fact dropped. PostgreSQL has no such clause and takes one `ALTER COLUMN`
	// action per fact instead. See AlterColumn.
	RestatesColumn bool
	// RestateClause is the words in front of that restated column, and differs where the shape does
	// not: `MODIFY COLUMN` on MySQL, `MODIFY` on Oracle. Empty takes MySQL's.
	RestateClause string
	// SeparatesDefault says a default is a *constraint* with a name of its own rather than a fact
	// of the column — SQL Server — so an altered column carries its type and its nullability alone
	// and the default is a second statement. See AlterColumn.
	SeparatesDefault bool
	// AltersColumnOnly says the restated column carries no default clause at all. It goes with
	// SeparatesDefault and exists because the two answer different questions: one is where the
	// default goes, the other is what the ALTER itself is allowed to say.
	AltersColumnOnly bool
	// MethodBeforeTable says an index's access method sits between its name and the table —
	// MySQL's `CREATE INDEX n USING m ON t (…)`. PostgreSQL takes it after the table.
	MethodBeforeTable bool
	// IndexIsObject says an index is an object of the schema in its own right and is dropped by
	// name — PostgreSQL's `DROP INDEX`. On MySQL an index belongs to its table and is dropped
	// through it.
	IndexIsObject bool
	// QualifiesIndexName says an index is *created* under a namespace of its own — Oracle, where an
	// index is an object beside the table rather than a part of it. PostgreSQL, whose index is also
	// an object, refuses a qualified name on the create and takes one on the drop, which is why
	// this is not simply IndexIsObject.
	QualifiesIndexName bool
	// ObjectTableIsBare says the table an index or a trigger is declared *on* carries no namespace,
	// because the object's own name already did — SQLite, whose grammar puts the schema on the index
	// and on the trigger and refuses one on the table. It goes with QualifiesIndexName rather than
	// replacing it: Oracle qualifies both.
	ObjectTableIsBare bool
	// DropsIndexOnTable says an index is dropped by naming it *and* its table — SQL Server's
	// `DROP INDEX i ON t`, which is neither of the other two spellings.
	DropsIndexOnTable bool
	// ForeignKeyIsConstraint says a referential constraint is dropped as a constraint —
	// PostgreSQL's `DROP CONSTRAINT`. MySQL has a clause of its own, `DROP FOREIGN KEY`.
	ForeignKeyIsConstraint bool
	// TextType is what this engine calls text in a CAST, which is how a column of any type is
	// compared as text — see Where's `contains`.
	TextType string
	// RegexIsFunction says a regular-expression match is a function whose third argument is the
	// case sensitivity — MySQL's `REGEXP_LIKE`, which Oracle and SQL Server spell the same way.
	// PostgreSQL has an operator per answer, `~` and `~*`.
	RegexIsFunction bool
	// RegexFlagsInPattern says the match takes no flags and has no second operator, so the case box
	// is written into the pattern itself as `(?i)`. That is what a `REGEXP` implemented *here*
	// rather than by the server looks like — see the SQLite adapter, which registers one.
	RegexFlagsInPattern bool
	// MatchesByRegex says every search mode is spelled as an expression rather than only the two a
	// pattern cannot say. It is for an engine whose own LIKE cannot answer the case box — SQLite's
	// ignores case for ASCII and for nothing else — and it is worth having because the expression
	// is then the same rule a *name* is matched by, on the same engine, in one place.
	MatchesByRegex bool
	// SensitiveCast is the type a pattern is cast to for a case-*sensitive* LIKE on an engine whose
	// own collation ignores case — MySQL's `BINARY`. Empty where LIKE is already sensitive.
	SensitiveCast string
	// SensitiveCollation is the same answer for an engine that has no such cast but does have a
	// collation to compare under — SQL Server's `Latin1_General_BIN2`.
	SensitiveCollation string
	// WordBoundary is this engine's regular-expression token for the edge of a word, and is the one
	// piece of that syntax the engines spell differently: `\b` on MySQL's ICU and in Go, and `\y` on
	// PostgreSQL, where `\b` is a backspace.
	WordBoundary string
	// TriggerOnTable says a trigger is dropped by naming its table as well as itself — PostgreSQL's
	// `DROP TRIGGER t ON tbl`, a trigger name there being unique per table rather than per schema.
	// Everywhere else a trigger is an object of the schema and its own qualified name finds it.
	TriggerOnTable bool
	// Starters is what a reader begins a *new* view, routine or trigger from — see SourceStarter.
	Starters SourceStarter
	// RefreshViewSQL recomputes the rows a materialized view stores, `%s` being the view. Empty is
	// an engine with no such view, which answers `domain.Unsupported` rather than running nothing.
	RefreshViewSQL string
	// RefreshNamesString says the view goes in as a **string literal** rather than as an identifier,
	// because on that engine a refresh is a procedure call rather than a statement — Oracle's
	// `DBMS_MVIEW.REFRESH('SHOP.SALES')`. It is the same kind of exception AccountHasHost is: one
	// name in this package that is not quoted as a name.
	RefreshNamesString bool
	// ColumnCommentInline says what somebody wrote about a column is a *clause of the column* —
	// MySQL's `COMMENT 'x'` — so it travels in the same statement that declares it, and a restated
	// column that leaves it out is a description dropped.
	ColumnCommentInline bool
	// CommentsOnObject says the description is a statement of its own, `COMMENT ON COLUMN t.c IS 'x'`
	// — PostgreSQL and Oracle, whose grammar is the standard's word for word. An engine setting
	// neither has nowhere to keep one: SQL Server's is an extended property written by a stored
	// procedure rather than a comment, and SQLite has none at all.
	CommentsOnObject bool
	// Partitions is how one piece of a partitioned table is named and changed — see PartitionStyle.
	Partitions PartitionStyle
	// Calls is how a routine is run and how a value it hands back is received — see CallStyle.
	Calls CallStyle

	// The account half. Everything below is one engine's grammar for "who may do what", and it is
	// here for the reason everything above is: `sqlkit.Auth` spells every account statement there is,
	// and what differs between five engines is a flag rather than a second speller.

	// AccountHasHost says an account **is** the pair `'name'@'host'` — MySQL's alone. Both halves are
	// then string literals rather than identifiers, which is the one name in this package that is not
	// quoted as a name.
	AccountHasHost bool
	// AccountKeyword is the word this engine declares one with. Empty takes `USER`; PostgreSQL says
	// `ROLE`, a role and a user being the same object there and telling apart only by whether it may
	// log in.
	AccountKeyword string
	// PasswordClause is where a password goes, with `%s` for the literal: `IDENTIFIED BY %s` on MySQL
	// and Oracle, `PASSWORD %s` inside PostgreSQL's `WITH`, `PASSWORD = %s` inside SQL Server's.
	// Empty is an engine with no password to set.
	PasswordClause string
	// PasswordIsIdentifier says the literal is quoted as a **name** rather than as a string, which is
	// Oracle's grammar and nobody else's.
	PasswordIsIdentifier bool
	// AttributesInWith says the clauses that follow the name sit after a `WITH` — PostgreSQL and SQL
	// Server — rather than being written one after another, which is MySQL's and Oracle's shape.
	AttributesInWith bool
	// LoginIsSeparate says a **login** belongs to the server and a **user** to a database, so
	// declaring an account is two statements and dropping one is two the other way round. SQL
	// Server's alone, and the reason `driver.AccountRef` carries a database.
	LoginIsSeparate bool
	// The three shapes of a grant's `ON` clause; an engine setting none of them names the object and
	// nothing else, which is Oracle. See GrantTarget for what each spells.
	GrantsByWildcard bool
	GrantsOnTable    bool
	GrantsByClass    bool
	// ColumnsAfterObject says the columns of a column-level grant sit after the object rather than
	// after the privilege — `GRANT SELECT ON OBJECT::t (c)` against `GRANT SELECT (c) ON t`. It is
	// the one difference in that statement that is about placement rather than about words.
	ColumnsAfterObject bool
	// HasDeny says this engine has a third verb — SQL Server's `DENY`, which is not the absence of a
	// grant: it outranks one arriving through a role. An engine without it refuses the change naming
	// it rather than quietly revoking instead.
	HasDeny bool
	// RoleByMembership says a role is granted by altering the **role** — `ALTER ROLE r ADD MEMBER u` —
	// rather than by granting it to the account. SQL Server's alone.
	RoleByMembership bool
	// RoleAdminOption says a granted role may carry the right to grant it on, spelled `WITH ADMIN
	// OPTION`.
	RoleAdminOption bool
	// SystemAdminOption says a privilege over the *server* is administered rather than granted, so
	// the option after it is `WITH ADMIN OPTION` — Oracle, the one engine whose grammar draws the
	// distinction.
	SystemAdminOption bool
	// RenameAccountSQL is the whole statement, `%s` being the account and `%t` the name it is taking.
	// Empty is an engine that cannot rename one at all — Oracle, where a user is a schema and its
	// name is on every object inside it.
	RenameAccountSQL string
	// DropAccountSuffix is what follows the name on a drop: Oracle's ` CASCADE`, without which the
	// statement is refused for any account that owns anything.
	DropAccountSuffix string
	// AccountAttrs is what an account carries besides a password, **in the order they are written**:
	// MySQL's grammar wants a resource option before a lock, and a clause out of place is refused by
	// the server rather than reordered here. See AccountAttr.
	AccountAttrs []AccountAttr

	// The monitoring half. Two statements and one flag, and `sqlkit.Watch` spells every kill there
	// is out of them — the arrangement the account half above has.

	// CancelStatementSQL stops the statement a process is running and leaves the connection open,
	// `%s` being the process. Empty is an engine that cannot do the one without the other: SQL Server
	// has only `KILL`, and Oracle's `ALTER SYSTEM CANCEL SQL` arrived in 18c and takes the statement's
	// own id — neither is a thing to guess at, so both say so instead. See Watch.
	CancelStatementSQL string
	// CloseConnectionSQL ends the connection itself. Empty is an engine whose client is disconnected
	// by the client and not by the server, which of the seven is only the document store — and that
	// one has no Dialect here at all.
	CloseConnectionSQL string
	// ProcessIsPair says a process **is** two numbers in one string literal — Oracle's `'15,3421'`,
	// a sid being reused the moment a session ends. It is the one name in this half that is not a
	// bare number, exactly as AccountHasHost is the one account name that is not an identifier.
	ProcessIsPair bool
}

// SourceStarter is the skeleton a reader starts a new view, routine or trigger from: the shape of
// the declaration with nothing in it yet. It is a **flag** in the sense every other difference here
// is — four strings per engine — and it is the dialect's rather than the frontend's for the reason
// nothing outside a driver ever spells a statement.
//
// The four differ in *grammar* rather than in quoting, which is why one speller could not do it:
// MySQL wraps a routine's body in `BEGIN … END`, PostgreSQL puts it in a string tagged with the
// language it is written in, SQL Server opens with `AS`, and Oracle says `IS`. A trigger's shape
// diverges further still — PostgreSQL's fires a function it cannot declare inline, and SQL Server's
// has no `FOR EACH ROW` at all. No reader should have to remember which, and an engine with no
// routines leaves those blank.
//
// Each holds `%s`, the qualified name every other statement here is addressed by, and a trigger's
// also holds `%t`, the qualified table it fires on — a trigger being the one of these declared
// *about* something else. Nothing runs one of these: what a Save runs is the reader's own buffer,
// which SourceStatements previews whole.
type SourceStarter struct {
	View      string
	Function  string
	Procedure string
	Trigger   string
	// The three an engine may not have at all, left blank where it has not — which is what the
	// frontend reads as there being no New to offer.
	MaterializedView string
	Sequence         string
	Event            string
}

// PartitionStyle is how an engine names one piece of a partitioned table and changes it. There are
// three grammars for one intention, and telling them apart is a *flag* in the sense every other
// difference in this file is — `sqlkit.DDL` spells all three and no adapter spells any of them.
//
//   - PartitionAsTable: a partition **is a table**, so it is dropped and truncated as one and added
//     by declaring it — PostgreSQL. The only style where the piece has a name of its own in the
//     catalogue that a `SELECT` could also use.
//   - PartitionInList: an added partition sits inside a parenthesised list after `ADD PARTITION`,
//     which is MySQL's grammar and nobody else's.
//   - PartitionAsClause: the partition is named straight after the keyword — Oracle.
//
// An engine that leaves it empty has no partition this app can address: SQL Server's are numbered
// pieces of a partition *scheme* rather than named objects, and SQLite has none.
type PartitionStyle string

const (
	PartitionNone     PartitionStyle = ""
	PartitionAsTable  PartitionStyle = "table"
	PartitionInList   PartitionStyle = "list"
	PartitionAsClause PartitionStyle = "clause"
)

// NewSource is the starter for this kind, addressed to the object it declares. An engine that has
// no such object answers with nothing, which is what the frontend reads as "there is no starter" —
// it does not offer a New for a kind this engine has not got in the first place.
//
// Both placeholders are filled in **one pass**, so a name holding the other's marker is a name and
// not a second substitution.
func (d Dialect) NewSource(ns driver.Namespace, ref driver.SourceRef) string {
	starter := d.Starters.View
	switch ref.Kind {
	case driver.SourceFunction:
		starter = d.Starters.Function
	case driver.SourceProcedure:
		starter = d.Starters.Procedure
	case driver.SourceTrigger:
		starter = d.Starters.Trigger
	case driver.SourceMaterializedView:
		starter = d.Starters.MaterializedView
	case driver.SourceSequence:
		starter = d.Starters.Sequence
	case driver.SourceEvent:
		starter = d.Starters.Event
	}

	if starter == "" {
		return ""
	}

	// A trigger is addressed by the two names its own statements take, which on two engines are not
	// the ones every other statement here writes — see TriggerName and ObjectTable.
	name := d.Table(ns, ref.Name)
	if ref.Kind == driver.SourceTrigger {
		name = d.TriggerName(ns, ref.Name)
	}

	return strings.NewReplacer(
		"%s", name,
		"%t", d.TriggerTable(ns, ref.Table),
	).Replace(starter)
}

// PatternIgnoresCase says this engine's own collation already makes LIKE blind to case, so a match
// the reader asked to be case-*sensitive* is the one that has to say so — see MatchAny. It is read
// off the two ways an engine has of saying so rather than declared beside them, which is what stops
// a dialect claiming one and supplying neither.
func (d Dialect) PatternIgnoresCase() bool {
	return d.SensitiveCast != "" || d.SensitiveCollation != ""
}

var (
	MySQL = Dialect{
		IdentQuote:        '`',
		HashComments:      true,
		BackslashEscapes:  true,
		MultiRowValues:    true,
		EmptyInsert:       " () VALUES ()",
		LimitsWrite:       true,
		RestatesColumn:    true,
		MethodBeforeTable: true,
		TextType:          "CHAR",
		RegexIsFunction:   true,
		SensitiveCast:     "BINARY",
		WordBoundary:      `\b`,
		Starters: SourceStarter{
			View: "CREATE VIEW %s AS\nSELECT 1",
			Function: "CREATE FUNCTION %s()\n" +
				"RETURNS INT\nDETERMINISTIC\nBEGIN\n  RETURN 0;\nEND",
			Procedure: "CREATE PROCEDURE %s()\nBEGIN\n  SELECT 1;\nEND",
			// A trigger body may not return rows, so the placeholder statement is an assignment
			// rather than the `SELECT 1` a procedure starts from.
			Trigger: "CREATE TRIGGER %s\nBEFORE INSERT ON %t\nFOR EACH ROW\n" +
				"BEGIN\n  SET @unused = 1;\nEND",
			// The one engine with an event. There is no materialized view and no sequence here: a
			// number generator on MySQL is a column's `AUTO_INCREMENT` rather than an object.
			Event: "CREATE EVENT %s\nON SCHEDULE EVERY 1 DAY\nDO\nBEGIN\n  SET @unused = 1;\nEND",
		},
		ColumnCommentInline: true,
		Partitions:          PartitionInList,
		Calls:               CallWithVariables,
		AccountHasHost:      true,
		PasswordClause:      "IDENTIFIED BY %s",
		GrantsByWildcard:    true,
		RoleAdminOption:     true,
		RenameAccountSQL:    "RENAME USER %s TO %t",
		AccountAttrs: []AccountAttr{
			// A resource option comes before a lock in MySQL's grammar, which is why the order
			// these are declared in is the order they are written in.
			{
				Name: "maxConnections", Label: "Max connections", Kind: driver.AttributeNumber,
				On:   "WITH MAX_USER_CONNECTIONS %s",
				Help: "0 leaves the server's own limit in force.",
			},
			{
				Name: "locked", Label: "Account locked", Kind: driver.AttributeCheck,
				On: "ACCOUNT LOCK", Off: "ACCOUNT UNLOCK",
			},
			// There is no clause that un-expires a password, so this one has no Off: what clears it
			// is setting a new password. `PASSWORD EXPIRE NEVER` is not the opposite of expired — it
			// is a *policy* saying it never will be — and writing it here would change a rule the
			// reader never asked about.
			{
				Name: "expired", Label: "Password expired", Kind: driver.AttributeCheck,
				On:   "PASSWORD EXPIRE",
				Help: "Setting a new password is what clears this.",
			},
		},
		// The one engine that says both halves in one verb, and the only reason `QUERY` and
		// `CONNECTION` are spelled out is that `KILL 5` alone means the connection.
		CancelStatementSQL: "KILL QUERY %s",
		CloseConnectionSQL: "KILL CONNECTION %s",
	}
	Postgres = Dialect{
		IdentQuote:             '"',
		NamespaceIsSchema:      true,
		ParamPrefix:            "$",
		DollarQuotes:           true,
		MultiRowValues:         true,
		EmptyInsert:            " DEFAULT VALUES",
		LikeInColumnList:       true,
		OverridesIdentity:      true,
		MergesOnConflict:       true,
		IndexIsObject:          true,
		ForeignKeyIsConstraint: true,
		TextType:               "TEXT",
		WordBoundary:           `\y`,
		TriggerOnTable:         true,
		Starters: SourceStarter{
			View: "CREATE OR REPLACE VIEW %s AS\nSELECT 1",
			Function: "CREATE OR REPLACE FUNCTION %s()\nRETURNS integer\nLANGUAGE plpgsql\n" +
				"AS $$\nBEGIN\n  RETURN 0;\nEND;\n$$",
			Procedure: "CREATE OR REPLACE PROCEDURE %s()\nLANGUAGE plpgsql\n" +
				"AS $$\nBEGIN\n  NULL;\nEND;\n$$",
			// A PostgreSQL trigger has no body of its own: it *calls* a function declared
			// separately, which is why this skeleton names one rather than opening a BEGIN.
			Trigger: "CREATE TRIGGER %s\nBEFORE INSERT ON %t\nFOR EACH ROW\n" +
				"EXECUTE FUNCTION trigger_function()",
			// There is no `OR REPLACE` for either of these, which is why they alone among this
			// engine's starters do not say it: a save of one is the drop and the create the preview
			// shows.
			MaterializedView: "CREATE MATERIALIZED VIEW %s AS\nSELECT 1",
			Sequence: "CREATE SEQUENCE %s\n" +
				"AS bigint\nSTART WITH 1\nINCREMENT BY 1\nNO MINVALUE\nNO MAXVALUE\nCACHE 1",
		},
		RefreshViewSQL:   "REFRESH MATERIALIZED VIEW %s",
		CommentsOnObject: true,
		Partitions:       PartitionAsTable,
		Calls:            CallDirect,
		// A role and a user are one object here, telling apart only by whether it may log in — which
		// is why `LOGIN` is an attribute rather than a second kind of thing to declare.
		AccountKeyword:   "ROLE",
		PasswordClause:   "PASSWORD %s",
		AttributesInWith: true,
		GrantsOnTable:    true,
		RoleAdminOption:  true,
		RenameAccountSQL: "ALTER ROLE %s RENAME TO %t",
		AccountAttrs: []AccountAttr{
			{
				Name: "login", Label: "Can log in", Kind: driver.AttributeCheck,
				On: "LOGIN", Off: "NOLOGIN",
				Help: "A role that cannot log in is what every other engine calls a role.",
			},
			{
				Name: "superuser", Label: "Superuser", Kind: driver.AttributeCheck,
				On: "SUPERUSER", Off: "NOSUPERUSER",
			},
			{
				Name: "createdb", Label: "Can create databases", Kind: driver.AttributeCheck,
				On: "CREATEDB", Off: "NOCREATEDB",
			},
			{
				Name: "createrole", Label: "Can create roles", Kind: driver.AttributeCheck,
				On: "CREATEROLE", Off: "NOCREATEROLE",
			},
			{
				Name: "inherit", Label: "Inherits its roles' privileges", Kind: driver.AttributeCheck,
				On: "INHERIT", Off: "NOINHERIT",
			},
			{
				Name: "replication", Label: "Replication", Kind: driver.AttributeCheck,
				On: "REPLICATION", Off: "NOREPLICATION",
			},
			{
				Name: "bypassrls", Label: "Bypasses row security", Kind: driver.AttributeCheck,
				On: "BYPASSRLS", Off: "NOBYPASSRLS",
			},
			{
				Name: "connectionLimit", Label: "Connection limit", Kind: driver.AttributeNumber,
				On:   "CONNECTION LIMIT %s",
				Help: "-1 is no limit, which is the default.",
			},
			{
				Name: "validUntil", Label: "Password valid until", Kind: driver.AttributeText,
				On:   "VALID UNTIL %s",
				Help: "A timestamp the server will parse, such as 2027-01-01.",
			},
		},
		// Both are *functions* here rather than statements, which is why they are read as a SELECT
		// and still spelled by one flag: a function that answers `false` is a backend that had already
		// gone, which is the server's own answer and not a failure.
		CancelStatementSQL: "SELECT pg_cancel_backend(%s)",
		CloseConnectionSQL: "SELECT pg_terminate_backend(%s)",
	}
	// SQLite is one file and therefore one namespace: `main`, which every name is qualified by
	// exactly as a MySQL name is qualified by its database. Its upsert is PostgreSQL's word for
	// word, and its search is spelled as an expression throughout — see MatchesByRegex, and the
	// `regexp` function the adapter registers to answer it.
	SQLite = Dialect{
		IdentQuote:             '"',
		MultiRowValues:         true,
		EmptyInsert:            " DEFAULT VALUES",
		CopyAsSelect:           true,
		MergesOnConflict:       true,
		IndexIsObject:          true,
		QualifiesIndexName:     true,
		ObjectTableIsBare:      true,
		KeepsColumnAsDeclared:  true,
		ForeignKeyIsConstraint: true,
		TextType:               "TEXT",
		RegexFlagsInPattern:    true,
		MatchesByRegex:         true,
		WordBoundary:           `\b`,
		// A function here belongs to the program that opened the file, so there is no routine to
		// start — which is the same thing `hasRoutines` says on the frontend. A trigger is the one
		// program a SQLite file *does* hold, and its body is statements rather than a language.
		Starters: SourceStarter{
			View:    "CREATE VIEW %s AS\nSELECT 1",
			Trigger: "CREATE TRIGGER %s\nAFTER INSERT ON %t\nBEGIN\n  SELECT 1;\nEND",
		},
	}
	// SQLServer names a table by its database *and* its schema, which is what lets one pool read
	// the whole server. A default there is a constraint rather than a fact of the column, so an
	// altered column is two statements — see AlterColumn.
	SQLServer = Dialect{
		IdentQuote:             '[',
		IdentClose:             ']',
		NamespaceIsSchema:      true,
		NamespaceIsBoth:        true,
		ParamPrefix:            "@p",
		FetchPaging:            true,
		PagingNeedsOrder:       true,
		MultiRowValues:         true,
		EmptyInsert:            " DEFAULT VALUES",
		SelectIntoCopy:         true,
		MergeByExists:          true,
		RestatesColumn:         true,
		RestateClause:          "ALTER COLUMN",
		SeparatesDefault:       true,
		AltersColumnOnly:       true,
		DropsIndexOnTable:      true,
		ForeignKeyIsConstraint: true,
		TextType:               "NVARCHAR(MAX)",
		RegexIsFunction:        true,
		SensitiveCollation:     "Latin1_General_BIN2",
		WordBoundary:           `\b`,
		Starters: SourceStarter{
			View:      "CREATE VIEW %s AS\nSELECT 1 AS one",
			Function:  "CREATE FUNCTION %s()\nRETURNS int\nAS\nBEGIN\n  RETURN 0\nEND",
			Procedure: "CREATE PROCEDURE %s\nAS\nBEGIN\n  SET NOCOUNT ON;\nEND",
			// SQL Server has no FOR EACH ROW: a trigger there fires once for the statement and reads
			// the rows out of `inserted` and `deleted`.
			Trigger: "CREATE TRIGGER %s\nON %t\nAFTER INSERT\nAS\nBEGIN\n  SET NOCOUNT ON;\nEND",
			// A sequence and nothing else: an indexed view here is a view with an index on it rather
			// than an object of its own, and a scheduled job belongs to SQL Server Agent — a service
			// beside the engine, whose jobs are made by calling `msdb.dbo.sp_add_job` rather than by
			// a statement anything could read back and put again.
			Sequence: "CREATE SEQUENCE %s\nAS bigint\nSTART WITH 1\nINCREMENT BY 1",
		},
		Calls: CallInBatch,
		// The one engine where an account is two objects: a **login** the server knows and a **user**
		// a database knows. A login with no user in a database can sign in and reach nothing inside
		// it, which is why declaring one is two statements and dropping one is two the other way.
		LoginIsSeparate:    true,
		PasswordClause:     "PASSWORD = %s",
		AttributesInWith:   true,
		GrantsByClass:      true,
		ColumnsAfterObject: true,
		HasDeny:            true,
		RoleByMembership:   true,
		RenameAccountSQL:   "ALTER USER %s WITH NAME = %t",
		AccountAttrs: []AccountAttr{
			{
				Name: "defaultDatabase", Label: "Default database", Kind: driver.AttributeChoice,
				Ident: true, Login: true, On: "DEFAULT_DATABASE = %s",
			},
			{
				Name: "checkPolicy", Label: "Enforce password policy", Kind: driver.AttributeCheck,
				Login: true, On: "CHECK_POLICY = ON", Off: "CHECK_POLICY = OFF",
			},
			{
				Name: "defaultSchema", Label: "Default schema", Kind: driver.AttributeChoice,
				Ident: true, On: "DEFAULT_SCHEMA = %s",
			},
			// `DISABLE` has no `WITH` to join, so it is a statement of its own — see AccountAttr.Bare.
			{
				Name: "disabled", Label: "Login disabled", Kind: driver.AttributeCheck,
				Login: true, Bare: true, On: "DISABLE", Off: "ENABLE",
			},
		},
		// There is no cancel here: `KILL` ends the session, and the closest thing to cancelling one
		// statement — `KILL <spid> WITH STATUSONLY` — only reports how far a rollback has got. An
		// engine that has not got it says so rather than ending a session the reader meant to keep.
		CloseConnectionSQL: "KILL %s",
	}
	// Oracle has no databases below its service, so the schema *is* what a Namespace's Database
	// holds and a name is qualified by one level exactly as MySQL's is. Everything else here is
	// what an engine older than the standard looks like: no LIMIT, no multi-row VALUES, no bare
	// SELECT, and an insert that may already be there written as two ordinary statements.
	Oracle = Dialect{
		IdentQuote:             '"',
		ParamPrefix:            ":",
		FetchPaging:            true,
		DualTable:              "dual",
		CopyAsSelect:           true,
		MergeByExists:          true,
		RestatesColumn:         true,
		RestateClause:          "MODIFY",
		IndexIsObject:          true,
		QualifiesIndexName:     true,
		ForeignKeyIsConstraint: true,
		TextType:               "VARCHAR2(4000)",
		RegexIsFunction:        true,
		WordBoundary:           `\b`,
		// A PL/SQL block ends at its own `END;`, and that semicolon is part of the statement rather
		// than a terminator after it — which is why nothing anywhere here trims one.
		Starters: SourceStarter{
			View:      "CREATE OR REPLACE VIEW %s AS\nSELECT 1 FROM dual",
			Function:  "CREATE OR REPLACE FUNCTION %s\nRETURN NUMBER\nIS\nBEGIN\n  RETURN 0;\nEND;",
			Procedure: "CREATE OR REPLACE PROCEDURE %s\nIS\nBEGIN\n  NULL;\nEND;",
			Trigger: "CREATE OR REPLACE TRIGGER %s\nBEFORE INSERT ON %t\nFOR EACH ROW\n" +
				"BEGIN\n  NULL;\nEND;",
			MaterializedView: "CREATE MATERIALIZED VIEW %s AS\nSELECT 1 FROM dual",
			Sequence:         "CREATE SEQUENCE %s\nSTART WITH 1\nINCREMENT BY 1\nNOCACHE\nNOCYCLE",
		},
		// A refresh here is a procedure rather than a statement: the block around it is what makes
		// the call one, and the view it names is an argument and so a string.
		RefreshViewSQL:       "BEGIN DBMS_MVIEW.REFRESH(%s); END;",
		RefreshNamesString:   true,
		CommentsOnObject:     true,
		Partitions:           PartitionAsClause,
		Calls:                CallInBlock,
		PasswordClause:       "IDENTIFIED BY %s",
		PasswordIsIdentifier: true,
		// A privilege over the server is *administered* here rather than granted, which is the one
		// engine that draws the distinction in its grammar. There is no RenameAccountSQL at all: a
		// user is a schema, and its name is on every object inside it.
		SystemAdminOption: true,
		RoleAdminOption:   true,
		DropAccountSuffix: " CASCADE",
		AccountAttrs: []AccountAttr{
			{
				Name: "tablespace", Label: "Default tablespace", Kind: driver.AttributeChoice,
				Ident: true, On: "DEFAULT TABLESPACE %s",
			},
			{
				Name: "temporaryTablespace", Label: "Temporary tablespace",
				Kind: driver.AttributeChoice, Ident: true, On: "TEMPORARY TABLESPACE %s",
			},
			{
				Name: "profile", Label: "Profile", Kind: driver.AttributeChoice,
				Ident: true, On: "PROFILE %s",
			},
			{
				Name: "locked", Label: "Account locked", Kind: driver.AttributeCheck,
				On: "ACCOUNT LOCK", Off: "ACCOUNT UNLOCK",
			},
			{
				Name: "expired", Label: "Password expired", Kind: driver.AttributeCheck,
				On:   "PASSWORD EXPIRE",
				Help: "Setting a new password is what clears this.",
			},
		},
		// A session here **is** `sid,serial#`, in one string literal, because a sid is handed to the
		// next session the moment this one ends. `IMMEDIATE` is what makes the statement return
		// without waiting for the session to notice it has been ended.
		CloseConnectionSQL: "ALTER SYSTEM KILL SESSION %s IMMEDIATE",
		ProcessIsPair:      true,
	}
	// Shell is the lexical half of a document store's own language, and is a Dialect for one
	// reason: so `Lex` reads it rather than a second scanner being written. `//` opens a comment,
	// both quotes open a string — there is no identifier quote to take one of them — and nothing
	// else in this package is meant for it.
	Shell = Dialect{SlashComments: true}
)

// CloseQuote is the character that ends a quoted name, which is the one that opened it on every
// engine but SQL Server.
func (d Dialect) CloseQuote() byte {
	if d.IdentClose != 0 {
		return d.IdentClose
	}
	return d.IdentQuote
}

// QuoteIdent writes a name the engine will read as exactly that name. What is doubled is the
// *closing* quote, since that is the only character that could end it early.
func (d Dialect) QuoteIdent(name string) string {
	closing := string(d.CloseQuote())
	return string(d.IdentQuote) +
		strings.ReplaceAll(name, closing, closing+closing) + closing
}

// QuoteString writes a value as a string literal, which is what a DDL clause that takes an
// encoding, a locale or a comment needs: DDL binds no arguments, so the text goes in verbatim
// and this is the only thing standing between it and the statement.
func (d Dialect) QuoteString(value string) string {
	escaped := value
	if d.BackslashEscapes {
		escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	}
	return "'" + strings.ReplaceAll(escaped, "'", "''") + "'"
}

func (d Dialect) QuoteAll(names []string) []string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = d.QuoteIdent(name)
	}
	return quoted
}

// Qualify names a table inside the namespace the UI addresses it by — a database in
// MySQL, a schema in PostgreSQL.
func (d Dialect) Qualify(namespace, table string) string {
	return d.QuoteIdent(namespace) + "." + d.QuoteIdent(table)
}

// Placeholder is the nth bound argument, counting from one. Most engines write every one of them
// the same way and never read the number; the three that number them are told which.
func (d Dialect) Placeholder(position int) string {
	if d.ParamPrefix == "" {
		return "?"
	}
	return d.ParamPrefix + strconv.Itoa(position)
}

// Page is the tail of a bounded read: the order it is cut in, and the bounds themselves. Both
// spellings are here because they are one decision — a page is `LIMIT … OFFSET …` or it is
// `OFFSET … ROWS FETCH NEXT … ROWS ONLY`, and nothing else about the read changes.
//
// The bounds are written in rather than bound, for the reason a filter's values are: one string is
// then both what runs and what the reader is shown. They are numbers this side counts, never
// anything a reader typed.
//
// `order` arrives already spelled, leading with a space or empty. An engine that refuses to page
// without one is given an order that orders nothing rather than an arbitrary column, since a
// column picked here would silently decide which rows the reader sees.
func (d Dialect) Page(order string, limit, offset int) string {
	if limit <= 0 {
		return order
	}
	if !d.FetchPaging {
		return order + " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
	}
	if order == "" && d.PagingNeedsOrder {
		order = " ORDER BY (SELECT NULL)"
	}
	return order +
		" OFFSET " + strconv.Itoa(offset) + " ROWS" +
		" FETCH NEXT " + strconv.Itoa(limit) + " ROWS ONLY"
}

// From is the FROM clause of a read that has no table of its own, which is a clause at all only on
// an engine that insists a SELECT have one.
func (d Dialect) From() string {
	if d.DualTable == "" {
		return ""
	}
	return " FROM " + d.DualTable
}
