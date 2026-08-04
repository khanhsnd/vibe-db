package sqlkit

import "strings"

// The three lists below are what Format reads a script's shape from. They are shared by both
// engines on purpose: a word one of them does not know is laid out as though it were a name,
// which is untidy at worst — where a *per-engine* list would lay the same script out two ways
// depending on which connection the tab happened to be on.

// blockClauses put their own word on a line and everything that follows it on the next, indented.
// They are the clauses whose argument is a *list* — the columns of a SELECT, the conditions of a
// WHERE, the assignments of a SET — which is what makes the indent worth the height it costs.
var blockClauses = phrases(
	"SELECT DISTINCT ON", "SELECT DISTINCT", "SELECT",
	"FROM", "WHERE", "GROUP BY", "HAVING", "ORDER BY", "PARTITION BY",
	"SET", "VALUES", "RETURNING", "WINDOW",
)

// inlineClauses start a line of their own and keep their argument on it: there is nothing to
// indent under `LIMIT 100`, and `UPDATE shop.users` reads worse in two lines than in one.
var inlineClauses = phrases(
	"WITH RECURSIVE", "WITH",
	"INSERT INTO", "INSERT IGNORE INTO", "REPLACE INTO", "DELETE FROM", "UPDATE",
	"UNION ALL", "UNION", "INTERSECT ALL", "INTERSECT", "EXCEPT ALL", "EXCEPT",
	"LIMIT", "OFFSET", "FETCH FIRST", "FETCH NEXT",
	"ON CONFLICT", "ON DUPLICATE KEY UPDATE",
	"CREATE TABLE", "CREATE TEMPORARY TABLE", "CREATE OR REPLACE VIEW", "CREATE VIEW",
	"CREATE UNIQUE INDEX", "CREATE INDEX", "CREATE DATABASE", "CREATE SCHEMA",
	"ALTER TABLE", "DROP TABLE", "TRUNCATE TABLE", "DROP INDEX", "DROP VIEW",
	"EXPLAIN ANALYZE", "EXPLAIN",
)

// itemClauses hang under the clause in force rather than beside it: a join belongs to the FROM
// above it, and an `AND` to the condition above it, so both are indented with the items.
var itemClauses = phrases(
	"LEFT OUTER JOIN", "RIGHT OUTER JOIN", "FULL OUTER JOIN",
	"LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "INNER JOIN", "CROSS JOIN",
	"NATURAL JOIN", "STRAIGHT_JOIN", "JOIN",
	"AND", "OR",
)

// keywords is only ever used to decide the *case* a word is written in, which is why it holds
// type names and function-ish words the lists above have no interest in. A name that collides
// with one of them is upper-cased and still names the same thing: neither engine folds case on
// its way to a table, and one written in quotes is a TokenIdent and never reaches here at all.
var keywords = words(`
	add all alter analyze and any as asc auto_increment before begin between bigint binary bit
	blob bool boolean both by call cascade case cast char character check collate column comment
	commit constraint create cross current_date current_time current_timestamp current_user
	cursor database databases date datetime day dec decimal declare default deferrable delete
	desc describe distinct div do double drop each else elseif enabled end enum escape except
	exists explain false fetch first float for force foreign from full function generated grant
	group having high_priority hour if ignore ilike in index inner inout insert int integer
	intersect interval into is join key keys leading leave left like limit lines load localtime
	localtimestamp lock long longblob longtext loop match materialized mediumblob mediumint
	mediumtext minute mod modifies month natural no not null numeric offset on only open
	optimize option or order out outer over partition precision primary procedure range read
	real recursive references rename repeat replace restrict return returning returns revoke
	right rlike rollback row rows savepoint schema second select sequence serial session set
	show similar smallint some start starting stored straight_join table tables temporary text
	then time timestamp tinyblob tinyint tinytext to trailing transaction trigger true truncate
	type union unique unlock unsigned update usage use using uuid values varbinary varchar
	varying view virtual when where while window with write xml year zerofill

	abs avg cast ceil coalesce concat count date_trunc extract floor greatest ifnull json_agg
	json_build_object least length lower ltrim max min now nullif round row_number rtrim
	string_agg substring sum trim upper
`)

// subqueryHeads are the words that make an opening parenthesis worth breaking a line for
// whatever its length: `(SELECT …)` is a statement inside a statement and reads as one.
var subqueryHeads = words("select with values table")

// clauseAt matches the longest clause phrase starting at `i`, and answers with how many tokens it
// took. Longest-first is the whole point: `GROUP BY` is one clause and `LEFT OUTER JOIN` is one
// join, and a shorter entry matching first would break both in half.
func clauseAt(tokens []Token, i int, list [][]string) (int, string, bool) {
	for _, phrase := range list {
		if i+len(phrase) > len(tokens) {
			continue
		}
		matched := true
		for n, word := range phrase {
			at := tokens[i+n]
			if at.Kind != TokenWord || !strings.EqualFold(at.Text, word) {
				matched = false
				break
			}
		}
		if matched {
			return len(phrase), strings.Join(phrase, " "), true
		}
	}
	return 0, "", false
}

// phrases cuts each entry into its words and sorts them by length, longest first, so clauseAt can
// walk the list in order and take the first match: `SELECT DISTINCT ON` has to be tried before
// `SELECT` can claim its first word. The sort is stable, so entries of the same length stay in
// the order they are written above and a new one can be added anywhere in its family.
func phrases(list ...string) [][]string {
	out := make([][]string, 0, len(list))
	for _, entry := range list {
		out = append(out, strings.Fields(entry))
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(out[j]) > len(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(list) {
		set[word] = true
	}
	return set
}
