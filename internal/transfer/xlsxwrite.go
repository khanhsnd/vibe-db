package transfer

import (
	"archive/zip"
	"io"
	"strconv"
	"strings"
)

// The four parts a workbook needs besides its sheet. They are constant — one sheet, no styles, no
// shared strings — so they are written once at the head of the file and never referred to again.
const (
	contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`

	packageRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

	workbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`

	sheetHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="` + sheetMain + `"><sheetData>`

	sheetTail = `</sheetData></worksheet>`
)

// workbookWriter writes one table as one sheet of one workbook.
//
// **One file per table**, always — an export of six tables is six workbooks, not one with six tabs.
// A tabbed workbook would have to know every table before the first row is written, which is exactly
// what a streamed export does not know; and six files is what every other format here produces, so
// the wizard's answer to "where did it go" is the same whichever format was picked.
type workbookWriter struct {
	zip     *zip.Writer
	file    io.Closer
	sheet   io.Writer
	options Options
	columns []string
	at      int
}

func newWorkbookWriter(file io.WriteCloser, options Options) Writer {
	return &workbookWriter{zip: zip.NewWriter(file), file: file, options: options}
}

func (w *workbookWriter) Begin(table string, columns []string) error {
	w.columns = columns

	parts := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", packageRels},
		{"xl/workbook.xml", workbookXML(table)},
		{"xl/_rels/workbook.xml.rels", workbookRels},
	}
	for _, part := range parts {
		if err := w.part(part.name, part.body); err != nil {
			return err
		}
	}

	sheet, err := w.zip.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	w.sheet = sheet

	if _, err := io.WriteString(w.sheet, sheetHead); err != nil {
		return err
	}
	if !w.options.HeaderRow {
		return nil
	}

	names := make([]*string, len(columns))
	for at := range columns {
		names[at] = &columns[at]
	}
	return w.write(names)
}

func (w *workbookWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		if err := w.write(row); err != nil {
			return err
		}
	}
	return nil
}

func (w *workbookWriter) write(row []*string) error {
	w.at++

	line := &strings.Builder{}
	line.WriteString(`<row r="`)
	line.WriteString(strconv.Itoa(w.at))
	line.WriteString(`">`)
	for at := range w.columns {
		line.WriteString(sheetCell(columnLetters(at)+strconv.Itoa(w.at), value(row, at)))
	}
	line.WriteString("</row>")

	_, err := io.WriteString(w.sheet, line.String())
	return err
}

func (w *workbookWriter) part(name, body string) error {
	out, err := w.zip.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}

func (w *workbookWriter) Close() error {
	if _, err := io.WriteString(w.sheet, sheetTail); err != nil {
		w.file.Close()
		return err
	}
	if err := w.zip.Close(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

// sheetCell writes one cell. A NULL is written as no cell at all, which is what an empty cell in a
// spreadsheet *is* — and is what makes a NULL and an empty string tell apart on the way back in.
//
// A value that reads as a plain decimal number is written as one, so a column of amounts arrives in
// Excel as amounts and can be summed. Everything else is an inline string — including a number with
// a leading zero, one too long to survive a float, and anything in exponent notation, all of which
// are values a *reader* would recognise and a spreadsheet would quietly change.
func sheetCell(reference string, cell *string) string {
	if cell == nil {
		return ""
	}

	if numeric(*cell) {
		return `<c r="` + reference + `"><v>` + *cell + `</v></c>`
	}
	return `<c r="` + reference + `" t="inlineStr"><is><t xml:space="preserve">` +
		xmlText(*cell) + `</t></is></c>`
}

// numeric says a value is a decimal number that survives being one. The round trip is the whole
// test: it refuses a leading zero, a leading plus, exponent notation and anything with more
// significant digits than a float64 keeps — every one of which is a value that would come back out
// of Excel as something other than what went in.
func numeric(value string) bool {
	if value == "" || strings.ContainsAny(value, "eExXnNiI ") {
		return false
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	return strconv.FormatFloat(number, 'f', -1, 64) == value
}

// xmlText escapes a value for a sheet, and drops the control characters Excel refuses to open a
// file containing. A tab, a newline and a carriage return are the three it allows and the three a
// text column really holds; the rest cannot be represented in XML 1.0 at all, so a byte column that
// found its way into a text one cannot take the workbook down with it.
func xmlText(value string) string {
	out := &strings.Builder{}

	for _, char := range value {
		switch {
		case char == '&':
			out.WriteString("&amp;")
		case char == '<':
			out.WriteString("&lt;")
		case char == '>':
			out.WriteString("&gt;")
		case char == '\t' || char == '\n' || char == '\r':
			out.WriteRune(char)
		case char < 0x20 || char == 0xFFFE || char == 0xFFFF:
			// dropped
		default:
			out.WriteRune(char)
		}
	}
	return out.String()
}

func workbookXML(table string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="` + sheetMain + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="` + xmlText(sheetName(table)) + `" sheetId="1" r:id="rId1"/></sheets>
</workbook>`
}

// sheetName is a table's name as a tab may be called it. Excel refuses to open a workbook whose tab
// breaks any of these rules — and says the file is corrupt rather than saying which rule — so a
// table called `sales/2026` is renamed here instead of producing a file nobody can open.
func sheetName(table string) string {
	name := strings.Map(func(char rune) rune {
		if strings.ContainsRune(`[]:*?/\`, char) {
			return '_'
		}
		return char
	}, table)

	name = strings.Trim(name, "'")
	if runes := []rune(name); len(runes) > sheetNameLimit {
		name = string(runes[:sheetNameLimit])
	}
	if name == "" {
		return "Sheet1"
	}
	return name
}
