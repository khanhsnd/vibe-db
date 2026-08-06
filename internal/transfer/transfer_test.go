package transfer

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func str(value string) *string { return &value }

var (
	sampleColumns = []string{"id", "name", "note"}
	sampleRows    = [][]*string{
		{str("1"), str("Ada"), str("first")},
		{str("2"), str(`say "hi", it's fine`), nil},
		{str("3"), str("line\nbreak"), str("")},
	}
)

func write(t *testing.T, format string, options Options) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sample."+format)
	writer, err := Create(format, path, options)
	if err != nil {
		t.Fatalf("Create(%s): %v", format, err)
	}
	if err := writer.Begin("people", sampleColumns); err != nil {
		t.Fatalf("Begin(%s): %v", format, err)
	}
	// Written in two batches, because a streamed export arrives in batches and a writer that only
	// worked when handed everything at once would fail on the first table worth exporting.
	if err := writer.Rows(sampleRows[:1]); err != nil {
		t.Fatalf("Rows(%s): %v", format, err)
	}
	if err := writer.Rows(sampleRows[1:]); err != nil {
		t.Fatalf("Rows(%s): %v", format, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(%s): %v", format, err)
	}
	return path
}

func read(t *testing.T, format, path string, options Options) ([]string, [][]*string) {
	t.Helper()

	reader, err := Open(format, path, options)
	if err != nil {
		t.Fatalf("Open(%s): %v", format, err)
	}
	defer reader.Close()

	rows := [][]*string{}
	for {
		row, err := reader.Next()
		if err == io.EOF {
			return reader.Columns(), rows
		}
		if err != nil {
			t.Fatalf("Next(%s): %v", format, err)
		}
		rows = append(rows, row)
	}
}

// Every format that reads what it writes has to hand back what it was given, to the character. The
// one thing a format is allowed to lose is a NULL, and only where it has no way to say one — see
// `nullSurvives` for which ones those are.
func TestRoundTrip(t *testing.T) {
	for _, format := range []string{"csv", "txt", "json", "xml", "xlsx"} {
		t.Run(format, func(t *testing.T) {
			options := Options{HeaderRow: true, Qualifier: `"`}
			if format == "txt" {
				options.Delimiter = "\t"
			}

			columns, rows := read(t, format, write(t, format, options), options)

			if strings.Join(columns, ",") != strings.Join(sampleColumns, ",") {
				t.Fatalf("columns = %v, want %v", columns, sampleColumns)
			}
			if len(rows) != len(sampleRows) {
				t.Fatalf("read %d rows, want %d", len(rows), len(sampleRows))
			}

			for at, row := range rows {
				for column := range sampleColumns {
					got, want := row[column], sampleRows[at][column]
					if want == nil && !nullSurvives(format) {
						want = str("")
					}
					if !same(got, want) {
						t.Errorf("row %d %s = %s, want %s",
							at, sampleColumns[column], show(got), show(want))
					}
				}
			}
		})
	}
}

// A CSV and a tab-separated file have nothing to write a NULL as: an empty field is the only thing
// either of them can say "nothing" with, and it already means the empty string. Every other format
// here carries one — JSON has `null`, XML says so in an attribute, and a spreadsheet's empty cell is
// not the same as a cell holding no characters.
func nullSurvives(format string) bool {
	return format != "csv" && format != "txt"
}

func TestEmptyFieldReadsAsNullWhereTheReaderSaidSo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	if err := os.WriteFile(path, []byte("id,name\n1,\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	options := Options{HeaderRow: true, Qualifier: `"`, EmptyIsNull: true}
	_, rows := read(t, "csv", path, options)

	if len(rows) != 1 || rows[0][1] != nil {
		t.Errorf("an empty field did not read as NULL: %v", rows)
	}
}

// A file with no header row is still readable, and its fields are named for where they sit — which
// is what the mapping step of the wizard then shows the reader.
func TestAFileWithNoHeaderIsNamedByPosition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	if err := os.WriteFile(path, []byte("1,Ada\n2,Grace\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	columns, rows := read(t, "csv", path, Options{Qualifier: `"`})

	if strings.Join(columns, ",") != "field1,field2" {
		t.Errorf("columns = %v, want field1,field2", columns)
	}
	if len(rows) != 2 {
		t.Errorf("read %d rows, want 2 — the first is data, not a header", len(rows))
	}
}

func TestSkipAndLimitCutTheFileDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	body := "banner\nid,name\n1,Ada\n2,Grace\n3,Edsger\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	columns, rows := read(t, "csv", path, Options{
		HeaderRow: true,
		Qualifier: `"`,
		Skip:      1,
		Limit:     2,
	})

	if strings.Join(columns, ",") != "id,name" {
		t.Errorf("columns = %v, want id,name — the banner should have been skipped", columns)
	}
	if len(rows) != 2 {
		t.Errorf("read %d rows, want 2", len(rows))
	}
}

// A ragged row is squared up rather than refused: enough writers leave a trailing empty field out
// that refusing it would fail on files every other tool reads.
func TestAShortRowIsPaddedWithNull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	if err := os.WriteFile(path, []byte("id,name,note\n1,Ada\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, rows := read(t, "csv", path, Options{HeaderRow: true, Qualifier: `"`})

	if len(rows) != 1 || len(rows[0]) != 3 || rows[0][2] != nil {
		t.Errorf("a short row was not padded: %v", rows)
	}
}

// A fixed-width file has no delimiter to find, so the widths are what cuts it — and the padding
// each field was aligned with is not part of its value.
func TestFixedWidthIsCutByCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("1    Ada       \n22   Grace     \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, rows := read(t, "txt", path, Options{Widths: []int{5, 10}})

	if len(rows) != 2 || *rows[0][0] != "1" || *rows[0][1] != "Ada" || *rows[1][0] != "22" {
		t.Errorf("fixed width was not cut by count: %v", show(rows[0][0]))
	}
}

// A workbook is a zip of XML this package writes by hand, so the two things a spreadsheet does that
// no other format here does are worth proving: a number stays a number, and a date stored as a count
// of days comes back as a date rather than as that count.
func TestExcelWritesANumberAsANumber(t *testing.T) {
	path := write(t, "xlsx", Options{HeaderRow: true})

	if !strings.Contains(sheetXML(t, path), `<v>1</v>`) {
		t.Error("1 was not written as a number")
	}
	if !strings.Contains(sheetXML(t, path), `t="inlineStr"`) {
		t.Error("a text value was not written as a string")
	}
}

func TestExcelReadsASerialAsADate(t *testing.T) {
	cases := []struct {
		serial string
		want   string
	}{
		{"45678", "2025-01-21"},
		{"45678.5", "2025-01-21 12:00:00"},
		{"0.25", "06:00:00"},
	}

	for _, one := range cases {
		got, ok := serialDate(one.serial)
		if !ok || got != one.want {
			t.Errorf("serialDate(%s) = %q, want %q", one.serial, got, one.want)
		}
	}
}

// A name Excel will not open a workbook with is renamed here rather than producing a file that
// reports itself corrupt.
func TestSheetNameIsMadeSafe(t *testing.T) {
	if got := sheetName(`sales/2026`); got != "sales_2026" {
		t.Errorf("sheetName = %q, want sales_2026", got)
	}
	if got := sheetName(strings.Repeat("a", 40)); len(got) != sheetNameLimit {
		t.Errorf("sheetName kept %d characters, want %d", len(got), sheetNameLimit)
	}
}

func TestUnknownFormatIsRefused(t *testing.T) {
	if _, err := FormatNamed("mdb"); err == nil {
		t.Error("FormatNamed accepted a format this app does not have")
	}
	writer, err := Create("html", filepath.Join(t.TempDir(), "page.html"), Options{})
	if err != nil {
		t.Fatalf("HTML should be writable: %v", err)
	}
	writer.Close()

	if _, err := Open("html", filepath.Join(t.TempDir(), "page.html"), Options{}); err == nil {
		t.Error("HTML should not be importable")
	}
}

func sheetXML(t *testing.T, path string) string {
	t.Helper()

	reader, err := Open("xlsx", path, Options{HeaderRow: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	part, err := reader.(*workbookReader).open("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer part.Close()

	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func same(got, want *string) bool {
	if got == nil || want == nil {
		return got == want
	}
	return *got == *want
}

func show(value *string) string {
	if value == nil {
		return "NULL"
	}
	return `"` + *value + `"`
}
