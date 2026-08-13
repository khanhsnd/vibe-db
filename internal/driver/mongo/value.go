package mongo

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// typeNames is what a BSON type is called where a reader ever has to name one: `$type` in a
// query, `to` in a `$convert`, `bsonType` in a schema. The driver's own `Type.String()` says
// "embedded document" and "objectID", which are neither of those spellings and cannot be typed
// back into anything, so the aliases are written out here instead.
var typeNames = map[bson.Type]string{
	bson.TypeDouble:           "double",
	bson.TypeString:           "string",
	bson.TypeEmbeddedDocument: "object",
	bson.TypeArray:            "array",
	bson.TypeBinary:           "binData",
	bson.TypeUndefined:        "undefined",
	bson.TypeObjectID:         "objectId",
	bson.TypeBoolean:          "bool",
	bson.TypeDateTime:         "date",
	bson.TypeNull:             "null",
	bson.TypeRegex:            "regex",
	bson.TypeDBPointer:        "dbPointer",
	bson.TypeJavaScript:       "javascript",
	bson.TypeSymbol:           "symbol",
	bson.TypeCodeWithScope:    "javascriptWithScope",
	bson.TypeInt32:            "int",
	bson.TypeTimestamp:        "timestamp",
	bson.TypeInt64:            "long",
	bson.TypeDecimal128:       "decimal",
	bson.TypeMinKey:           "minKey",
	bson.TypeMaxKey:           "maxKey",
}

func typeName(kind bson.Type) string {
	if name, ok := typeNames[kind]; ok {
		return name
	}
	return "unknown"
}

// cell is how one field of a document becomes the one thing the grid can draw: text.
//
// A scalar is written bare — an ObjectId is its hex, a date is an instant, a string is itself —
// because that is what a reader is looking at a column of. A **document or an array** is written
// in the shell's own notation instead, which is the same notation `Lex` reads back: what the grid
// shows can be pasted into the editor and mean what it showed. See literal.
//
// The absence of a value is nil, and so is an explicit null: `{a: null}` and a document with no
// `a` are the same question to a filter (`is NULL` finds both), so they read the same here.
func cell(value bson.RawValue) *string {
	switch value.Type {
	case 0, bson.TypeNull, bson.TypeUndefined:
		return nil

	case bson.TypeString:
		text := value.StringValue()
		return &text

	// Bytes that are not text become `\x…` hex, the same spelling `sqlkit` gives a `bytea`: a byte
	// sequence that is not text cannot survive the trip to the WebView as a JSON string, and the
	// value viewer already decodes that form into an image or a hex dump.
	//
	// **That spelling carries no subtype**, so it is only the answer for the plain one. A UUID is
	// `BinData(4, …)`, and a cell drawn without the 4 would be read back as the plain subtype and
	// address no document — which is what an `_id` stored as a UUID is. Anything with a subtype is
	// therefore written in the shell's own constructor, which `coerce` reads straight back.
	case bson.TypeBinary:
		subtype, data := value.Binary()
		if subtype != bson.TypeBinaryGeneric {
			text := literal(value)
			return &text
		}
		text := sqlkit.BinaryPrefix + hex.EncodeToString(data)
		return &text

	default:
		text := literal(value)
		return &text
	}
}

// literal writes a BSON value the way the shell spells it, which is the way this adapter's own
// parser reads it. It is what a statement carries and what an export writes, so a value that came
// out of a collection can go back into one — an `ObjectId` that arrived as a hex string and were
// written back as one would silently become a different field.
func literal(value bson.RawValue) string {
	switch value.Type {
	case 0, bson.TypeNull:
		return "null"
	case bson.TypeUndefined:
		return "undefined"

	case bson.TypeString:
		return quote(value.StringValue())

	case bson.TypeBoolean:
		return strconv.FormatBool(value.Boolean())

	case bson.TypeInt32:
		return strconv.FormatInt(int64(value.Int32()), 10)

	case bson.TypeInt64:
		return strconv.FormatInt(value.Int64(), 10)

	case bson.TypeDouble:
		return strconv.FormatFloat(value.Double(), 'g', -1, 64)

	case bson.TypeDecimal128:
		return "NumberDecimal(" + quote(value.Decimal128().String()) + ")"

	case bson.TypeObjectID:
		return "ObjectId(" + quote(value.ObjectID().Hex()) + ")"

	case bson.TypeDateTime:
		return "ISODate(" + quote(value.Time().UTC().Format(time.RFC3339Nano)) + ")"

	case bson.TypeTimestamp:
		seconds, ordinal := value.Timestamp()
		return "Timestamp(" + strconv.FormatUint(uint64(seconds), 10) + ", " +
			strconv.FormatUint(uint64(ordinal), 10) + ")"

	case bson.TypeRegex:
		pattern, options := value.Regex()
		return "/" + pattern + "/" + options

	// Inside a document, bytes are written as the constructor that reads them straight back —
	// `\x…` hex would come back as the *text* of a hex string. At the top of a cell they are the hex,
	// which is what the value viewer decodes; see cell.
	case bson.TypeBinary:
		subtype, data := value.Binary()
		return "BinData(" + strconv.Itoa(int(subtype)) + ", " +
			quote(base64.StdEncoding.EncodeToString(data)) + ")"

	case bson.TypeJavaScript:
		return quote(value.JavaScript())

	case bson.TypeSymbol:
		return quote(value.Symbol())

	case bson.TypeMinKey:
		return "MinKey()"

	case bson.TypeMaxKey:
		return "MaxKey()"

	case bson.TypeEmbeddedDocument:
		return document(value.Document())

	case bson.TypeArray:
		return array(bson.Raw(value.Array()))

	default:
		return quote(value.String())
	}
}

// Document renders a document in the shell's notation, and is exported because it is also how a
// statement the reader is shown spells its filter: what ran and what is displayed are the same
// bytes rendered once.
func Document(raw bson.Raw) string {
	return document(raw)
}

// valueText renders one value this side built. BSON has no top level but a document, so the value is
// wrapped, marshalled and read straight back out — which keeps `literal` the only renderer there is.
func valueText(value any) string {
	raw, err := bson.Marshal(bson.D{{Key: "v", Value: value}})
	if err != nil {
		return "null"
	}
	return literal(bson.Raw(raw).Lookup("v"))
}

// shellValue reads a cell's text back into the value it was drawn from: a number as a number, a
// document as a document, an ObjectId as one. Anything the grammar cannot read is the string it
// plainly is, which is the only honest answer where a grid has drawn `5` and cannot say whether the
// field held a number or the text of one.
func shellValue(text string) any {
	if value, err := parseValue(text); err == nil {
		return value
	}
	return text
}

func document(raw bson.Raw) string {
	elements, err := raw.Elements()
	if err != nil || len(elements) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(elements))
	for _, element := range elements {
		name, err := element.KeyErr()
		if err != nil {
			continue
		}
		parts = append(parts, key(name)+": "+literal(element.Value()))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func array(raw bson.Raw) string {
	values, err := raw.Values()
	if err != nil || len(values) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, literal(value))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// key writes a field name as the shell would: bare where it is a plain name or an operator, and
// quoted where it is anything else — a path's `.`, or a name with a space in it.
//
// `$` counts as bare because every operator starts with one and the shell writes them bare, and
// because no *field* may begin with one: `{ $gt: 30 }` is how a reader would type it and how they
// would read it back.
func key(name string) string {
	if name == "" {
		return `""`
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		bare := c == '_' || c == '$' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9' && i > 0)
		if !bare {
			return quote(name)
		}
	}
	return name
}

// quote writes a string literal. Double quotes throughout, so what is written is JSON as well as
// shell and can be pasted into either.
func quote(text string) string {
	out := strings.Builder{}
	out.WriteByte('"')

	for _, r := range text {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			out.WriteRune(r)
		}
	}

	out.WriteByte('"')
	return out.String()
}
