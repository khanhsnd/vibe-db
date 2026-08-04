package sqlkit

import "strings"

const (
	// indentUnit is one level of indent, and a tab because that is what the editor's own snippets
	// already write: a formatted script and a snippet-filled one should not indent two ways.
	indentUnit = "\t"

	// breakItems is how many things a parenthesis may hold before it is laid out as a block of
	// its own rather than as punctuation. It counts *items* and not characters on purpose:
	// formatting changes how wide a list is written, so a width test would break a parenthesis
	// on the second pass that it left alone on the first, and formatting twice has to be the
	// same as formatting once.
	breakItems = 3
)

// Format lays a script out: one clause to a line, its items indented under it, keywords in upper
// case, and one blank line between statements.
//
// **Nothing a server would notice changes.** A string, a quoted identifier and a comment are
// single tokens and are written back byte for byte, so only whitespace between tokens and the
// case of unquoted keywords ever moves. A script the lexer cannot make sense of — one ending
// inside a quote — is handed back untouched rather than guessed at, because the one thing worse
// than an unformatted script is a formatted one that no longer runs. Review is what says so.
func (d Dialect) Format(script string) string {
	tokens := d.Lex(script)
	if len(tokens) == 0 {
		return strings.TrimSpace(script)
	}
	for _, one := range tokens {
		if one.Unterminated {
			return script
		}
	}

	formatted := make([]string, 0, 4)
	for _, chunk := range statementTokens(tokens) {
		if text := layout(chunk); text != "" {
			formatted = append(formatted, text)
		}
	}
	if len(formatted) == 0 {
		return strings.TrimSpace(script)
	}

	// The last statement keeps whatever terminator it had: a reader who selected one statement
	// and formatted it should not find a semicolon they did not type.
	tail := ""
	if last := tokens[len(tokens)-1]; last.Kind == TokenPunct && last.Text == ";" {
		tail = ";"
	}
	return strings.Join(formatted, ";\n\n") + tail
}

// statementTokens cuts the token list at the semicolons between statements. Unlike Split it keeps
// a chunk holding nothing but comments, because a formatter that silently dropped the reader's
// notes would be a formatter nobody used twice.
func statementTokens(tokens []Token) [][]Token {
	chunks := [][]Token{}
	start := 0

	for i, one := range tokens {
		if one.Kind == TokenPunct && one.Text == ";" {
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

// frame is one level of parentheses that was worth breaking a line for: where a clause head
// inside it goes, where its own `)` goes — the indent the line it opened on had — and the item
// indent that was in force outside it, which closing it puts back.
type frame struct {
	base  int
	close int
	item  int
}

// writer builds the laid-out text one line at a time, which is what lets every rule below say
// "start a line here" without any of them knowing what came before.
type writer struct {
	out    strings.Builder
	line   strings.Builder
	indent int
}

func (w *writer) flush() {
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

func (w *writer) newline(indent int) {
	w.flush()
	w.indent = indent
}

// write appends a token to the line in force, putting a space in front of it unless the two
// tokens are ones nothing goes between.
func (w *writer) write(text string, spaced bool) {
	if w.line.Len() > 0 && spaced {
		w.line.WriteByte(' ')
	}
	w.line.WriteString(text)
}

func (w *writer) String() string {
	w.flush()
	return w.out.String()
}

// layout writes one statement. It walks the tokens once and decides three things about each: what
// case it is written in, whether a line starts before it, and whether a space goes in front of it.
// It takes no Dialect: every lexical difference between the engines was settled by Lex, and what
// is left — where a clause begins, what a keyword looks like — is the same SQL on both.
func layout(tokens []Token) string {
	match := matchParens(tokens)
	keyword := leadingWord(tokens)
	w := &writer{}

	stack := []frame{{}}
	item := 1
	// inline counts the parentheses that were *not* worth breaking. Inside one, nothing starts a
	// line: a function call and a value list read as the punctuation they are.
	inline := 0
	// breakBefore is a line the previous token asked for and the next token opens, so a clause
	// head with nothing after it leaves no empty line behind.
	breakBefore := -1
	previous := Token{Kind: TokenPunct, Text: ""}

	start := func(indent int) {
		if inline == 0 {
			breakBefore = indent
		}
	}
	// A clause is written as one phrase but is two tokens' worth of context: the first decides
	// whether a space goes in front of it and the last is what the *next* token is spaced against.
	put := func(text string, first, last Token) {
		if breakBefore >= 0 {
			w.newline(breakBefore)
			breakBefore = -1
		}
		w.write(text, spaced(previous, first))
		previous = last
	}
	write := func(text string, one Token) { put(text, one, one) }

	for i := 0; i < len(tokens); {
		one := tokens[i]
		here := stack[len(stack)-1]

		// A line comment runs to the end of its line by definition, so whatever follows it has to
		// start a new one or be swallowed. It keeps the line it is on when it is trailing.
		if one.Kind == TokenComment {
			write(one.Text, one)
			if !strings.HasPrefix(one.Text, "/*") {
				breakBefore = w.indent
			}
			i++
			continue
		}

		if n, phrase, ok := clauseAt(tokens, i, blockClauses); ok {
			start(here.base)
			put(phrase, one, tokens[i+n-1])
			item = here.base + 1
			start(item)
			i += n
			continue
		}
		if n, phrase, ok := clauseAt(tokens, i, inlineClauses); ok {
			start(here.base)
			put(phrase, one, tokens[i+n-1])
			item = here.base + 1
			i += n
			continue
		}
		if n, phrase, ok := clauseAt(tokens, i, itemClauses); ok {
			start(item)
			put(phrase, one, tokens[i+n-1])
			i += n
			continue
		}

		switch {
		case one.Kind == TokenPunct && one.Text == "(":
			write("(", one)
			if inline > 0 || !breaks(tokens, i, match[i], keyword) {
				inline++
				break
			}
			// A block parenthesis goes one level in from the line it opened on, and its `)`
			// comes back out to that line. Its contents start at that level whether they are a
			// list or a statement: a clause head inside indents its own items from there, so
			// `(SELECT …)` finds its second level without being told there is one.
			opened := frame{base: w.indent + 1, close: w.indent, item: item}
			stack = append(stack, opened)
			item = opened.base
			start(opened.base)

		case one.Kind == TokenPunct && one.Text == ")":
			if inline > 0 {
				inline--
				write(")", one)
				break
			}
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
				item = here.item
				start(here.close)
			}
			write(")", one)

		case one.Kind == TokenPunct && one.Text == ",":
			write(",", one)
			start(item)

		case one.Kind == TokenWord && keywords[strings.ToLower(one.Text)]:
			write(strings.ToUpper(one.Text), one)

		default:
			write(one.Text, one)
		}
		i++
	}

	return w.String()
}

// breaks says an opening parenthesis is a block rather than punctuation. Three things earn it,
// and an unmatched parenthesis earns none of them — there is no line to close it on.
//
//   - It holds a statement. `(SELECT …)` reads as one wherever it appears.
//   - It is the field list of a CREATE or an ALTER, which is the one list always worth a line
//     each however short it is: it is a table's definition and it is read down, not across.
//   - It holds more items than a line comfortably carries.
//
// A **function call** earns none of them: `COUNT(*)` and `numeric(10,2)` are punctuation whatever
// is inside them. What tells one from a list is the reader's own spacing, which is also what
// decides the space in front of it — see spaced.
func breaks(tokens []Token, open, close int, keyword string) bool {
	if close < 0 || !opened(tokens, open) {
		return false
	}

	if open+1 < close {
		next := tokens[open+1]
		if next.Kind == TokenWord && subqueryHeads[strings.ToLower(next.Text)] {
			return true
		}
	}

	if (keyword == "create" || keyword == "alter") && depthOf(tokens, open) == 0 {
		return true
	}
	return items(tokens, open, close) > breakItems
}

// opened reports whether the reader left a space in front of this parenthesis, which is the one
// honest way to tell `users (id, name)` from `count(*)`: both are a word followed by a list, and
// only the writer knows which is a call. Keeping their spacing is also what makes formatting
// idempotent, since the answer does not change when the text is written out again.
func opened(tokens []Token, open int) bool {
	if open == 0 {
		return true
	}
	previous := tokens[open-1]
	if previous.Kind == TokenWord && !keywords[strings.ToLower(previous.Text)] {
		return previous.End < tokens[open].Start
	}
	return previous.Kind != TokenIdent || previous.End < tokens[open].Start
}

// items counts what a parenthesis holds, by the commas directly inside it.
func items(tokens []Token, open, close int) int {
	count := 1
	depth := 0

	for i := open + 1; i < close; i++ {
		switch {
		case tokens[i].Kind != TokenPunct:
		case tokens[i].Text == "(":
			depth++
		case tokens[i].Text == ")":
			depth--
		case tokens[i].Text == "," && depth == 0:
			count++
		}
	}
	return count
}

func depthOf(tokens []Token, at int) int {
	depth := 0
	for i := 0; i < at; i++ {
		if tokens[i].Kind != TokenPunct {
			continue
		}
		switch tokens[i].Text {
		case "(":
			depth++
		case ")":
			depth--
		}
	}
	return depth
}

// matchParens pairs every parenthesis with its partner, answering -1 for one that has none. It is
// computed once because `breaks` asks about the width of a parenthesis before writing its first
// byte, which is a question that cannot be answered by walking forwards.
func matchParens(tokens []Token) []int {
	match := make([]int, len(tokens))
	for i := range match {
		match[i] = -1
	}

	open := []int{}
	for i, one := range tokens {
		if one.Kind != TokenPunct {
			continue
		}
		switch one.Text {
		case "(":
			open = append(open, i)
		case ")":
			if len(open) > 0 {
				last := open[len(open)-1]
				open = open[:len(open)-1]
				match[last], match[i] = i, last
			}
		}
	}
	return match
}

// leadingWord is the first word of a statement, which is what `breaks` reads a DDL off. It is the
// token list's answer to LeadingKeyword, and it is the same answer: comments and an opening
// parenthesis are stepped over, and the word after them is the statement's.
func leadingWord(tokens []Token) string {
	for _, one := range tokens {
		switch {
		case one.Kind == TokenComment:
		case one.Kind == TokenPunct && one.Text == "(":
		case one.Kind == TokenWord:
			return strings.ToLower(one.Text)
		default:
			return ""
		}
	}
	return ""
}

// noSpaceBefore and noSpaceAfter are the punctuation nothing goes between. `.` is in both,
// because `shop.users` is one name written in three tokens.
var (
	noSpaceBefore = words(", ; ) . :: ->> -> #>> #>")
	noSpaceAfter  = words("( . ::")
)

// spaced decides whether a space goes between two tokens. All of it is fixed by the punctuation
// except `(`, where the reader's own spacing is kept: `COUNT(*)` closes up and
// `INSERT INTO users (id, name)` does not, and no rule this side can tell a function call from a
// field list better than the person who wrote it.
func spaced(previous, next Token) bool {
	if previous.Text == "" {
		return false
	}
	if noSpaceBefore[next.Text] || noSpaceAfter[previous.Text] {
		return false
	}
	if next.Text == "(" {
		return previous.End < next.Start
	}
	return true
}
