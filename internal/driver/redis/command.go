package redis

import (
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// dialect is the lexical half of this engine, and it is a `sqlkit.Dialect` for one reason: so
// `sqlkit.Lex` reads a command rather than a second scanner being written. `#` opens a comment, both
// quotes open a string — there is no identifier quote to take one of them — and a backslash inside
// one escapes what follows it, which is what `redis-cli` does. Nothing else in `sqlkit` is meant for
// it: a key-value store has no clauses to lay out and no keywords to upper-case.
var dialect = sqlkit.Dialect{HashComments: true, BackslashEscapes: true}

// arguments cuts one command into the arguments the server will receive.
//
// **A Redis command is argv and not a sentence**, which is the whole difficulty: `user:1` is one
// argument and the lexer sees three tokens in it, because a colon is not a word character in any
// language `sqlkit.Lex` was written for. So an argument is a *run of adjacent tokens* — nothing
// between them, not even a space — and the run ends where whitespace does. That keeps the one
// scanner rule intact and costs a single comparison of offsets.
//
// A quoted token contributes what it holds rather than how it was written, so `SET greeting "hello
// world"` is three arguments and the third has a space in it.
func arguments(command string) ([]string, error) {
	tokens := dialect.Lex(command)
	// A command that came out of a file written by this app — a dump, an export — ends in the
	// terminator every statement in one ends with. It is not an argument, and it is dropped here
	// rather than in the splitter so that a command handed straight over runs as it reads.
	for len(tokens) > 0 && terminator(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}

	args := []string{}
	current := strings.Builder{}
	// An argument is *started* rather than measured, because `SET key ""` has one that is empty and
	// a builder with nothing in it cannot tell that from no argument at all.
	started := false
	end := -1

	for _, token := range tokens {
		if token.Kind == sqlkit.TokenComment {
			continue
		}
		if token.Unterminated {
			return nil, domain.StatementError{
				Code:    "SHELL",
				Message: "the command ends inside a quote",
			}
		}

		if token.Start != end && started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}

		text := token.Text
		if token.Kind == sqlkit.TokenString {
			text = unquote(token.Text)
		}
		current.WriteString(text)
		started = true
		end = token.End
	}

	if started {
		args = append(args, current.String())
	}
	return args, nil
}

// unquote reads what a quoted argument holds. The escapes are `redis-cli`'s own, which is the
// grammar a reader of this box already has in their fingers: `\n` is a newline, `\x41` is an `A`,
// and a backslash before anything else is that character — a rule that cannot fail, so nothing here
// answers an error.
func unquote(text string) string {
	if len(text) < 2 {
		return text
	}
	quote := text[0]
	body := text[1 : len(text)-1]

	var out strings.Builder
	for i := 0; i < len(body); i++ {
		character := body[i]

		// A doubled quote is one character, which is what the lexer already read it as.
		if character == quote && i+1 < len(body) && body[i+1] == quote {
			out.WriteByte(quote)
			i++
			continue
		}
		if character != '\\' || i+1 >= len(body) {
			out.WriteByte(character)
			continue
		}

		i++
		switch escaped := body[i]; escaped {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'b':
			out.WriteByte('\b')
		case 'a':
			out.WriteByte('\a')
		case 'x':
			if i+2 < len(body) {
				if value, err := strconv.ParseUint(body[i+1:i+3], 16, 8); err == nil {
					out.WriteByte(byte(value))
					i += 2
					continue
				}
			}
			out.WriteByte('x')
		default:
			out.WriteByte(escaped)
		}
	}
	return out.String()
}

// quoteArgument writes one argument the way a reader would have to type it, and is the other half of
// `arguments`: everything this adapter *spells* — a scan it shows, a row it exports as commands —
// goes through it, so what the reader is shown is a command they could paste back.
//
// A word with nothing special in it is written as itself, because `SET user:1 hello` is what a Redis
// reader expects to see and `"SET" "user:1" "hello"` is not.
func quoteArgument(text string) string {
	if text == "" {
		return `""`
	}

	plain := true
	for i := 0; i < len(text); i++ {
		if special(text[i]) {
			plain = false
			break
		}
	}
	if plain {
		return text
	}

	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(text); i++ {
		switch character := text[i]; character {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteByte(character)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if character < 0x20 || character == 0x7f {
				out.WriteString(`\x`)
				const hex = "0123456789abcdef"
				out.WriteByte(hex[character>>4])
				out.WriteByte(hex[character&0x0f])
				continue
			}
			out.WriteByte(character)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// special says a character would not survive being written bare: whitespace ends an argument, a
// quote starts one, a hash opens a comment, a backslash escapes whatever follows and a semicolon
// ends the command — see terminator.
func special(character byte) bool {
	switch character {
	case ' ', '\t', '\r', '\n', '"', '\'', '\\', '#', ';':
		return true
	}
	return character < 0x20 || character == 0x7f
}

// terminator is a bare `;`. Redis has no statement terminator — a newline ends a command — but a
// reader coming from any other tab will type one and every file this app writes ends its statements
// with one, so it is read as the punctuation it plainly is. A key that really holds a semicolon is
// written quoted, which is what `quoteArgument` does with one.
func terminator(token sqlkit.Token) bool {
	return token.Kind == sqlkit.TokenPunct && token.Text == ";"
}

// verb is what a command is answered for: its name, and the second word of a container command —
// `CONFIG SET` writes where `CONFIG GET` does not, so the pair is what `domain.RedisHazard` is asked
// about. Reading two words rather than one is the whole of what this knows about Redis's grammar.
func verb(args []string) (name, argument string) {
	if len(args) == 0 {
		return "", ""
	}
	if len(args) == 1 {
		return args[0], ""
	}
	return args[0], args[1]
}

// command spells arguments as the line a reader would type. It is what every statement this adapter
// answers with is built from — see `driver.Schema.RowStatements` — and it is the only way a command
// is ever written out.
func command(args ...string) string {
	spelled := make([]string, 0, len(args))
	for _, one := range args {
		spelled = append(spelled, quoteArgument(one))
	}
	return strings.Join(spelled, " ")
}
