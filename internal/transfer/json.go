package transfer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// sampleObjects is how many objects at the head of a JSON file are read before the field list is
// settled. A JSON array declares no columns, so they are *inferred* — and inferring them from the
// first object alone would lose a field the second one carries and the first left out, which is the
// normal shape of exported JSON.
//
// It is a sample and behaves like one, exactly as `mongo.fields` does and for the same reason: a
// field that appears in none of them is not drawn until a read that returns it is, and the file is
// streamed from here on rather than held.
const sampleObjects = 100

type jsonReader struct {
	dec     *json.Decoder
	file    io.Closer
	options Options
	columns []string
	at      map[string]int
	// held is the head of the file, read to settle the column list and then handed over as the rows
	// it always was.
	held [][]*string
	next int
	read int
}

func openJSON(decoded io.Reader, file io.Closer, options Options) (Reader, error) {
	r := &jsonReader{
		dec:     json.NewDecoder(decoded),
		file:    file,
		options: options,
		at:      map[string]int{},
	}
	r.dec.UseNumber()

	// The file is an array of objects, which is what every tool that writes one writes. Anything
	// else is refused here rather than read as one row of nonsense.
	open, err := r.dec.Token()
	if err != nil {
		return nil, badJSON(err)
	}
	if delim, ok := open.(json.Delim); !ok || delim != '[' {
		return nil, domain.ValidationError{Field: "file", Message: "is not a JSON array of objects"}
	}

	for skipped := 0; skipped < options.Skip && r.dec.More(); skipped++ {
		var ignored json.RawMessage
		if err := r.dec.Decode(&ignored); err != nil {
			return nil, badJSON(err)
		}
	}

	for len(r.held) < sampleObjects && r.dec.More() {
		names, values, err := r.object()
		if err != nil {
			return nil, err
		}
		r.widen(names)
		r.held = append(r.held, r.place(names, values))
	}

	// A row read before the last field was seen is still one column short, so every held row is
	// squared up once the sample is over rather than as it was read.
	for held := range r.held {
		r.held[held] = pad(r.held[held], len(r.columns))
	}
	return r, nil
}

func (r *jsonReader) Columns() []string { return r.columns }

func (r *jsonReader) Close() error { return r.file.Close() }

func (r *jsonReader) Next() ([]*string, error) {
	if r.options.Limit > 0 && r.read >= r.options.Limit {
		return nil, io.EOF
	}

	if r.next < len(r.held) {
		row := r.held[r.next]
		r.next++
		r.read++
		return row, nil
	}

	if !r.dec.More() {
		return nil, io.EOF
	}

	names, values, err := r.object()
	if err != nil {
		return nil, err
	}

	r.read++
	return pad(r.place(names, values), len(r.columns)), nil
}

// widen adds the fields this object carries that no earlier one did, keeping the order they were
// first seen in — which is the order the file itself reads.
func (r *jsonReader) widen(names []string) {
	for _, name := range names {
		if _, known := r.at[name]; known {
			continue
		}
		r.at[name] = len(r.columns)
		r.columns = append(r.columns, name)
	}
}

// place puts an object's values under the columns they belong to by *name*, so an object that
// spells its fields in another order — or leaves one out — still lands in the right cells.
func (r *jsonReader) place(names []string, values []*string) []*string {
	row := make([]*string, len(r.columns))
	for at, name := range names {
		if column, known := r.at[name]; known {
			row[column] = values[at]
		}
	}
	return row
}

// object reads one object of the array as the fields it holds, in the order it holds them. A nested
// object or array is kept as the JSON it is written as, which is exactly how the grid draws one:
// there is no column to flatten it into and losing it would be worse than carrying it as text.
func (r *jsonReader) object() ([]string, []*string, error) {
	var raw json.RawMessage
	if err := r.dec.Decode(&raw); err != nil {
		return nil, nil, badJSON(err)
	}

	inner := json.NewDecoder(bytes.NewReader(raw))
	inner.UseNumber()

	open, err := inner.Token()
	if err != nil {
		return nil, nil, badJSON(err)
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		return nil, nil, domain.ValidationError{
			Field:   "file",
			Message: "holds something other than objects",
		}
	}

	names, values := []string{}, []*string{}
	for inner.More() {
		key, err := inner.Token()
		if err != nil {
			return nil, nil, badJSON(err)
		}

		var value json.RawMessage
		if err := inner.Decode(&value); err != nil {
			return nil, nil, badJSON(err)
		}

		name, _ := key.(string)
		names = append(names, name)
		values = append(values, jsonCell(value, r.options))
	}
	return names, values, nil
}

// jsonCell is one JSON value as the text a column takes. `null` is the NULL it plainly is; a string
// is its own contents rather than the quoted form, so `"36"` reaches a numeric column as `36`; and
// everything else — a number, a boolean, an object, an array — is the JSON verbatim.
func jsonCell(raw json.RawMessage, options Options) *string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return nil
	}

	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return cell(value, options)
		}
	}
	return &trimmed
}

// jsonWriter writes the array `openJSON` reads: one object per row, its fields in the column order
// the table has them in.
//
// **Every value is written as a string**, `null` excepted. The rows arrive as text — that is what a
// driver hands over and what the grid drew — so writing `1` rather than `"1"` would be this file
// guessing at a type it was never told, and guessing wrong the first time a zero-padded account
// number went past. It is the same choice the grid's own `Export as JSON` makes.
type jsonWriter struct {
	out     *bufio.Writer
	file    io.Closer
	options Options
	columns []string
	written bool
}

func newJSONWriter(encoded io.Writer, file io.Closer, options Options) Writer {
	return &jsonWriter{out: bufio.NewWriter(encoded), file: file, options: options}
}

func (w *jsonWriter) Begin(_ string, columns []string) error {
	w.columns = columns
	_, err := w.out.WriteString("[")
	return err
}

func (w *jsonWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		if w.written {
			if _, err := w.out.WriteString(","); err != nil {
				return err
			}
		}
		w.written = true

		if _, err := w.out.WriteString("\n  {"); err != nil {
			return err
		}
		if err := w.object(row); err != nil {
			return err
		}
		if _, err := w.out.WriteString("}"); err != nil {
			return err
		}
	}
	return nil
}

func (w *jsonWriter) object(row []*string) error {
	for at, name := range w.columns {
		separator := ", "
		if at == 0 {
			separator = ""
		}

		value := "null"
		if at < len(row) && row[at] != nil {
			value = jsonString(*row[at])
		}
		if _, err := w.out.WriteString(separator + jsonString(name) + ": " + value); err != nil {
			return err
		}
	}
	return nil
}

func (w *jsonWriter) Close() error {
	tail := "\n]\n"
	if !w.written {
		tail = "]\n"
	}
	if _, err := w.out.WriteString(tail); err != nil {
		w.file.Close()
		return err
	}
	if err := w.out.Flush(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

// jsonString quotes a value the way the encoder does, so a quote, a backslash and a control
// character in a cell come back out as the value that went in.
func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func badJSON(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return domain.ValidationError{Field: "file", Message: "ends in the middle of its JSON"}
	}
	return domain.FileError{Err: err}
}
