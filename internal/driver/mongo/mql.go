package mongo

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// dialect is the lexical half of this engine. It is a `sqlkit.Dialect` for one reason and one
// reason only: so the shell is read by `sqlkit.Lex` rather than by a second scanner — see the
// dependency rule in docs/architecture.md. Nothing else in `sqlkit` is meant for a document store.
var dialect = sqlkit.Shell

// statement is one shell command, read. `db.users.find({ age: { $gt: 30 } }).limit(10)` is a
// collection, a method, its arguments, and what was then called on the cursor it returned.
type statement struct {
	// Collection is empty for a command addressed to the database itself — `db.runCommand(…)`,
	// `db.getCollectionNames()` — which is the one shape that has no collection to name.
	Collection string
	Method     string
	Args       []any
	// Chain is what was called on the method's own result, in the order it was written. It is a
	// list rather than named fields because the shell allows any order and only the interpreter
	// cares which calls it understands — see cursor.
	Chain []call
}

type call struct {
	Method string
	Args   []any
}

// Arg is the argument at this position, or nil where the command carried none. Every method here
// takes optional arguments, so asking past the end is ordinary rather than a fault.
func (s statement) Arg(at int) any {
	if at >= len(s.Args) {
		return nil
	}
	return s.Args[at]
}

// Filter is the first argument read as a document, which is what all but a handful of methods take
// it as. A command that named none filters nothing, exactly as `find()` does.
func (s statement) Filter() bson.D {
	document, _ := s.Arg(0).(bson.D)
	return document
}

// Filtered says the command narrowed itself to less than the whole collection, which is the fact
// `domain.MongoHazard` pairs with the method name. An absent filter and `{}` are the same thing:
// every document.
func (s statement) Filtered() bool {
	return len(s.Filter()) > 0
}

// parse reads one shell command. It is deliberately a *reader of one statement* rather than a
// script: cutting a script into its statements is `dialect.Split`'s, over the same tokens.
//
// What it accepts is the shell as a query language and not as a language: a command, its
// Extended-JSON-ish arguments, and the cursor methods that bound a read. There are no variables, no
// arithmetic and no control flow, because a client that half-implemented JavaScript would be a
// client whose answers could not be trusted — anything beyond this reaches the server verbatim
// through `db.runCommand`.
func parse(script string) (statement, error) {
	p := &parser{script: script, tokens: code(dialect.Lex(script))}

	one, err := p.statement()
	if err != nil {
		return statement{}, err
	}
	if p.more() {
		return statement{}, p.fail("expected the end of the command")
	}
	return one, nil
}

// parseValue reads one value out of the text a reader typed into a cell. It is the same grammar a
// command's arguments are read by, which is what makes the grid's own notation round-trip: a nested
// document drawn as `{ a: 1 }` is edited in that spelling and goes back as that document.
func parseValue(text string) (any, error) {
	p := &parser{script: text, tokens: code(dialect.Lex(text))}

	value, err := p.value()
	if err != nil {
		return nil, err
	}
	if p.more() {
		return nil, p.fail("expected the end of the value")
	}
	return value, nil
}

type parser struct {
	script string
	tokens []sqlkit.Token
	at     int
}

// code drops the comments. A comment is never part of a command anywhere in this grammar, so
// dropping it once here is what lets `db.users /* the live ones */ .find({})` read as the command it
// is — and what lets a command a reader wrote a note above run at all, since `Split` keeps the note
// in the chunk exactly as it does for SQL.
func code(tokens []sqlkit.Token) []sqlkit.Token {
	kept := make([]sqlkit.Token, 0, len(tokens))
	for _, one := range tokens {
		if one.Kind != sqlkit.TokenComment {
			kept = append(kept, one)
		}
	}
	return kept
}

// helpers are the shell's own constructors, and the one place a value that is not JSON gets in:
// `ObjectId("…")` is how a reader names a document, and a filter that spelled it as a string would
// silently match nothing. Each takes the arguments it takes and no others.
var helpers = map[string]bool{
	"objectid":      true,
	"isodate":       true,
	"date":          true,
	"numberlong":    true,
	"numberint":     true,
	"numberdecimal": true,
	"numberdouble":  true,
	"uuid":          true,
	"bindata":       true,
	"timestamp":     true,
	"minkey":        true,
	"maxkey":        true,
	"regexp":        true,
}

func (p *parser) statement() (statement, error) {
	if err := p.expectWord("db"); err != nil {
		return statement{}, err
	}
	if err := p.expect("."); err != nil {
		return statement{}, err
	}

	name, err := p.word("a collection or a command")
	if err != nil {
		return statement{}, err
	}

	// `db.getCollection("name")` is how a collection whose name is not a bare word is reached —
	// one holding a dash, or one called `stats`.
	if strings.EqualFold(name, "getCollection") {
		return p.afterCollection(nil)
	}

	// A word followed by `(` is a command on the database; a word followed by `.` is a collection
	// and the command comes next. That is the whole of the grammar's ambiguity.
	if p.at < len(p.tokens) && p.text() == "." {
		return p.afterCollection(&name)
	}
	return p.command("", name)
}

// afterCollection reads the collection name and then the command on it. A nil `named` means the
// name is still to be read from a `getCollection("…")` call.
func (p *parser) afterCollection(named *string) (statement, error) {
	collection := ""
	if named == nil {
		values, err := p.arguments()
		if err != nil {
			return statement{}, err
		}
		text, ok := oneString(values)
		if !ok {
			return statement{}, p.fail("getCollection takes the collection's name as a string")
		}
		collection = text
	} else {
		collection = *named
	}

	if err := p.expect("."); err != nil {
		return statement{}, err
	}
	method, err := p.word("a method")
	if err != nil {
		return statement{}, err
	}
	return p.command(collection, method)
}

// command reads a method's arguments and whatever was chained onto its result.
func (p *parser) command(collection, method string) (statement, error) {
	args, err := p.arguments()
	if err != nil {
		return statement{}, err
	}

	one := statement{Collection: collection, Method: method, Args: args}

	for p.at < len(p.tokens) && p.text() == "." {
		p.at++
		name, err := p.word("a cursor method")
		if err != nil {
			return statement{}, err
		}
		chained, err := p.arguments()
		if err != nil {
			return statement{}, err
		}
		one.Chain = append(one.Chain, call{Method: name, Args: chained})
	}

	return one, nil
}

// arguments reads a parenthesised, comma-separated list of values, which is what every call in
// this grammar takes — including the empty one.
func (p *parser) arguments() ([]any, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}

	values := []any{}
	for {
		if !p.more() {
			return nil, p.fail("expected a closing bracket")
		}
		if p.text() == ")" {
			p.at++
			return values, nil
		}

		value, err := p.value()
		if err != nil {
			return nil, err
		}
		values = append(values, value)

		if p.more() && p.text() == "," {
			p.at++
		}
	}
}

func (p *parser) value() (any, error) {
	if !p.more() {
		return nil, p.fail("expected a value")
	}

	one := p.tokens[p.at]
	switch {
	case one.Kind == sqlkit.TokenString:
		p.at++
		return unquote(one.Text), nil

	case one.Kind == sqlkit.TokenNumber:
		p.at++
		return number(one.Text, false)

	case one.Kind == sqlkit.TokenPunct && (one.Text == "-" || one.Text == "+"):
		p.at++
		if !p.more() || p.tokens[p.at].Kind != sqlkit.TokenNumber {
			return nil, p.fail("expected a number after the sign")
		}
		digits := p.tokens[p.at].Text
		p.at++
		return number(digits, one.Text == "-")

	case one.Kind == sqlkit.TokenPunct && one.Text == "{":
		return p.object()

	case one.Kind == sqlkit.TokenPunct && one.Text == "[":
		return p.array()

	case one.Kind == sqlkit.TokenPunct && one.Text == "/":
		return p.regex()

	case one.Kind == sqlkit.TokenWord:
		return p.namedValue()

	default:
		return nil, p.fail("expected a value")
	}
}

func (p *parser) object() (any, error) {
	p.at++ // {

	out := bson.D{}
	for {
		if !p.more() {
			return nil, p.fail("expected a closing brace")
		}
		if p.text() == "}" {
			p.at++
			return out, nil
		}

		name, err := p.fieldName()
		if err != nil {
			return nil, err
		}
		if err := p.expect(":"); err != nil {
			return nil, err
		}
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, bson.E{Key: name, Value: value})

		if p.more() && p.text() == "," {
			p.at++
		}
	}
}

// fieldName reads a key. The shell writes one bare where it can and quoted where it cannot, and a
// number is a key too — that is how an array's element is addressed by `"tags.0"`'s cousin.
func (p *parser) fieldName() (string, error) {
	if !p.more() {
		return "", p.fail("expected a field name")
	}

	one := p.tokens[p.at]
	switch one.Kind {
	case sqlkit.TokenString:
		p.at++
		return unquote(one.Text), nil
	case sqlkit.TokenWord, sqlkit.TokenNumber:
		p.at++
		// A dotted path is one key — `{"address.city": …}` written bare, which the shell allows.
		name := one.Text
		for p.more() && p.text() == "." {
			p.at++
			part, err := p.word("the rest of the field path")
			if err != nil {
				return "", err
			}
			name += "." + part
		}
		return name, nil
	default:
		return "", p.fail("expected a field name")
	}
}

func (p *parser) array() (any, error) {
	p.at++ // [

	out := bson.A{}
	for {
		if !p.more() {
			return nil, p.fail("expected a closing bracket")
		}
		if p.text() == "]" {
			p.at++
			return out, nil
		}

		value, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, value)

		if p.more() && p.text() == "," {
			p.at++
		}
	}
}

// regex reads a `/pattern/flags` literal off the *script* rather than off the tokens, because a
// pattern is not made of tokens: `/^a.*b$/i` is one value however the lexer cut it up. Nothing is
// lost by doing so — a regular expression cannot begin with `*`, so `/*` is always a comment and
// never the start of one of these.
func (p *parser) regex() (any, error) {
	open := p.tokens[p.at].Start

	for i := open + 1; i < len(p.script); i++ {
		switch p.script[i] {
		case '\\':
			i++
		case '\n', '\r':
			return nil, p.fail("this regular expression is never closed")
		case '/':
			pattern := p.script[open+1 : i]
			end := i + 1
			for end < len(p.script) && isFlag(p.script[end]) {
				end++
			}
			p.seek(end)
			return bson.Regex{Pattern: pattern, Options: p.script[i+1 : end]}, nil
		}
	}
	return nil, p.fail("this regular expression is never closed")
}

// namedValue reads the values that are spelled as words: the literals, and the shell's own
// constructors. Anything else is a name this grammar has no variables to look it up in.
func (p *parser) namedValue() (any, error) {
	one := p.tokens[p.at]
	p.at++

	switch strings.ToLower(one.Text) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	case "undefined":
		return bson.Undefined{}, nil
	// `new Date(…)` is the same value as `Date(…)`; the shell takes either.
	case "new":
		return p.value()
	}

	if !helpers[strings.ToLower(one.Text)] {
		return nil, p.failAt(one, "`"+one.Text+"` is not a value this side can read")
	}

	args, err := p.arguments()
	if err != nil {
		return nil, err
	}
	value, err := helper(one.Text, args)
	if err != nil {
		return nil, p.failAt(one, err.Error())
	}
	return value, nil
}

// helper builds the value one of the shell's constructors names. Each is strict about what it
// takes: a bad ObjectId is told about here rather than sent to the server as a string that will
// match nothing.
func helper(name string, args []any) (any, error) {
	switch strings.ToLower(name) {
	case "objectid":
		text, ok := oneString(args)
		if !ok {
			if len(args) == 0 {
				return bson.NewObjectID(), nil
			}
			return nil, statementError("ObjectId takes a 24-character hex string")
		}
		id, err := bson.ObjectIDFromHex(text)
		if err != nil {
			return nil, statementError("ObjectId takes a 24-character hex string")
		}
		return id, nil

	case "isodate", "date":
		return dateValue(args)

	case "numberlong":
		return integer(args, "NumberLong")

	case "numberint":
		value, err := integer(args, "NumberInt")
		if err != nil {
			return nil, err
		}
		return int32(value), nil

	case "numberdecimal":
		text, ok := oneString(args)
		if !ok {
			return nil, statementError("NumberDecimal takes its digits as a string")
		}
		return bson.ParseDecimal128(text)

	case "numberdouble":
		text, ok := oneString(args)
		if !ok {
			return nil, statementError("NumberDouble takes its digits as a string")
		}
		return strconv.ParseFloat(text, 64)

	case "uuid":
		text, ok := oneString(args)
		if !ok {
			return nil, statementError("UUID takes the uuid as a string")
		}
		data, err := uuidBytes(text)
		if err != nil {
			return nil, err
		}
		return bson.Binary{Subtype: 4, Data: data}, nil

	case "bindata":
		if len(args) != 2 {
			return nil, statementError("BinData takes a subtype and its base64 data")
		}
		subtype, err := integer(args[:1], "BinData")
		if err != nil {
			return nil, err
		}
		text, ok := args[1].(string)
		if !ok {
			return nil, statementError("BinData takes a subtype and its base64 data")
		}
		data, err := decodeBase64(text)
		if err != nil {
			return nil, err
		}
		return bson.Binary{Subtype: byte(subtype), Data: data}, nil

	case "timestamp":
		if len(args) != 2 {
			return nil, statementError("Timestamp takes a time and an ordinal")
		}
		seconds, err := integer(args[:1], "Timestamp")
		if err != nil {
			return nil, err
		}
		ordinal, err := integer(args[1:], "Timestamp")
		if err != nil {
			return nil, err
		}
		return bson.Timestamp{T: uint32(seconds), I: uint32(ordinal)}, nil

	case "minkey":
		return bson.MinKey{}, nil

	case "maxkey":
		return bson.MaxKey{}, nil

	case "regexp":
		if len(args) == 0 {
			return nil, statementError("RegExp takes a pattern and, optionally, its flags")
		}
		pattern, ok := args[0].(string)
		if !ok {
			return nil, statementError("RegExp takes a pattern and, optionally, its flags")
		}
		options := ""
		if len(args) > 1 {
			options, _ = args[1].(string)
		}
		return bson.Regex{Pattern: pattern, Options: options}, nil

	default:
		return nil, statementError(name + " is not a constructor this side knows")
	}
}

// dateValue reads what `ISODate` and `Date` take: nothing at all is now, a string is an instant,
// and a number is milliseconds since the epoch.
func dateValue(args []any) (any, error) {
	if len(args) == 0 {
		return bson.NewDateTimeFromTime(time.Now()), nil
	}
	if len(args) > 1 {
		return nil, statementError("a date takes one argument")
	}

	switch value := args[0].(type) {
	case string:
		return parseDate(value)
	case int32:
		return bson.DateTime(int64(value)), nil
	case int64:
		return bson.DateTime(value), nil
	case float64:
		return bson.DateTime(int64(value)), nil
	default:
		return nil, statementError("a date takes an instant or milliseconds since the epoch")
	}
}

// dateLayouts are the forms a reader writes an instant in, longest first. One naming no zone is
// read as UTC, which is the zone a date is stored and drawn in — see literal.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseDate reads an instant the way a reader writes one, and is also what a `date` field's cell
// is edited through: the grid drew it with `literal`, so the first layout here is the one that
// reads it straight back.
func parseDate(text string) (bson.DateTime, error) {
	for _, layout := range dateLayouts {
		if when, err := time.Parse(layout, text); err == nil {
			return bson.NewDateTimeFromTime(when), nil
		}
	}
	return 0, statementError(`a date takes an ISO 8601 instant, like "2024-01-31T09:00:00Z"`)
}

// hexBytes reads the hex a value that is not text was drawn as.
func hexBytes(digits string) ([]byte, error) {
	data, err := hex.DecodeString(digits)
	if err != nil {
		return nil, statementError("expected hex digits")
	}
	return data, nil
}

func integer(args []any, what string) (int64, error) {
	if len(args) != 1 {
		return 0, statementError(what + " takes one number")
	}

	switch value := args[0].(type) {
	case int32:
		return int64(value), nil
	case int64:
		return value, nil
	case float64:
		return int64(value), nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, statementError(what + " takes one number")
	}
}

func oneString(args []any) (string, bool) {
	if len(args) != 1 {
		return "", false
	}
	text, ok := args[0].(string)
	return text, ok
}

// number reads a numeric literal. A whole number that fits an int32 is one, which is what the
// grid then shows as a plain integer; anything wider is an int64 and anything with a fraction or
// an exponent is a double. Mongo compares numbers across all three, so the choice costs a filter
// nothing.
func number(text string, negative bool) (any, error) {
	if !strings.ContainsAny(text, ".eE") {
		whole, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			if negative {
				whole = -whole
			}
			if whole >= -2147483648 && whole <= 2147483647 {
				return int32(whole), nil
			}
			return whole, nil
		}
	}

	real, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, statementError(text + " is not a number")
	}
	if negative {
		real = -real
	}
	return real, nil
}

// unquote reads a string literal's own text: the quotes off, and the escapes a reader writes read
// back. It is not JSON's unescaping — `\u` is left alone, because a string that came out of the
// lexer whole is already the bytes the reader typed.
func unquote(text string) string {
	if len(text) < 2 {
		return text
	}
	body := text[1 : len(text)-1]
	if !strings.Contains(body, `\`) {
		return body
	}

	out := strings.Builder{}
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 >= len(body) {
			out.WriteByte(body[i])
			continue
		}
		i++
		switch body[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		default:
			out.WriteByte(body[i])
		}
	}
	return out.String()
}

func (p *parser) more() bool {
	return p.at < len(p.tokens)
}

func (p *parser) text() string {
	return p.tokens[p.at].Text
}

// seek moves past every token that begins before `offset`, which is how reading a value off the
// raw script — a regular expression — puts the token cursor back where it belongs.
func (p *parser) seek(offset int) {
	for p.at < len(p.tokens) && p.tokens[p.at].Start < offset {
		p.at++
	}
}

func (p *parser) expect(text string) error {
	if !p.more() {
		return p.fail("expected `" + text + "`")
	}
	if p.text() != text {
		return p.failAt(p.tokens[p.at], "expected `"+text+"`")
	}
	p.at++
	return nil
}

func (p *parser) expectWord(word string) error {
	if !p.more() || p.tokens[p.at].Kind != sqlkit.TokenWord ||
		!strings.EqualFold(p.text(), word) {
		return p.fail("a command begins with `" + word + "`")
	}
	p.at++
	return nil
}

func (p *parser) word(what string) (string, error) {
	if !p.more() || p.tokens[p.at].Kind != sqlkit.TokenWord {
		return "", p.fail("expected " + what)
	}
	text := p.text()
	p.at++
	return text, nil
}

// fail names where the reading stopped, since a message about a command says far less than one
// about the character the command went wrong at. A command that stopped at its own end is
// `unfinished` rather than wrong.
func (p *parser) fail(message string) error {
	if p.more() {
		return p.failAt(p.tokens[p.at], message)
	}
	return unfinished{statementError(message + ", but the command ends here")}
}

func (p *parser) failAt(one sqlkit.Token, message string) error {
	return statementError(message + ", at `" + one.Text + "`")
}

// statementError reports what the reader typed as the engine's own rejection would be reported:
// the wording is the only useful thing to show, and the code says it was proved here rather than
// by a server — nothing was sent to one.
func statementError(message string) error {
	return domain.StatementError{Code: "SHELL", Message: message}
}

// unfinished marks a command that ran *out* rather than one that went wrong: the reading reached the
// end of the text with something still to come. It is the difference between a command being typed
// and a command typed incorrectly, and it is the whole of what lets `Review` keep quiet about the
// first. Running one still fails, with the same wording it always had.
type unfinished struct{ error }

// Unwrap is what keeps the failure a `domain.StatementError` on its way to the UI, rather than a
// bare "statement failed" — see internal/app/errmap.go.
func (u unfinished) Unwrap() error { return u.error }

// stillTyping says a command was only unfinished, which is what every command looks like from here
// while it is being written.
func stillTyping(err error) bool {
	var out unfinished
	return errors.As(err, &out)
}

func isFlag(c byte) bool {
	return c == 'i' || c == 'm' || c == 's' || c == 'x' || c == 'u' || c == 'g'
}

// uuidBytes reads the sixteen bytes of a uuid, with or without the dashes a reader writes it with.
func uuidBytes(text string) ([]byte, error) {
	data, err := hexBytes(strings.ReplaceAll(text, "-", ""))
	if err != nil || len(data) != 16 {
		return nil, statementError("UUID takes 32 hex digits, dashes optional")
	}
	return data, nil
}

func decodeBase64(text string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, statementError("BinData takes its data as base64")
	}
	return data, nil
}
