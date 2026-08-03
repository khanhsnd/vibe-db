package app

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportInput is the rows on screen and how to write them. They travel back from the frontend
// rather than being read again, because what a reader asked to export is what they can see —
// a replay could return something else.
type ExportInput struct {
	Namespace
	Format string `json:"format"`
	// Table names the INSERT statements of a SQL export and the file the dialog suggests. It is
	// empty for a result no single table can be traced to.
	Table   string      `json:"table"`
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}

// exportFormat is one way out, and the three of them are the whole list: what a reader picks in
// the menu, what the file is called, and what the dialog filters by.
type exportFormat struct {
	name      string
	label     string
	extension string
}

var exportFormats = []exportFormat{
	{name: "csv", label: "Comma-separated values", extension: "csv"},
	{name: "json", label: "JSON", extension: "json"},
	{name: "sql", label: "SQL INSERT statements", extension: "sql"},
}

// ExportRows writes the rows to a file the reader picks, and answers with the path it wrote.
// An empty path is a dismissed dialog rather than a failure, so nothing is reported for it.
func (a *ConnectionAPI) ExportRows(sessionID string, input ExportInput) (string, error) {
	format, err := formatNamed(input.Format)
	if err != nil {
		return "", wrapError(err)
	}

	body, err := a.render(sessionID, format, input)
	if err != nil {
		return "", wrapError(err)
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export rows",
		DefaultFilename: input.fileName(format),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: format.label, Pattern: "*." + format.extension},
		},
	})
	if err != nil {
		return "", wrapError(err)
	}
	if path == "" {
		return "", nil
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

func (a *ConnectionAPI) render(
	sessionID string,
	format exportFormat,
	input ExportInput,
) (string, error) {
	switch format.name {
	case "csv":
		return exportCSV(input)
	case "json":
		return exportJSON(input)
	}

	// Only SQL needs the session: how a name and a string are quoted is the driver's, so the
	// statements are asked for rather than spelled here.
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return "", err
	}

	statements := schema.InsertStatements(
		input.Namespace.toDriver(),
		input.tableName(),
		input.Columns,
		input.Rows,
	)
	if len(statements) == 0 {
		return "", nil
	}
	return strings.Join(statements, ";\n") + ";\n", nil
}

// exportCSV writes a NULL as an empty field, which is the only thing a CSV has to say "nothing"
// with. A reader who needs to tell the two apart wants JSON, where null is its own value.
func exportCSV(input ExportInput) (string, error) {
	out := &strings.Builder{}
	writer := csv.NewWriter(out)

	if err := writer.Write(input.Columns); err != nil {
		return "", err
	}

	record := make([]string, len(input.Columns))
	for _, row := range input.Rows {
		for at := range record {
			record[at] = ""
			if at < len(row) && row[at] != nil {
				record[at] = *row[at]
			}
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	return out.String(), writer.Error()
}

func exportJSON(input ExportInput) (string, error) {
	objects := make([]map[string]*string, 0, len(input.Rows))
	for _, row := range input.Rows {
		object := make(map[string]*string, len(input.Columns))
		for at, name := range input.Columns {
			if at < len(row) {
				object[name] = row[at]
			}
		}
		objects = append(objects, object)
	}

	encoded, err := json.MarshalIndent(objects, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

func formatNamed(name string) (exportFormat, error) {
	for _, format := range exportFormats {
		if format.name == name {
			return format, nil
		}
	}
	return exportFormat{}, domain.ValidationError{Field: "format", Message: "is not supported"}
}

// tableName stands in for a result no single table can be traced to, so a SQL export of a join
// is still valid SQL — with a target the reader has to correct, which is the honest outcome.
func (in ExportInput) tableName() string {
	if in.Table == "" {
		return "exported_rows"
	}
	return in.Table
}

func (in ExportInput) fileName(format exportFormat) string {
	name := in.Table
	if name == "" {
		name = "result"
	}
	return name + "." + format.extension
}
