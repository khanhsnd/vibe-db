package transfer

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// A delimited file is read and written here, and so is a fixed-width one: the two differ only in
// where a field ends, and everything above that — the header row, the rows skipped before it, the
// cap on how many are read — is the same question either way.
//
// **The record delimiter is detected rather than asked for.** LF, CRLF and CR all end a record, in
// one file if it comes to that, because a file written on one machine and imported on another is
// the normal case and a wrong answer to that question reads the whole file as a single row. What is
// *written* is CRLF, which is what RFC 4180 says a CSV holds.
const recordDelimiter = "\r\n"

type delimitedReader struct {
	in      *bufio.Reader
	file    io.Closer
	options Options
	comma   rune
	quote   rune
	columns []string
	read    int
	// pending is the one record `openDelimited` had to read to find out how wide a file with no
	// header row is, and then had no header to spend it on. There is never more than one — a reader
	// needing two would be a reader buffering the file.
	pending []string
}

func openDelimited(decoded io.Reader, file io.Closer, options Options) (Reader, error) {
	comma, err := oneRune(options.delimiter(), "delimiter")
	if err != nil {
		return nil, err
	}

	// No qualifier is a real answer — a fixed-width file has none, and neither does a file whose
	// fields are known to hold no delimiter — so an empty one is not a rune at all.
	quote := rune(-1)
	if options.Qualifier != "" {
		if quote, err = oneRune(options.Qualifier, "qualifier"); err != nil {
			return nil, err
		}
	}

	r := &delimitedReader{
		in:      bufio.NewReader(decoded),
		file:    file,
		options: options,
		comma:   comma,
		quote:   quote,
	}

	for at := 0; at < options.Skip; at++ {
		if _, err := r.record(); err != nil && err != io.EOF {
			return nil, err
		}
	}

	first, err := r.record()
	if err != nil && err != io.EOF {
		return nil, err
	}

	if options.HeaderRow {
		r.columns = trimAll(first)
		return r, nil
	}

	// A file with no header still has to say how wide it is, and the first record is the only thing
	// that can: it is named by position and then handed back, because it is data.
	r.columns = positionalColumns(len(first))
	r.unread(first)
	return r, nil
}

func (r *delimitedReader) Columns() []string { return r.columns }

func (r *delimitedReader) Close() error { return r.file.Close() }

func (r *delimitedReader) Next() ([]*string, error) {
	if r.options.Limit > 0 && r.read >= r.options.Limit {
		return nil, io.EOF
	}

	fields, err := r.record()
	if err != nil {
		return nil, err
	}

	r.read++
	row := make([]*string, len(fields))
	for at, field := range fields {
		row[at] = cell(field, r.options)
	}
	return pad(row, len(r.columns)), nil
}

func (r *delimitedReader) unread(fields []string) { r.pending = fields }

// record reads one row of the file. A fixed-width file is cut by count and a delimited one is
// scanned field by field, and both end at whichever of the three line endings comes first.
func (r *delimitedReader) record() ([]string, error) {
	if r.pending != nil {
		fields := r.pending
		r.pending = nil
		return fields, nil
	}

	if len(r.options.Widths) > 0 {
		return r.fixed()
	}

	// A blank line is nothing at all rather than a row of one empty field, which is what every other
	// reader of these files makes of it — and without which a file ending in a newline would import
	// one row too many.
	for {
		fields, err := r.delimited()
		if err != nil || len(fields) > 1 || fields[0] != "" {
			return fields, err
		}
	}
}

func (r *delimitedReader) fixed() ([]string, error) {
	line, err := r.line()
	if err != nil {
		return nil, err
	}

	runes := []rune(line)
	fields := make([]string, 0, len(r.options.Widths))
	at := 0
	for _, width := range r.options.Widths {
		start := min(at, len(runes))
		at += max(width, 0)
		fields = append(fields, strings.TrimRight(string(runes[start:min(at, len(runes))]), " "))
	}
	return fields, nil
}

// delimited scans one record field by field. A qualified field ends at the next qualifier that is
// not doubled — a doubled one is a qualifier the value holds — and may hold the delimiter and line
// breaks, which is the whole reason a file has qualifiers at all.
func (r *delimitedReader) delimited() ([]string, error) {
	fields := []string{}
	value := &strings.Builder{}

	for {
		char, _, err := r.in.ReadRune()
		if err == io.EOF {
			if len(fields) == 0 && value.Len() == 0 {
				return nil, io.EOF
			}
			return append(fields, value.String()), nil
		}
		if err != nil {
			return nil, unreadable(err)
		}

		switch {
		case char == r.quote && value.Len() == 0:
			quoted, err := r.quoted()
			if err != nil {
				return nil, err
			}
			value.WriteString(quoted)

		case char == r.comma:
			fields = append(fields, value.String())
			value.Reset()

		case char == '\n':
			return append(fields, value.String()), nil

		case char == '\r':
			// CRLF and a bare CR both end the record, so the LF of a CRLF is eaten here rather than
			// becoming the first character of the next one.
			if next, _, err := r.in.ReadRune(); err == nil && next != '\n' {
				r.in.UnreadRune()
			}
			return append(fields, value.String()), nil

		default:
			value.WriteRune(char)
		}
	}
}

// quoted reads the rest of a qualified field, the opening qualifier already consumed. A file that
// ends inside one is not an error: what was read is the field, which is the same thing every other
// reader of these files does with a truncated last line.
func (r *delimitedReader) quoted() (string, error) {
	value := &strings.Builder{}

	for {
		char, _, err := r.in.ReadRune()
		if err == io.EOF {
			return value.String(), nil
		}
		if err != nil {
			return "", unreadable(err)
		}

		if char != r.quote {
			value.WriteRune(char)
			continue
		}

		next, _, err := r.in.ReadRune()
		if err != nil {
			return value.String(), nil
		}
		if next != r.quote {
			r.in.UnreadRune()
			return value.String(), nil
		}
		value.WriteRune(r.quote)
	}
}

func (r *delimitedReader) line() (string, error) {
	line, err := r.in.ReadString('\n')
	if err == io.EOF && line == "" {
		return "", io.EOF
	}
	if err != nil && err != io.EOF {
		return "", unreadable(err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

type delimitedWriter struct {
	out     *bufio.Writer
	file    io.Closer
	options Options
	comma   string
	quote   string
}

func newDelimitedWriter(encoded io.Writer, file io.Closer, options Options) Writer {
	return &delimitedWriter{
		out:     bufio.NewWriter(encoded),
		file:    file,
		options: options,
		comma:   options.delimiter(),
		quote:   options.Qualifier,
	}
}

func (w *delimitedWriter) Begin(_ string, columns []string) error {
	if !w.options.HeaderRow {
		return nil
	}

	names := make([]*string, len(columns))
	for at := range columns {
		names[at] = &columns[at]
	}
	return w.write(names)
}

func (w *delimitedWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		if err := w.write(row); err != nil {
			return err
		}
	}
	return nil
}

func (w *delimitedWriter) write(row []*string) error {
	fields := make([]string, len(row))
	for at, value := range row {
		fields[at] = w.field(text(value, w.options))
	}

	_, err := w.out.WriteString(strings.Join(fields, w.comma) + recordDelimiter)
	return err
}

// field wraps a value only where it has to be: one that holds the delimiter, a qualifier of its own
// or a line break. Quoting everything would be simpler and is what makes a CSV unreadable by eye,
// which is half of what the format is for.
func (w *delimitedWriter) field(value string) string {
	if w.quote == "" || !strings.ContainsAny(value, w.comma+w.quote+"\r\n") {
		return value
	}
	return w.quote + strings.ReplaceAll(value, w.quote, w.quote+w.quote) + w.quote
}

func (w *delimitedWriter) Close() error {
	if err := w.out.Flush(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

func oneRune(value, what string) (rune, error) {
	if utf8.RuneCountInString(value) != 1 {
		return 0, domain.ValidationError{Field: what, Message: "must be a single character"}
	}
	first, _ := utf8.DecodeRuneInString(value)
	return first, nil
}

func trimAll(values []string) []string {
	trimmed := make([]string, len(values))
	for at, value := range values {
		trimmed[at] = strings.TrimSpace(value)
	}
	return trimmed
}
