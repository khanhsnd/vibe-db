package transfer

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// An .xlsx workbook is a zip of XML parts, and this is the whole of what one has to be to carry a
// table: a content-type map, two relationship files, a workbook naming one sheet, and the sheet.
// No dependency is taken for it, for the reason `internal/cloudauth` takes none — SpreadsheetML is
// a documented format, and the part of it a grid of cells uses is the part written here.
//
// **What is deliberately not written**: shared strings, styles, calculation chains, and a second
// sheet. A value is written inline, which costs bytes in a file that is compressed anyway and buys
// a writer that streams — a shared-string table has to be complete before the first row can be
// written, which for a table of a million rows is a table's worth of memory.
//
// **What is read is wider than what is written**, because a workbook to import is one Excel wrote:
// shared strings, inline strings, formula results, booleans, and a number that is really a date —
// see `serialDate`, without which every date in the file would import as a five-digit number.
const (
	sheetMain = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	// The 1900 date system's day zero. It is 30 December and not 31 because Excel believes 1900 was
	// a leap year, and the serial numbers in every file it has ever written are counted from a
	// calendar with that extra day in it.
	excelEpoch = "1899-12-30"

	// sheetNameLimit is Excel's own cap on what a tab may be called. A longer name is not refused
	// by the reader that opens it — it is refused by Excel, with a message about the file being
	// corrupt, which is the least useful thing it could say.
	sheetNameLimit = 31
)

type workbookReader struct {
	zip     *zip.ReadCloser
	rows    *xml.Decoder
	shared  []string
	dates   []bool
	options Options
	columns []string
	read    int
	pending []*string
	done    bool
}

func openWorkbook(path string, options Options) (Reader, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, domain.FileError{Err: err}
	}

	r := &workbookReader{zip: archive, options: options}
	// The zip is this reader's to close once it is handed over, and nobody else's until then — so
	// every way out of here that is not a Reader closes it.
	fail := func(err error) (Reader, error) {
		archive.Close()
		return nil, err
	}

	if err := r.load(options.Sheet); err != nil {
		return fail(err)
	}

	for skipped := 0; skipped < options.Skip; skipped++ {
		if _, err := r.record(); err != nil && err != io.EOF {
			return fail(err)
		}
	}

	first, err := r.record()
	if err != nil && err != io.EOF {
		return fail(err)
	}

	if options.HeaderRow {
		r.columns = headerNames(first)
		return r, nil
	}

	r.columns = positionalColumns(len(first))
	r.pending = first
	return r, nil
}

func (r *workbookReader) load(sheet string) error {
	part, err := r.sheetPart(sheet)
	if err != nil {
		return err
	}

	if r.shared, err = r.sharedStrings(); err != nil {
		return err
	}
	if r.dates, err = r.dateStyles(); err != nil {
		return err
	}

	rows, err := r.open(part)
	if err != nil {
		return err
	}
	r.rows = xml.NewDecoder(rows)
	return nil
}

func (r *workbookReader) Columns() []string { return r.columns }

func (r *workbookReader) Close() error { return r.zip.Close() }

func (r *workbookReader) Next() ([]*string, error) {
	if r.options.Limit > 0 && r.read >= r.options.Limit {
		return nil, io.EOF
	}

	row, err := r.record()
	if err != nil {
		return nil, err
	}

	r.read++
	return pad(row, len(r.columns)), nil
}

// record reads one `<row>` and answers with its cells, placed at the positions their references
// give. A row Excel wrote holds no cell at all for a column nobody typed in, so where a cell lands
// is read from its own `r="C7"` rather than from how many came before it.
func (r *workbookReader) record() ([]*string, error) {
	if r.pending != nil {
		row := r.pending
		r.pending = nil
		return row, nil
	}
	if r.done {
		return nil, io.EOF
	}

	for {
		token, err := r.rows.Token()
		if err == io.EOF {
			r.done = true
			return nil, io.EOF
		}
		if err != nil {
			return nil, badXML(err)
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		return r.cells()
	}
}

func (r *workbookReader) cells() ([]*string, error) {
	cells := []*string{}

	for {
		token, err := r.rows.Token()
		if err != nil {
			return nil, badXML(err)
		}

		switch shape := token.(type) {
		case xml.EndElement:
			if shape.Name.Local == "row" {
				return cells, nil
			}
		case xml.StartElement:
			if shape.Name.Local != "c" {
				continue
			}
			value, at, err := r.cell(shape)
			if err != nil {
				return nil, err
			}
			for len(cells) < at {
				cells = append(cells, nil)
			}
			cells = append(cells, value)
		}
	}
}

// cell reads one `<c>` and answers with its value and the column it belongs in. Which of the five
// shapes a value is in is the cell's own `t`, and the sixth — a number that a *style* says is a
// date — is why the style table was read at all.
func (r *workbookReader) cell(start xml.StartElement) (*string, int, error) {
	kind, reference, style := "", "", -1
	for _, attribute := range start.Attr {
		switch attribute.Name.Local {
		case "t":
			kind = attribute.Value
		case "r":
			reference = attribute.Value
		case "s":
			style, _ = strconv.Atoi(attribute.Value)
		}
	}

	raw, inline, err := r.cellText()
	if err != nil {
		return nil, 0, err
	}

	at := columnIndex(reference)
	if raw == "" && !inline {
		return nil, at, nil
	}

	switch kind {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 0 || index >= len(r.shared) {
			return nil, at, nil
		}
		return cell(r.shared[index], r.options), at, nil

	case "b":
		truth := "false"
		if strings.TrimSpace(raw) == "1" {
			truth = "true"
		}
		return &truth, at, nil

	case "inlineStr", "str", "e":
		return cell(raw, r.options), at, nil

	default:
		if style >= 0 && style < len(r.dates) && r.dates[style] {
			if formatted, ok := serialDate(raw); ok {
				return &formatted, at, nil
			}
		}
		return cell(raw, r.options), at, nil
	}
}

// cellText is everything under a `<c>` that carries its value: `<v>` for a number, a shared-string
// index or a formula's cached result, and `<is><t>` for an inline string. A `<f>` is skipped — the
// formula itself is not the value, and this app has no engine to evaluate one with.
func (r *workbookReader) cellText() (string, bool, error) {
	value := &strings.Builder{}
	inline, capture, depth := false, false, 0

	for {
		token, err := r.rows.Token()
		if err != nil {
			return "", false, badXML(err)
		}

		switch shape := token.(type) {
		case xml.CharData:
			if capture {
				value.Write(shape)
			}
		case xml.StartElement:
			depth++
			switch shape.Name.Local {
			case "v":
				capture = true
			case "t":
				capture, inline = true, true
			case "f":
				if err := r.rows.Skip(); err != nil {
					return "", false, badXML(err)
				}
				depth--
			}
		case xml.EndElement:
			if depth == 0 && shape.Name.Local == "c" {
				return value.String(), inline, nil
			}
			capture = false
			depth--
		}
	}
}

// sheetPart is the part inside the zip that holds the sheet asked for, found the way the format
// says to find it: the workbook names its sheets and points at each by relationship id, and the
// relationships turn that id into a path. Nothing here assumes `sheet1.xml`, because a workbook
// saved after a sheet was deleted does not have one.
func (r *workbookReader) sheetPart(name string) (string, error) {
	type sheet struct {
		Name string `xml:"name,attr"`
		ID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	}
	var workbook struct {
		Sheets []sheet `xml:"sheets>sheet"`
	}
	if err := r.decode("xl/workbook.xml", &workbook); err != nil {
		return "", err
	}
	if len(workbook.Sheets) == 0 {
		return "", domain.ValidationError{Field: "file", Message: "holds no worksheets"}
	}

	chosen := workbook.Sheets[0]
	if name != "" {
		found := false
		for _, one := range workbook.Sheets {
			if one.Name == name {
				chosen, found = one, true
				break
			}
		}
		if !found {
			return "", domain.ValidationError{Field: "sheet", Message: "is not in this workbook"}
		}
	}

	var rels struct {
		Relationships []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := r.decode("xl/_rels/workbook.xml.rels", &rels); err != nil {
		return "", err
	}

	for _, one := range rels.Relationships {
		if one.ID == chosen.ID {
			return "xl/" + strings.TrimPrefix(one.Target, "/xl/"), nil
		}
	}
	return "", domain.ValidationError{Field: "file", Message: "names a worksheet it does not hold"}
}

// SheetNames is every worksheet in a workbook, in the order its tabs read. It is what the wizard
// fills its Sheet picker from, and answers nothing at all for a file that is not a workbook.
func SheetNames(path string) []string {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil
	}
	defer archive.Close()

	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
		} `xml:"sheets>sheet"`
	}
	reader := &workbookReader{zip: archive}
	if err := reader.decode("xl/workbook.xml", &workbook); err != nil {
		return nil
	}

	names := make([]string, 0, len(workbook.Sheets))
	for _, one := range workbook.Sheets {
		names = append(names, one.Name)
	}
	return names
}

// sharedStrings is the table a cell of type `s` indexes into. It is the one part that has to be
// held whole — an index is meaningless without the table it indexes — and it is the reason this
// reader writes none of its own.
func (r *workbookReader) sharedStrings() ([]string, error) {
	var table struct {
		Items []struct {
			Runs []string `xml:"r>t"`
			Text string   `xml:"t"`
		} `xml:"si"`
	}
	if err := r.decode("xl/sharedStrings.xml", &table); err != nil {
		// A workbook holding no shared strings holds no such part at all, which is not a failure.
		return nil, nil
	}

	strs := make([]string, 0, len(table.Items))
	for _, item := range table.Items {
		if len(item.Runs) > 0 {
			strs = append(strs, strings.Join(item.Runs, ""))
			continue
		}
		strs = append(strs, item.Text)
	}
	return strs, nil
}

// dateStyles is, for each cell format the workbook declares, whether a number wearing it is a date.
// Excel stores every date as a number of days and says so nowhere but here — so without this part,
// every date in the file imports as a five-digit number.
func (r *workbookReader) dateStyles() ([]bool, error) {
	var styles struct {
		Formats []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Cells []struct {
			Format int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := r.decode("xl/styles.xml", &styles); err != nil {
		return nil, nil
	}

	custom := map[int]bool{}
	for _, format := range styles.Formats {
		custom[format.ID] = isDateCode(format.Code)
	}

	dates := make([]bool, 0, len(styles.Cells))
	for _, one := range styles.Cells {
		if known, ok := custom[one.Format]; ok {
			dates = append(dates, known)
			continue
		}
		dates = append(dates, builtinDateFormat(one.Format))
	}
	return dates, nil
}

func (r *workbookReader) decode(part string, into any) error {
	file, err := r.open(part)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := xml.NewDecoder(file).Decode(into); err != nil {
		return badXML(err)
	}
	return nil
}

func (r *workbookReader) open(part string) (io.ReadCloser, error) {
	for _, file := range r.zip.File {
		if file.Name == part {
			opened, err := file.Open()
			if err != nil {
				return nil, domain.FileError{Err: err}
			}
			return opened, nil
		}
	}
	return nil, domain.ValidationError{Field: "file", Message: "is not a workbook this can read"}
}

// builtinDateFormat is the part of the format table that is fixed by the specification rather than
// written into the file: 14 to 22 are the dates and times, and 45 to 47 the elapsed ones.
func builtinDateFormat(id int) bool {
	return (id >= 14 && id <= 22) || (id >= 45 && id <= 47)
}

// isDateCode reads a custom format code for the letters that make it one. Anything in quotes or in
// brackets is a literal or a locale rather than a field, so it is dropped before the letters are
// looked for — without which `0.00" months"` would make every number in the column a date.
func isDateCode(code string) bool {
	stripped := &strings.Builder{}
	skip := byte(0)

	for at := 0; at < len(code); at++ {
		char := code[at]
		switch {
		case skip != 0:
			if char == skip {
				skip = 0
			}
		case char == '"':
			skip = '"'
		case char == '[':
			skip = ']'
		default:
			stripped.WriteByte(char)
		}
	}

	return strings.ContainsAny(strings.ToLower(stripped.String()), "ydhs")
}

// serialDate turns Excel's own count of days into the text a date column takes. Which of the three
// shapes it is written in is what the number itself holds: no whole part is a time of day, no
// fraction is a date, and both is a timestamp.
func serialDate(raw string) (string, bool) {
	serial, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || serial < 0 || serial > 2958465 {
		return "", false
	}

	epoch, _ := time.Parse(time.DateOnly, excelEpoch)
	// Rounded to the second: a serial is a fraction of a day in binary floating point, so the
	// midnight of a whole day arrives as 23:59:59.9999 often enough to matter.
	moment := epoch.Add(time.Duration(math.Round(serial*24*60*60)) * time.Second)

	switch {
	case serial < 1:
		return moment.Format(time.TimeOnly), true
	case serial == math.Trunc(serial):
		return moment.Format(time.DateOnly), true
	default:
		return moment.Format(time.DateTime), true
	}
}

// columnIndex reads the column out of a cell reference: `C7` is the third column, and so is `C`.
// A reference the file left out answers zero, which is where a cell with no reference goes.
func columnIndex(reference string) int {
	index := 0
	for at := 0; at < len(reference); at++ {
		char := reference[at] | 0x20
		if char < 'a' || char > 'z' {
			break
		}
		index = index*26 + int(char-'a') + 1
	}
	return max(index-1, 0)
}

// columnLetters is columnIndex the other way about, which is what a cell being written needs.
func columnLetters(index int) string {
	letters := ""
	for index >= 0 {
		letters = string(rune('A'+index%26)) + letters
		index = index/26 - 1
	}
	return letters
}

// headerNames is a header row as column names. A blank header is named for its position rather than
// left empty, because a column with no name is one nothing can be mapped onto.
func headerNames(row []*string) []string {
	names := make([]string, len(row))
	for at, value := range row {
		names[at] = "field" + strconv.Itoa(at+1)
		if value != nil && strings.TrimSpace(*value) != "" {
			names[at] = strings.TrimSpace(*value)
		}
	}
	return names
}
