// Package sqlkit holds the parts of a database/sql adapter that do not vary by engine:
// row scanning, result-column reflection, statement classification, and reading the table
// a result can be written back to. Engines differ only in the lexical details a Dialect
// carries, which leaves an adapter its DSN, its error map and its catalogue queries.
package sqlkit

import "strings"

// Dialect is the lexical difference between engines: the character that quotes an
// identifier, whether `#` starts a line comment, and which part of a Namespace qualifies a
// table name.
//
// **Lex is not only SQL's.** Where a statement ends, what a string is and what a comment is are
// the same questions in a document store's shell, so the MongoDB adapter carries a Dialect of its
// own for the lexer alone — see `Shell` — and nothing else in this file has anything to say to it.
type Dialect struct {
	IdentQuote   byte
	HashComments bool
	// SlashComments says `//` starts a line comment, which is what a shell script uses and
	// neither SQL engine has.
	SlashComments bool
	// NamespaceIsSchema says a table is qualified by the schema inside its database rather
	// than by the database itself, which is the one structural difference between the two
	// engines' names for the same table.
	NamespaceIsSchema bool
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
	// OverridesIdentity says a write has to declare it is supplying a value for an identity
	// column, which PostgreSQL refuses the row without and MySQL has no clause for. See CopyRows.
	OverridesIdentity bool
	// MergesOnConflict says an insert that meets a row already there says so in a clause *after*
	// the values, naming the columns it conflicts on — PostgreSQL's `ON CONFLICT`. MySQL says it
	// in the verb (`INSERT IGNORE`) and in a clause that names no columns at all, leaving the
	// server to find whichever key was broken. See InsertVerb and MergeClause.
	MergesOnConflict bool
	// RestatesColumn says an altered column is restated *whole* — MySQL's `MODIFY COLUMN`, where a
	// fact left out is a fact dropped. PostgreSQL has no such clause and takes one `ALTER COLUMN`
	// action per fact instead. See AlterColumn.
	RestatesColumn bool
	// MethodBeforeTable says an index's access method sits between its name and the table —
	// MySQL's `CREATE INDEX n USING m ON t (…)`. PostgreSQL takes it after the table.
	MethodBeforeTable bool
	// IndexIsObject says an index is an object of the schema in its own right and is dropped by
	// name — PostgreSQL's `DROP INDEX`. On MySQL an index belongs to its table and is dropped
	// through it.
	IndexIsObject bool
	// ForeignKeyIsConstraint says a referential constraint is dropped as a constraint —
	// PostgreSQL's `DROP CONSTRAINT`. MySQL has a clause of its own, `DROP FOREIGN KEY`.
	ForeignKeyIsConstraint bool
	// TextType is what this engine calls text in a CAST, which is how a column of any type is
	// compared as text — see Where's `contains`.
	TextType string
	// RegexIsFunction says a regular-expression match is a function whose third argument is the
	// case sensitivity — MySQL's `REGEXP_LIKE`. PostgreSQL has an operator per answer, `~` and `~*`.
	RegexIsFunction bool
	// PatternIgnoresCase says this engine's own collation already makes LIKE blind to case, so a
	// match the reader asked to be case-*sensitive* is the one that has to say so — see MatchAny.
	// MySQL's usual collations do; PostgreSQL's LIKE is sensitive as it stands.
	PatternIgnoresCase bool
	// WordBoundary is this engine's regular-expression token for the edge of a word, and is the one
	// piece of that syntax the two spell differently: `\b` on MySQL's ICU, and `\y` on PostgreSQL,
	// where `\b` is a backspace.
	WordBoundary string
}

var (
	MySQL = Dialect{
		IdentQuote:         '`',
		HashComments:       true,
		BackslashEscapes:   true,
		RestatesColumn:     true,
		MethodBeforeTable:  true,
		TextType:           "CHAR",
		RegexIsFunction:    true,
		PatternIgnoresCase: true,
		WordBoundary:       `\b`,
	}
	Postgres = Dialect{
		IdentQuote:             '"',
		NamespaceIsSchema:      true,
		DollarQuotes:           true,
		LikeInColumnList:       true,
		OverridesIdentity:      true,
		MergesOnConflict:       true,
		IndexIsObject:          true,
		ForeignKeyIsConstraint: true,
		TextType:               "TEXT",
		WordBoundary:           `\y`,
	}
	// Shell is the lexical half of a document store's own language, and is a Dialect for one
	// reason: so `Lex` reads it rather than a second scanner being written. `//` opens a comment,
	// both quotes open a string — there is no identifier quote to take one of them — and nothing
	// else in this package is meant for it.
	Shell = Dialect{SlashComments: true}
)

func (d Dialect) QuoteIdent(name string) string {
	quote := string(d.IdentQuote)
	return quote + strings.ReplaceAll(name, quote, quote+quote) + quote
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
