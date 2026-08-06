package transfer

import (
	"bufio"
	"encoding/xml"
	"io"
	"strings"
	"unicode"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// nullAttribute is how a NULL survives a round trip through XML. An element left out is
// indistinguishable from a value nobody wrote and an empty element is indistinguishable from the
// empty string, so the one that is *neither* says so out loud.
const nullAttribute = "null"

// xmlReader reads the shape every tool that writes a table as XML writes: a root element holding one
// element per row, each holding one element per field. An attribute on the row is read as a field
// too, because half the tools that write this put the short values there.
//
// Where a field's own element holds elements rather than text, its text is what they hold, joined —
// which is the same bargain the JSON reader strikes with a nested object, and for the same reason:
// there is no column to flatten it into and dropping it would be worse.
type xmlReader struct {
	dec     *xml.Decoder
	file    io.Closer
	options Options
	columns []string
	at      map[string]int
	held    [][]*string
	next    int
	read    int
	done    bool
}

func openXML(decoded io.Reader, file io.Closer, options Options) (Reader, error) {
	r := &xmlReader{
		dec:     xml.NewDecoder(decoded),
		file:    file,
		options: options,
		at:      map[string]int{},
	}

	if _, err := r.element(); err != nil {
		return nil, err
	}

	for skipped := 0; skipped < options.Skip; skipped++ {
		if _, _, err := r.row(); err != nil {
			return nil, err
		}
	}

	for len(r.held) < sampleObjects && !r.done {
		names, values, err := r.row()
		if err != nil {
			return nil, err
		}
		if names == nil {
			break
		}
		r.widen(names)
		r.held = append(r.held, r.place(names, values))
	}

	for held := range r.held {
		r.held[held] = pad(r.held[held], len(r.columns))
	}
	return r, nil
}

func (r *xmlReader) Columns() []string { return r.columns }

func (r *xmlReader) Close() error { return r.file.Close() }

func (r *xmlReader) Next() ([]*string, error) {
	if r.options.Limit > 0 && r.read >= r.options.Limit {
		return nil, io.EOF
	}

	if r.next < len(r.held) {
		row := r.held[r.next]
		r.next++
		r.read++
		return row, nil
	}

	names, values, err := r.row()
	if err != nil {
		return nil, err
	}
	if names == nil {
		return nil, io.EOF
	}

	r.read++
	return pad(r.place(names, values), len(r.columns)), nil
}

func (r *xmlReader) widen(names []string) {
	for _, name := range names {
		if _, known := r.at[name]; known {
			continue
		}
		r.at[name] = len(r.columns)
		r.columns = append(r.columns, name)
	}
}

func (r *xmlReader) place(names []string, values []*string) []*string {
	row := make([]*string, len(r.columns))
	for at, name := range names {
		if column, known := r.at[name]; known {
			row[column] = values[at]
		}
	}
	return row
}

// element reads forward to the next start tag and answers with it. It is how the root is found
// without the file having to be told what the root is called.
func (r *xmlReader) element() (xml.StartElement, error) {
	for {
		token, err := r.dec.Token()
		if err == io.EOF {
			r.done = true
			return xml.StartElement{}, domain.ValidationError{
				Field:   "file",
				Message: "holds no elements",
			}
		}
		if err != nil {
			return xml.StartElement{}, badXML(err)
		}
		if start, ok := token.(xml.StartElement); ok {
			return start, nil
		}
	}
}

// row reads one row element and everything under it. A nil name list is the end of the file rather
// than a failure, which is what the root's own end tag means.
func (r *xmlReader) row() ([]string, []*string, error) {
	if r.done {
		return nil, nil, nil
	}

	for {
		token, err := r.dec.Token()
		if err == io.EOF {
			r.done = true
			return nil, nil, nil
		}
		if err != nil {
			return nil, nil, badXML(err)
		}

		switch shape := token.(type) {
		case xml.EndElement:
			// The root closing is the end of the rows; nothing else can close at this depth.
			r.done = true
			return nil, nil, nil
		case xml.StartElement:
			names, values, err := r.fields(shape)
			return names, values, err
		}
	}
}

func (r *xmlReader) fields(row xml.StartElement) ([]string, []*string, error) {
	names, values := []string{}, []*string{}

	for _, attribute := range row.Attr {
		if attribute.Name.Local == nullAttribute {
			continue
		}
		names = append(names, attribute.Name.Local)
		values = append(values, cell(attribute.Value, r.options))
	}

	for {
		token, err := r.dec.Token()
		if err != nil {
			return nil, nil, badXML(err)
		}

		switch shape := token.(type) {
		case xml.EndElement:
			return names, values, nil
		case xml.StartElement:
			value, err := r.field(shape)
			if err != nil {
				return nil, nil, err
			}
			names = append(names, shape.Name.Local)
			values = append(values, value)
		}
	}
}

// field is one field element's value: NULL where it says it is, and otherwise every piece of text
// under it joined — which for the ordinary case of `<name>Ada</name>` is `Ada`.
func (r *xmlReader) field(start xml.StartElement) (*string, error) {
	for _, attribute := range start.Attr {
		if attribute.Name.Local == nullAttribute && attribute.Value == "true" {
			return nil, r.dec.Skip()
		}
	}

	value := &strings.Builder{}
	depth := 0
	for {
		token, err := r.dec.Token()
		if err != nil {
			return nil, badXML(err)
		}

		switch shape := token.(type) {
		case xml.CharData:
			value.Write(shape)
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if depth == 0 {
				return cell(value.String(), r.options), nil
			}
			depth--
		}
	}
}

type xmlWriter struct {
	out     *bufio.Writer
	file    io.Closer
	options Options
	columns []string
	root    string
}

func newXMLWriter(encoded io.Writer, file io.Closer, options Options) Writer {
	return &xmlWriter{out: bufio.NewWriter(encoded), file: file, options: options}
}

func (w *xmlWriter) Begin(table string, columns []string) error {
	w.columns = columns
	w.root = xmlName(table, "table")

	charset := "UTF-8"
	if chosen, err := EncodingNamed(w.options.Encoding); err == nil {
		charset = chosen.Charset
	}

	_, err := w.out.WriteString(
		`<?xml version="1.0" encoding="` + charset + `"?>` + "\n<" + w.root + ">\n")
	return err
}

func (w *xmlWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		if _, err := w.out.WriteString("  <row>\n"); err != nil {
			return err
		}
		for at, name := range w.columns {
			if err := w.field(xmlName(name, "field"), value(row, at)); err != nil {
				return err
			}
		}
		if _, err := w.out.WriteString("  </row>\n"); err != nil {
			return err
		}
	}
	return nil
}

func (w *xmlWriter) field(name string, cell *string) error {
	if cell == nil {
		_, err := w.out.WriteString("    <" + name + ` ` + nullAttribute + `="true"/>` + "\n")
		return err
	}

	escaped := &strings.Builder{}
	if err := xml.EscapeText(escaped, []byte(*cell)); err != nil {
		return err
	}
	_, err := w.out.WriteString("    <" + name + ">" + escaped.String() + "</" + name + ">\n")
	return err
}

func (w *xmlWriter) Close() error {
	if _, err := w.out.WriteString("</" + w.root + ">\n"); err != nil {
		w.file.Close()
		return err
	}
	if err := w.out.Flush(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

// xmlName is a column or table name as an element name, which is a narrower alphabet than a column
// name is: a space, a dot or a leading digit are all legal in a database and none of them is legal
// here. Anything else becomes an underscore rather than being dropped, so two columns that differ
// only in punctuation do not become one element.
func xmlName(name, fallback string) string {
	out := &strings.Builder{}
	for at, char := range name {
		switch {
		case unicode.IsLetter(char) || char == '_':
			out.WriteRune(char)
		case at > 0 && (unicode.IsDigit(char) || char == '-' || char == '.'):
			out.WriteRune(char)
		default:
			out.WriteRune('_')
		}
	}

	if out.Len() == 0 {
		return fallback
	}
	return out.String()
}

func value(row []*string, at int) *string {
	if at >= len(row) {
		return nil
	}
	return row[at]
}

func badXML(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return domain.ValidationError{Field: "file", Message: "ends in the middle of its XML"}
	}
	return domain.FileError{Err: err}
}
