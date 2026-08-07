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
	// IndexTableIsBare says the table of a CREATE INDEX carries no namespace, because the index's
	// own name already did — SQLite, whose grammar puts the schema on the index and refuses one on
	// the table. It goes with QualifiesIndexName rather than replacing it: Oracle qualifies both.
	IndexTableIsBare bool
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
	// Starters is what a reader begins a *new* view or routine from — see SourceStarter.
	Starters SourceStarter
}

// SourceStarter is the skeleton a reader starts a new view or routine from: the shape of the
// declaration with nothing in it yet. It is a **flag** in the sense every other difference here is
// — three strings per engine — and it is the dialect's rather than the frontend's for the reason
// nothing outside a driver ever spells a statement.
//
// The three differ in *grammar* rather than in quoting, which is why one speller could not do it:
// MySQL wraps a routine's body in `BEGIN … END`, PostgreSQL puts it in a string tagged with the
// language it is written in, SQL Server opens with `AS`, and Oracle says `IS`. No reader should
// have to remember which, and an engine with no routines at all leaves both blank.
//
// Each holds one `%s`, which is the qualified name — the same name every other statement here is
// addressed by. Nothing runs one of these: what a Save runs is the reader's own buffer, which
// SourceStatements previews whole.
type SourceStarter struct {
	View      string
	Function  string
	Procedure string
}

// NewSource is the starter for this kind, addressed to a name. An engine that has no such object
// answers with nothing, which is what the frontend reads as "there is no starter" — it does not
// offer a New for a kind this engine has not got in the first place.
func (d Dialect) NewSource(ns driver.Namespace, kind driver.SourceKind, name string) string {
	starter := d.Starters.View
	switch kind {
	case driver.SourceFunction:
		starter = d.Starters.Function
	case driver.SourceProcedure:
		starter = d.Starters.Procedure
	}

	if starter == "" {
		return ""
	}
	return strings.ReplaceAll(starter, "%s", d.Table(ns, name))
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
		},
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
		Starters: SourceStarter{
			View: "CREATE OR REPLACE VIEW %s AS\nSELECT 1",
			Function: "CREATE OR REPLACE FUNCTION %s()\nRETURNS integer\nLANGUAGE plpgsql\n" +
				"AS $$\nBEGIN\n  RETURN 0;\nEND;\n$$",
			Procedure: "CREATE OR REPLACE PROCEDURE %s()\nLANGUAGE plpgsql\n" +
				"AS $$\nBEGIN\n  NULL;\nEND;\n$$",
		},
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
		IndexTableIsBare:       true,
		KeepsColumnAsDeclared:  true,
		ForeignKeyIsConstraint: true,
		TextType:               "TEXT",
		RegexFlagsInPattern:    true,
		MatchesByRegex:         true,
		WordBoundary:           `\b`,
		// A function here belongs to the program that opened the file, so there is no routine to
		// start — which is the same thing `hasRoutines` says on the frontend.
		Starters: SourceStarter{View: "CREATE VIEW %s AS\nSELECT 1"},
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
		},
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
		},
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
