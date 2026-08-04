package mongo

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	// indentUnit is one level of indent, and a tab for the reason `sqlkit` uses one: it is what
	// the editor's own snippets already write.
	indentUnit = "\t"

	// breakItems is how many things a brace may hold before it is laid out down the page rather
	// than across it. It counts items and not characters so that formatting twice is the same as
	// formatting once — see sqlkit.Format, which counts them for the same reason.
	breakItems = 3
)

// text is the lexical half of this engine: where a command ends, how a command is laid out, and
// what each command in a script is going to do. It is its own type rather than a `sqlkit.Dialect`
// because only the *scanning* is shared — a document store has no clauses to lay out and no
// keywords to upper-case, and its verb is a method rather than a first word.
type text struct{}

// Split cuts a script into the commands it holds, over the same tokens everything else here reads:
// a `;` that is not inside a string, a comment or a brace ends a command.
func (text) Split(script string) []string {
	return dialect.Split(script)
}

// Review reads what a script is going to do without sending any of it anywhere.
//
// It reports three things, and every one of them it can prove: a quote or a comment the script ends
// inside of, a bracket with no partner, and — this being a language with a small enough grammar to
// read whole — a command this side cannot make sense of at all. That last one is the difference
// from SQL, where guessing at a statement's shape would cost the reader more than it saved: here
// the whole grammar is four productions, so a command that will not parse will not run either.
//
// **A command that is merely unfinished is not one of them.** This runs on every keystroke, and half
// of `db.users.f` is what every command looks like on the way to being written; a squiggle that is
// under the text the whole time it is being typed is one the reader stops seeing. Nothing is lost by
// keeping quiet, because a command truncated for real is a bracket or a quote with no partner and
// that is already the first thing here — see `unfinished`.
//
// What a command *does* is `domain.MongoHazard`'s rule and not this file's.
func (text) Review(script string) driver.Review {
	problems := sqlkit.Problems(dialect.Lex(script))
	spans := dialect.Spans(script)

	statements := make([]driver.ReviewedStatement, 0, len(spans))
	for _, span := range spans {
		command := script[span.Start:span.End]
		reviewed := driver.ReviewedStatement{Start: span.Start, End: span.End, SQL: command}

		one, err := parse(command)
		if err != nil {
			if !stillTyping(err) {
				problems = append(problems, driver.Problem{
					Start:   span.Start,
					End:     span.End,
					Message: err.Error(),
				})
			}
			statements = append(statements, reviewed)
			continue
		}

		reviewed.Keyword, reviewed.Hazard = verb(one)
		statements = append(statements, reviewed)
	}

	return driver.Review{Statements: statements, Problems: problems}
}

// verb is the method a command is answered for, and what that method will do. `runCommand` is the
// one whose reach is its argument's rather than its own name's — the *first field* of a command
// document is the command, so `db.runCommand({ drop: "users" })` is read as the drop it is.
func verb(one statement) (string, domain.Hazard) {
	method, filtered := one.Method, one.Filtered()

	if strings.EqualFold(method, "runCommand") {
		if fields := one.Filter(); len(fields) > 0 {
			// The *first field* of a command document is the command: `{ drop: "users" }` names one
			// collection and `{ dropDatabase: 1 }` names a whole database.
			//
			// It is answered for as though it were unfiltered, because a command carried this way
			// keeps its filter out of reach — an update's own `q` is nested inside an array — and a
			// warning the reader can dismiss is cheaper than one they never saw.
			return fields[0].Key, domain.MongoHazard(fields[0].Key, false)
		}
		return method, domain.MongoHazard(method, filtered)
	}

	return method, domain.MongoHazard(method, filtered)
}

// Format lays a command out: one field to a line where a brace holds more than a line comfortably
// carries, and across the line where it does not.
//
// **Nothing a server would notice changes.** A string, a regular expression and a comment are
// single tokens and are written back byte for byte, so only the whitespace between tokens moves. A
// script the lexer cannot make sense of — one ending inside a quote — is handed back untouched
// rather than guessed at, for the reason `sqlkit.Format` does the same: the one thing worse than an
// unformatted script is a formatted one that no longer runs.
func (text) Format(script string) string {
	tokens := dialect.Lex(script)
	if len(tokens) == 0 {
		return strings.TrimSpace(script)
	}
	for _, one := range tokens {
		if one.Unterminated {
			return script
		}
	}

	formatted := make([]string, 0, 4)
	for _, chunk := range commandTokens(tokens) {
		if laid := layout(chunk); laid != "" {
			formatted = append(formatted, laid)
		}
	}
	if len(formatted) == 0 {
		return strings.TrimSpace(script)
	}

	// The last command keeps whatever terminator it had, so a reader who formatted one command
	// does not find a semicolon they never typed.
	tail := ""
	if last := tokens[len(tokens)-1]; last.Kind == sqlkit.TokenPunct && last.Text == ";" {
		tail = ";"
	}
	return strings.Join(formatted, ";\n\n") + tail
}

// commandTokens cuts the token list at the semicolons between commands, keeping a chunk that holds
// nothing but comments: a formatter that dropped the reader's notes is one nobody uses twice.
func commandTokens(tokens []sqlkit.Token) [][]sqlkit.Token {
	chunks := [][]sqlkit.Token{}
	start := 0

	for i, one := range tokens {
		if one.Kind == sqlkit.TokenPunct && one.Text == ";" {
			if start < i {
				chunks = append(chunks, tokens[start:i])
			}
			start = i + 1
		}
	}
	if start < len(tokens) {
		chunks = append(chunks, tokens[start:])
	}
	return chunks
}

// noSpaceBefore and noSpaceAfter are the punctuation nothing goes between. `.` is in both, because
// `db.users.find` is one path written in five tokens.
//
// A brace is in them too, so the *only* thing that ever puts a space inside one is `pad` — which is
// what lets `{}` close up and `{ a: 1 }` stay open without the two rules disagreeing.
var (
	noSpaceBefore = set(",", ";", ")", "]", "}", ".", ":", "(")
	noSpaceAfter  = set("(", "[", "{", ".", "-")
)

func set(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

// layout writes one command. It walks the tokens once and decides two things about each: whether a
// line starts before it, and whether a space goes in front of it.
//
// **A parenthesis is never a block**, only a brace or a bracket is. That is the whole of what makes
// the result read the way every other client writes one: `find({` stays on its line and the fields
// go under it, rather than the call's own parenthesis earning a line of its own first.
func layout(tokens []sqlkit.Token) string {
	match := matchBrackets(tokens)
	out := &writer{}

	// blocks is the indent each open block came back out to, innermost last. `inline` counts the
	// braces that were *not* worth breaking: inside one, nothing starts a line.
	blocks := []int{}
	inline := 0
	previous := ""

	put := func(one sqlkit.Token) {
		out.write(one.Text, spaced(previous, one.Text))
		previous = one.Text
	}
	start := func(indent int) {
		out.newline(indent)
		previous = ""
	}

	for i, one := range tokens {
		block := one.Kind == sqlkit.TokenPunct && braces(one.Text)

		switch {
		case block && opens(one.Text):
			if inline > 0 || !wide(tokens, match, i, match[i]) {
				inline++
				put(one)
				// An inline brace reads as `{ a: 1 }`, which is what every client spells one as.
				if one.Text == "{" {
					out.pad()
				}
				break
			}
			put(one)
			blocks = append(blocks, out.indent)
			start(out.indent + 1)

		case block && closes(one.Text):
			if inline > 0 {
				inline--
				if one.Text == "}" {
					out.closePad()
				}
				put(one)
				break
			}
			if len(blocks) > 0 {
				start(blocks[len(blocks)-1])
				blocks = blocks[:len(blocks)-1]
			}
			put(one)

		case one.Kind == sqlkit.TokenPunct && one.Text == ",":
			put(one)
			if inline == 0 && len(blocks) > 0 {
				start(out.indent)
			}

		case one.Kind == sqlkit.TokenComment:
			put(one)
			// A line comment runs to the end of its line by definition, so whatever follows it has
			// to start a new one or be swallowed.
			if !strings.HasPrefix(one.Text, "/*") {
				start(out.indent)
			}

		default:
			put(one)
		}
	}

	return out.String()
}

// spaced decides whether a space goes between two tokens. All of it is fixed by the punctuation,
// there being no call-versus-list ambiguity in this grammar: `find(` is always a call.
func spaced(previous, next string) bool {
	if previous == "" {
		return false
	}
	return !noSpaceBefore[next] && !noSpaceAfter[previous]
}

// wide says a brace is worth laying out down the page: one holding more items than a line carries,
// or one holding a brace that is itself a block — otherwise the parent would read as one line
// holding several. An unmatched brace earns neither; there is no line to close it on.
func wide(tokens []sqlkit.Token, match []int, open, close int) bool {
	if close < 0 {
		return false
	}
	if items(tokens, open, close) > breakItems {
		return true
	}

	for i := open + 1; i < close; i++ {
		if tokens[i].Kind != sqlkit.TokenPunct || !braces(tokens[i].Text) || !opens(tokens[i].Text) {
			continue
		}
		if wide(tokens, match, i, match[i]) {
			return true
		}
	}
	return false
}

// items counts what a bracket holds, by the commas directly inside it.
func items(tokens []sqlkit.Token, open, close int) int {
	count := 1
	depth := 0

	for i := open + 1; i < close; i++ {
		if tokens[i].Kind != sqlkit.TokenPunct {
			continue
		}
		switch {
		case opens(tokens[i].Text):
			depth++
		case closes(tokens[i].Text):
			depth--
		case tokens[i].Text == "," && depth == 0:
			count++
		}
	}
	return count
}

// braces says this bracket is one a block can be laid out inside. A parenthesis is not one of
// them — see layout.
func braces(text string) bool {
	return text == "{" || text == "}" || text == "[" || text == "]"
}

func opens(text string) bool {
	return text == "{" || text == "[" || text == "("
}

func closes(text string) bool {
	return text == "}" || text == "]" || text == ")"
}

// matchBrackets pairs every bracket with its partner, answering -1 for one that has none. It is
// computed once because `wide` asks about the width of a brace before writing its first byte,
// which cannot be answered by walking forwards.
func matchBrackets(tokens []sqlkit.Token) []int {
	match := make([]int, len(tokens))
	for i := range match {
		match[i] = -1
	}

	open := []int{}
	for i, one := range tokens {
		if one.Kind != sqlkit.TokenPunct {
			continue
		}
		switch {
		case opens(one.Text):
			open = append(open, i)
		case closes(one.Text):
			if len(open) > 0 {
				last := open[len(open)-1]
				open = open[:len(open)-1]
				match[last], match[i] = i, last
			}
		}
	}
	return match
}

// writer builds the laid-out text one line at a time, which is what lets every rule above say
// "start a line here" without any of them knowing what came before.
type writer struct {
	out    strings.Builder
	line   strings.Builder
	indent int
	// owed is a space asked for but not yet written, which is what an inline brace's inner padding
	// is: a space nothing followed is a space that was never wanted.
	owed bool
}

func (w *writer) write(text string, space bool) {
	switch {
	case w.owed:
		w.line.WriteByte(' ')
		w.owed = false
	case w.line.Len() > 0 && space:
		w.line.WriteByte(' ')
	}
	w.line.WriteString(text)
}

// pad asks for the single space inside an inline brace, which is written only if something follows
// it — see owed.
func (w *writer) pad() {
	w.owed = w.line.Len() > 0
}

// closePad is pad from the other side. It asks for the space before a `}` *unless the `{` right
// behind it is still asking for one*, which is the whole of what makes `{}` close up while
// `{ a: 1 }` stays open.
func (w *writer) closePad() {
	if w.owed {
		w.owed = false
		return
	}
	w.pad()
}

func (w *writer) newline(indent int) {
	w.flush()
	w.indent = indent
}

func (w *writer) flush() {
	w.owed = false
	if w.line.Len() == 0 {
		return
	}
	if w.out.Len() > 0 {
		w.out.WriteByte('\n')
	}
	w.out.WriteString(strings.Repeat(indentUnit, w.indent))
	w.out.WriteString(w.line.String())
	w.line.Reset()
}

func (w *writer) String() string {
	w.flush()
	return w.out.String()
}
