package redis

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// text is the lexical half of this engine: where a command ends, how one is laid out, and what each
// command in a script is going to do. It is its own type rather than a `sqlkit.Dialect` because only
// the *scanning* is shared — there is no clause here to put on a line of its own and no keyword to
// upper-case beyond the first word, and the terminator is not a semicolon.
type text struct{}

// Split cuts a script into the commands it holds. **A newline ends a command**, which is the one
// place this grammar parts company with both the others: a Redis command is a line of arguments, so
// there is no terminator to leave out and a semicolon is an ordinary character a key may hold.
//
// A newline inside a quote ends nothing, which is why this is written over the tokens rather than
// over the text: `SET note "line one\nline two"` typed with a real newline in it is one command.
func (text) Split(script string) []string {
	commands := []string{}
	for _, one := range spans(script) {
		commands = append(commands, script[one.start:one.end])
	}
	return commands
}

// span is one command's place in the script, which is what a review points back at.
type span struct {
	start int
	end   int
}

// spans walks the tokens and cuts where a newline sits *between* two of them, and at a bare
// semicolon. A comment is dropped from the command it sits on but does not end it, the same way a
// trailing `-- note` does not end a SQL statement.
func spans(script string) []span {
	found := []span{}
	current := span{start: -1}

	for _, token := range dialect.Lex(script) {
		ended := current.start >= 0 &&
			strings.ContainsAny(script[current.end:token.Start], "\r\n")
		if ended || terminator(token) {
			if current.start >= 0 {
				found = append(found, current)
			}
			current = span{start: -1}
		}
		if token.Kind == sqlkit.TokenComment || terminator(token) {
			continue
		}
		if current.start < 0 {
			current.start = token.Start
		}
		current.end = token.End
	}

	if current.start >= 0 {
		found = append(found, current)
	}
	return found
}

// Review reads what a script is going to do without sending any of it anywhere.
//
// There is exactly one thing it can prove wrong, and it proves it: a command the script ends inside
// a quote of. Everything else about a Redis command is the server's to judge — whether `SET` was
// given the right number of arguments is a table this side would have to keep in step with every
// release, and a client that refused a command the server would have taken is worse than one that
// asks. What a command *does* is `domain.RedisHazard`'s rule and not this file's.
func (text) Review(script string) driver.Review {
	problems := []driver.Problem{}
	for _, token := range dialect.Lex(script) {
		if token.Unterminated {
			problems = append(problems, driver.Problem{
				Start:   token.Start,
				End:     token.End,
				Message: "this quote is never closed",
			})
		}
	}

	statements := []driver.ReviewedStatement{}
	for _, one := range spans(script) {
		line := script[one.start:one.end]
		reviewed := driver.ReviewedStatement{Start: one.start, End: one.end, SQL: line}

		if args, err := arguments(line); err == nil {
			name, argument := verb(args)
			reviewed.Keyword = strings.ToUpper(name)
			reviewed.Hazard = domain.RedisHazard(name, argument)
		}
		statements = append(statements, reviewed)
	}

	return driver.Review{Statements: statements, Problems: problems}
}

// Format lays a script out: one command to a line, the command's own name in upper case, and one
// space between arguments.
//
// **Nothing a server would notice changes.** An argument is written back exactly as it was typed —
// a quoted string byte for byte, its quotes included — because a key is bytes and this has no
// business deciding that `"user:1"` meant `user:1`. Only the name of the command and the whitespace
// between arguments move, which is the most a formatter can honestly do to argv.
func (text) Format(script string) string {
	tokens := dialect.Lex(script)
	for _, token := range tokens {
		if token.Unterminated {
			return script
		}
	}

	lines := []string{}
	for _, one := range spans(script) {
		if laid := layout(script, one); laid != "" {
			lines = append(lines, laid)
		}
	}
	if len(lines) == 0 {
		return strings.TrimSpace(script)
	}
	return strings.Join(lines, "\n")
}

// layout writes one command. It reads the same runs of adjacent tokens `arguments` does — an
// argument is what has no whitespace in it — and writes them back with the spacing normalised.
func layout(script string, one span) string {
	words := []string{}
	current := strings.Builder{}
	end := -1

	for _, token := range dialect.Lex(script[one.start:one.end]) {
		if token.Start != end && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
		current.WriteString(token.Text)
		end = token.End
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	if len(words) == 0 {
		return ""
	}

	// The name is upper-cased and a container command's second word with it, because `CONFIG SET` is
	// one verb in two words. Nothing after that is touched: an argument is data.
	words[0] = strings.ToUpper(words[0])
	if len(words) > 1 && container[strings.ToLower(words[0])] {
		words[1] = strings.ToUpper(words[1])
	}
	return strings.Join(words, " ")
}

// container is every command whose verb is two words. It is a list of *names* rather than of pairs,
// because it answers one question — is the second word part of the command — and the hazard of each
// pair is `domain`'s to keep.
var container = map[string]bool{
	"acl": true, "client": true, "cluster": true, "command": true, "config": true,
	"function": true, "latency": true, "memory": true, "object": true, "pubsub": true,
	"script": true, "slowlog": true, "xgroup": true, "xinfo": true,
}
