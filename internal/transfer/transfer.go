// Package transfer carries a table between a database and a file: the formats an import reads and
// an export writes, and nothing about either end of the journey.
//
// It knows no databases. A row here is `[]*string` — the shape a driver hands rows over in — and a
// `nil` is a NULL in every format that can tell one from an empty string. What a row *means* to a
// server is the driver's, and how it is quoted into a statement is `sqlkit.Dialect`'s; this package
// only ever sees text.
//
// **Both halves stream.** A Reader hands rows over as it reads them and a Writer takes them as they
// arrive, so an import of a million rows and an export of one both cost a batch of memory rather
// than a table's worth — which is the same bargain `internal/app/dump.go` already struck.
//
// **The readers here are not a second `sqlkit.Lex`.** That rule is about reading a *script*: where a
// statement ends is the engine's answer and there is one scanner for it. Nothing this package reads
// is ever a statement — a CSV field, a JSON value and a spreadsheet cell are data, and they reach a
// server only as values a driver has quoted.
package transfer

import (
	"io"
	"os"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Format is one file a table can travel as: what the reader picks in the wizard, what the file is
// called, and whether the journey can be made in that direction at all. HTML and Markdown are
// written and never read — both are laid out to be looked at, and reading one back would mean
// guessing which of its tables was data.
type Format struct {
	Name      string
	Label     string
	Extension string
	Reads     bool
	Writes    bool
}

// Formats is the whole list, in the order a wizard offers it. A name that is not here is refused
// rather than guessed at, exactly as an engine's is.
var Formats = []Format{
	{Name: "csv", Label: "CSV file", Extension: "csv", Reads: true, Writes: true},
	{Name: "txt", Label: "Text file", Extension: "txt", Reads: true, Writes: true},
	{Name: "json", Label: "JSON file", Extension: "json", Reads: true, Writes: true},
	{Name: "xml", Label: "XML file", Extension: "xml", Reads: true, Writes: true},
	{Name: "xlsx", Label: "Excel workbook", Extension: "xlsx", Reads: true, Writes: true},
	{Name: "html", Label: "HTML file", Extension: "html", Writes: true},
	{Name: "md", Label: "Markdown file", Extension: "md", Writes: true},
}

// Binary says the file's bytes are the format's own container rather than text, so an encoding has
// nothing to say about it: an Excel workbook is a zip of UTF-8 XML however the machine is set up.
func (f Format) Binary() bool { return f.Name == "xlsx" }

func FormatNamed(name string) (Format, error) {
	for _, format := range Formats {
		if format.Name == name {
			return format, nil
		}
	}
	return Format{}, domain.ValidationError{Field: "format", Message: "is not supported"}
}

// Options is everything about a file that is not its bytes. Most of it belongs to the delimited
// formats alone, which are the only ones that do not say inside the file where one field ends —
// which is exactly why a wizard asks about a CSV and asks nothing about a JSON.
type Options struct {
	// Encoding names how the bytes are read as text, and is ignored by a Binary format. Empty is
	// UTF-8.
	Encoding string
	// Delimiter separates one field from the next. Empty is a comma.
	Delimiter string
	// Qualifier wraps a field that holds the delimiter, a quote of its own or a line break. Empty
	// means nothing wraps one, and a field then ends at the first delimiter whatever it holds.
	Qualifier string
	// Widths cuts each line into fields by character count instead of by a delimiter, which is what
	// a fixed-width file has instead of one. Set, it is what is read and Delimiter goes unused.
	Widths []int
	// HeaderRow says the first row read holds the field names rather than data. Off, the fields are
	// named for their position — see `positionalColumns`.
	HeaderRow bool
	// Skip drops this many rows before anything at all is read, header included. It is how a file
	// with a banner over its table is imported.
	Skip int
	// Limit caps how many data rows are read; zero reads to the end of the file.
	Limit int
	// EmptyIsNull reads a field holding nothing as NULL rather than as the empty string. It is off
	// by default because the two are not the same value, and only the reader knows which their file
	// meant — see `Null` for the same question on the way out.
	EmptyIsNull bool
	// Null is what a NULL is written as by a format that cannot hold one: CSV, text, HTML and
	// Markdown. Empty writes an empty field, which is the only thing a CSV has to say "nothing"
	// with. JSON, XML and Excel ignore it — each has a null of its own.
	Null string
	// Sheet names the worksheet an Excel file is read from, and the one it is written to. Empty
	// reads the first sheet in the workbook and writes a sheet named after the table.
	Sheet string
}

func (o Options) delimiter() string {
	if o.Delimiter == "" {
		return ","
	}
	return o.Delimiter
}

// Reader hands a file's rows over one at a time, in the order the file holds them. Columns is
// answered before the first row, because an import maps the fields before it writes anything.
type Reader interface {
	Columns() []string
	// Next answers with the next row, and io.EOF once there are none left. A row shorter than the
	// column list is padded with NULL rather than refused: a trailing empty field is left out by
	// enough writers that refusing the row would fail on files every other tool reads.
	Next() ([]*string, error)
	Close() error
}

// Writer takes a table's rows as they are read out of a server. It is written to in the order the
// file reads back: the columns once, then batch after batch, then Close — which is where a format
// with a footer to write finally writes it, and so is checked like any other write.
type Writer interface {
	Begin(table string, columns []string) error
	Rows(rows [][]*string) error
	Close() error
}

// Open reads the head of a file and answers with something to read the rest of it from. The file is
// opened here rather than by the caller because one format needs more than a stream of bytes: a
// workbook is a zip, and a zip is read from the end.
func Open(name, path string, options Options) (Reader, error) {
	format, err := FormatNamed(name)
	if err != nil {
		return nil, err
	}
	if !format.Reads {
		return nil, domain.ValidationError{Field: "format", Message: "cannot be imported"}
	}

	if format.Name == "xlsx" {
		return openWorkbook(path, options)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, unreadable(err)
	}

	decoded, err := decodeReader(file, options.Encoding)
	if err != nil {
		file.Close()
		return nil, err
	}

	reader, err := openText(format.Name, decoded, file, options)
	if err != nil {
		file.Close()
		return nil, err
	}
	return reader, nil
}

func openText(name string, decoded io.Reader, file *os.File, options Options) (Reader, error) {
	switch name {
	case "json":
		return openJSON(decoded, file, options)
	case "xml":
		return openXML(decoded, file, options)
	default:
		return openDelimited(decoded, file, options)
	}
}

// Create makes the file and answers with something to write the table into it. The file is made
// before the first row is read out of the server, so a path this account cannot write to fails
// while nothing has been read — the same order `dump.go` opens its file in.
func Create(name, path string, options Options) (Writer, error) {
	format, err := FormatNamed(name)
	if err != nil {
		return nil, err
	}
	if !format.Writes {
		return nil, domain.ValidationError{Field: "format", Message: "cannot be exported"}
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, unreadable(err)
	}

	writer, err := newWriter(format, file, options)
	if err != nil {
		file.Close()
		return nil, err
	}
	return writer, nil
}

func newWriter(format Format, file *os.File, options Options) (Writer, error) {
	if format.Binary() {
		return newWorkbookWriter(file, options), nil
	}

	encoded, err := encodeWriter(file, options.Encoding)
	if err != nil {
		return nil, err
	}

	switch format.Name {
	case "json":
		return newJSONWriter(encoded, file, options), nil
	case "xml":
		return newXMLWriter(encoded, file, options), nil
	case "html":
		return newHTMLWriter(encoded, file, options), nil
	case "md":
		return newMarkdownWriter(encoded, file, options), nil
	default:
		return newDelimitedWriter(encoded, file, options), nil
	}
}

// text is the value of a cell as a format that has no null writes it.
func text(cell *string, options Options) string {
	if cell == nil {
		return options.Null
	}
	return *cell
}

// cell reads a field back the way the options asked: a field holding nothing is NULL where the
// reader said their file meant one by it, and the empty string otherwise.
func cell(value string, options Options) *string {
	if value == "" && options.EmptyIsNull {
		return nil
	}
	return &value
}

// positionalColumns names the fields of a file that carries no header row. They are one-based and
// spelled the way every other tool spells them, because a reader mapping `field3` onto a column is
// counting from the left of their own file.
func positionalColumns(count int) []string {
	names := make([]string, count)
	for at := range names {
		names[at] = "field" + strconv.Itoa(at+1)
	}
	return names
}

// pad squares a row up against the column list: a short row takes NULLs and a long one is cut. A
// file whose rows are ragged is a file every other tool reads, so refusing it here would only mean
// the reader has to go and fix a file that was never wrong.
func pad(row []*string, width int) []*string {
	for len(row) < width {
		row = append(row, nil)
	}
	return row[:width]
}

func unreadable(err error) error {
	return domain.FileError{Err: err}
}
