// Package sqlkit holds the parts of a database/sql adapter that do not vary by engine:
// row scanning, result-column reflection, statement classification, and reading the table
// a result can be written back to. Engines differ only in the lexical details a Dialect
// carries, which leaves an adapter its DSN, its error map and its catalogue queries.
package sqlkit

import "strings"

// Dialect is the lexical difference between engines: the character that quotes an
// identifier, whether `#` starts a line comment, and which part of a Namespace qualifies a
// table name.
type Dialect struct {
	IdentQuote   byte
	HashComments bool
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
}

var (
	MySQL    = Dialect{IdentQuote: '`', HashComments: true, BackslashEscapes: true}
	Postgres = Dialect{IdentQuote: '"', NamespaceIsSchema: true, DollarQuotes: true}
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
