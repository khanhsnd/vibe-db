package redis

import (
	"context"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// RowStatements spells rows as the commands that write them into a key space. It is the one way a
// row leaves for a server — an export, the data half of a dump and every mode of an import all come
// through here — and on this engine "a statement" is a line of argv.
//
// Two of the five modes are only partly spellable, and that is the engine's own doing rather than a
// shortcut. `SET` takes `NX` and `XX`, so a *string* can be added-only or changed-only exactly. A
// container is rebuilt by several commands with no such word between them, so **Skip and Update
// contribute nothing for one** — the alternative is a rebuild that overwrites a key the reader asked
// to be left alone, which is worse than a preview that visibly leaves it out. It is the same rule
// `AlterStatements` follows for a change an engine has no concept of.
func (*session) RowStatements(_ driver.Namespace, table string, write driver.RowWrite) []string {
	if !isKeySpace(table) {
		return nil
	}

	at := map[string]int{}
	for i, name := range write.Columns {
		if column, known := canonical(name); known {
			at[column] = i
		}
	}

	// A row with nothing to find it by matches nothing. Here it is stronger than the rule the SQL
	// engines follow: without a key there is not even a row to write, because a Redis value has
	// nowhere to be except under a name.
	keyAt, keyed := at[columnKey]
	if !keyed {
		return nil
	}

	statements := []string{}
	for _, row := range write.Rows {
		key := columnText(row, keyAt)
		if key == "" {
			continue
		}
		statements = append(statements, rowCommands(row, at, key, write.Mode)...)
	}
	return statements
}

// rowCommands is one row's worth. A string is one command and a container is three or more — empty
// what was there, fill it, then put the clock back — because that is what replacing a value is when
// the value has members.
func rowCommands(row []*string, at map[string]int, key string, mode driver.RowWriteMode) []string {
	if mode == driver.WriteDelete {
		return []string{command("UNLINK", key)}
	}

	kind := typeString
	if index, named := at[columnType]; named {
		if held := columnText(row, index); held != "" {
			kind = strings.ToLower(held)
		}
	}

	value := ""
	if index, held := at[columnValue]; held {
		value = columnText(row, index)
	}

	ttl := ""
	if index, held := at[columnTTL]; held {
		ttl = columnText(row, index)
	}

	if kind == typeString {
		args := []string{"SET", key, value}
		if seconds := positive(ttl); seconds != "" {
			args = append(args, "EX", seconds)
		}
		switch mode {
		case driver.WriteSkip:
			args = append(args, "NX")
		case driver.WriteUpdate:
			args = append(args, "XX")
		}
		return []string{command(args...)}
	}

	// A container under a mode `SET` would have spelled with a word: see the note on RowStatements.
	if mode == driver.WriteSkip || mode == driver.WriteUpdate {
		return nil
	}

	members, err := parseValue(kind, value)
	if err != nil {
		// A value this side cannot read is a row that cannot be written, and there is no error to
		// answer with here. Leaving it out is what the preview then shows the reader.
		return nil
	}

	statements := []string{command("UNLINK", key)}
	for _, one := range members {
		statements = append(statements, command(append([]string{one.name, key}, one.args...)...))
	}
	if seconds := positive(ttl); seconds != "" {
		statements = append(statements, command("EXPIRE", key, seconds))
	}
	return statements
}

// positive is a time to live worth writing. A blank column, a NULL and a zero are all "no expiry",
// and none of them is spelled: `EXPIRE key 0` deletes the key.
func positive(ttl string) string {
	seconds, err := strconv.ParseInt(strings.TrimSpace(ttl), 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return strconv.FormatInt(seconds, 10)
}

// columnText reads one column of a row, answering the empty string for a NULL and for a row that is short
// of the column — a file being imported is not required to hold every column the table has.
func columnText(row []*string, index int) string {
	if index < 0 || index >= len(row) || row[index] == nil {
		return ""
	}
	return *row[index]
}

// CreateStatements is the structure half of a dump, and a key space has no structure: it is not
// declared, it cannot be created and it cannot be dropped. What is written is a line saying so, so
// that a dump of a Redis database reads as a file about a Redis database rather than as one that
// forgot its first half.
func (*session) CreateStatements(
	_ context.Context,
	_ driver.Namespace,
	table string,
) ([]string, error) {
	if !isKeySpace(table) {
		return nil, domain.ErrUnknownTable
	}
	return []string{"# " + keySpace + ": a Redis key space is not declared, only filled"}, nil
}

// MatchingRows is the data half of a Find in Schema, and on this engine it is a search of the key
// *names*. That is not a narrowing of the feature but the whole of what the server can answer: there
// is no index over values, no LIKE and no regular expression — `SCAN … MATCH` is the one narrowing
// Redis has, and it reads a key's name.
//
// So the two modes a pattern can say are answered exactly, and the two it cannot are refused rather
// than approximated. **The count and the read are spelled from one glob**, which is the rule
// `driver.RowMatches` exists for: `KEYS` returns precisely the keys this counted, so a reader who
// double-clicks the row opens the rows that were counted and not a near miss of them.
func (s *session) MatchingRows(
	ctx context.Context,
	search driver.RowSearch,
) (driver.RowMatches, error) {
	client, err := s.client(search.Namespace)
	if err != nil {
		return driver.RowMatches{}, err
	}
	if !isKeySpace(search.Table) {
		return driver.RowMatches{}, domain.ErrUnknownTable
	}
	if err := search.Match.Validate(); err != nil {
		return driver.RowMatches{}, err
	}
	if !search.Match.IsPattern() {
		return driver.RowMatches{}, domain.Unsupported(
			"searching a key space by " + string(search.Match.Mode) +
				": a Redis server matches a key's name against a glob and nothing else",
		)
	}

	pattern := globPattern(search.Match)
	names, err := scanKeys(ctx, client, pattern)
	if err != nil {
		return driver.RowMatches{}, err
	}

	return driver.RowMatches{
		Count:     int64(len(names)),
		Statement: command("KEYS", pattern),
	}, nil
}

// globPattern spells a `domain.TextMatch` as the one thing a Redis server understands. A LIKE
// pattern and a glob are near neighbours — `%` is `*` and `_` is `?` — and the reader's own text is
// escaped so that a key holding `[` is a key they can search for.
//
// **Case is folded into the pattern itself.** Redis has no case-insensitive matching, so an
// insensitive search spells each letter as the class of its two cases: `[uU][sS][eE][rR]*`. It is
// exact rather than approximate, which is what lets the count and the read stay one clause.
func globPattern(match domain.TextMatch) string {
	pattern := match.Pattern()

	var out strings.Builder
	escaped := false

	for _, character := range pattern {
		switch {
		case escaped:
			out.WriteString(letter(character, match.CaseInsensitive))
			escaped = false
		case character == '\\':
			escaped = true
		case character == '%':
			out.WriteByte('*')
		case character == '_':
			out.WriteByte('?')
		default:
			out.WriteString(letter(character, match.CaseInsensitive))
		}
	}

	// A pattern ending in a lone backslash escapes nothing, so it is the character itself.
	if escaped {
		out.WriteString(escapeGlob(`\`))
	}
	return out.String()
}

// letter is one character of a pattern, standing for itself — and for its other case as well where
// the search asked to ignore case.
func letter(character rune, insensitive bool) string {
	lower, upper := strings.ToLower(string(character)), strings.ToUpper(string(character))
	if !insensitive || lower == upper {
		return escapeGlob(string(character))
	}
	return "[" + escapeGlob(lower) + escapeGlob(upper) + "]"
}
