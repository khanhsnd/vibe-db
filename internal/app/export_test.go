package app

import (
	"strings"
	"testing"
)

func text(value string) *string { return &value }

func sampleExport() ExportInput {
	return ExportInput{
		Table:   "users",
		Columns: []string{"id", "name"},
		Rows: [][]*string{
			{text("1"), text("Ada")},
			{text("2"), nil},
			{text("3"), text(`say "hi", it's fine`)},
		},
	}
}

func TestExportCSVWritesANullAsAnEmptyField(t *testing.T) {
	got, err := exportCSV(sampleExport())
	if err != nil {
		t.Fatalf("exportCSV: %v", err)
	}

	want := "id,name\n" +
		"1,Ada\n" +
		"2,\n" +
		"3,\"say \"\"hi\"\", it's fine\"\n"

	if got != want {
		t.Errorf("exportCSV =\n%q\nwant\n%q", got, want)
	}
}

// JSON is the format that can tell a NULL from an empty string, which is why the CSV writer
// is allowed to lose the difference.
func TestExportJSONKeepsNullAsNull(t *testing.T) {
	got, err := exportJSON(sampleExport())
	if err != nil {
		t.Fatalf("exportJSON: %v", err)
	}

	if !strings.Contains(got, `"name": null`) {
		t.Errorf("a null did not survive:\n%s", got)
	}
	if !strings.Contains(got, `"name": "Ada"`) {
		t.Errorf("a value did not survive:\n%s", got)
	}
}

func TestExportRejectsAFormatItDoesNotHave(t *testing.T) {
	if _, err := formatNamed("xlsx"); err == nil {
		t.Error("formatNamed accepted a format that is not offered")
	}
}

// A result no single table can be traced to still exports, with a target the reader corrects.
func TestExportNamesAResultWithNoTable(t *testing.T) {
	blank := ExportInput{}

	if got := blank.tableName(); got != "exported_rows" {
		t.Errorf("tableName = %s, want exported_rows", got)
	}
	if got := blank.fileName(exportFormats[0]); got != "result.csv" {
		t.Errorf("fileName = %s, want result.csv", got)
	}
}
