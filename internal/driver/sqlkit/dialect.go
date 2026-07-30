// Package sqlkit holds the parts of a database/sql adapter that do not vary by engine:
// row scanning, result-column reflection, statement classification, and reading the table
// a result can be written back to. Engines differ only in the lexical details a Dialect
// carries, which leaves an adapter its DSN, its error map and its catalogue queries.
package sqlkit

import "strings"

// Dialect is the lexical difference between engines: the character that quotes an
// identifier, and whether `#` starts a line comment.
type Dialect struct {
	IdentQuote   byte
	HashComments bool
}

var (
	MySQL    = Dialect{IdentQuote: '`', HashComments: true}
	Postgres = Dialect{IdentQuote: '"'}
)

func (d Dialect) QuoteIdent(name string) string {
	quote := string(d.IdentQuote)
	return quote + strings.ReplaceAll(name, quote, quote+quote) + quote
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
