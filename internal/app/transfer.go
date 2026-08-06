package app

import (
	"github.com/khanhsnd/vibe-db/internal/transfer"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The wire types the two wizards share, and the two host dialogs they raise.
//
// An import and an export are the same journey in opposite directions, so what a *file* is — its
// format, its encoding, where one field ends — is one shape asked once. What differs is at the
// database end, and that is what `import.go` and `export_job.go` hold.

// TransferFormat is one file a table can travel as, as the wizard lists it. It mirrors
// `transfer.Format` exactly, for the reason every other list is mirrored: the frontend draws the
// list and the Go side is what decides it, so a format only has to be added in one place.
type TransferFormat struct {
	Value     string `json:"value"`
	Label     string `json:"label"`
	Extension string `json:"extension"`
	// Delimited says this format has no way of its own to say where one field ends, so the wizard
	// asks — and asks nothing about any other format.
	Delimited bool `json:"delimited"`
	// Text says an encoding applies. A workbook is a zip of UTF-8 XML however the machine is set up.
	Text   bool `json:"text"`
	Reads  bool `json:"reads"`
	Writes bool `json:"writes"`
}

// TransferEncoding is one way of reading a file's bytes as text, mirrored for the same reason.
type TransferEncoding struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TransferOptions is everything about a file that is not its bytes, as the wizard fills it in.
// Most of it belongs to the delimited formats alone — which is why the wizard asks about a CSV and
// asks nothing at all about a JSON.
type TransferOptions struct {
	Encoding  string `json:"encoding"`
	Delimiter string `json:"delimiter"`
	Qualifier string `json:"qualifier"`
	// Widths cuts a fixed-width file by character count instead of by a delimiter. Set, it is what
	// is read and Delimiter goes unused.
	Widths []int `json:"widths"`
	// HeaderRow says the first row read holds the field names rather than data — and, on the way
	// out, that they are written.
	HeaderRow bool `json:"headerRow"`
	// Skip drops this many rows before anything is read, header included: it is how a file with a
	// banner over its table is imported.
	Skip int `json:"skip"`
	// Limit caps how many data rows are read; zero reads the file to its end.
	Limit int `json:"limit"`
	// EmptyIsNull reads a field holding nothing as NULL rather than as the empty string.
	EmptyIsNull bool `json:"emptyIsNull"`
	// Null is what a NULL is written as by a format that has none of its own.
	Null string `json:"null"`
	// Sheet names the worksheet a workbook is read from, and is empty for its first.
	Sheet string `json:"sheet"`
}

func (o TransferOptions) toTransfer() transfer.Options {
	return transfer.Options{
		Encoding:    o.Encoding,
		Delimiter:   o.Delimiter,
		Qualifier:   o.Qualifier,
		Widths:      o.Widths,
		HeaderRow:   o.HeaderRow,
		Skip:        max(o.Skip, 0),
		Limit:       max(o.Limit, 0),
		EmptyIsNull: o.EmptyIsNull,
		Null:        o.Null,
		Sheet:       o.Sheet,
	}
}

// TransferMessage is one line of a wizard's log: what a step did, and whether it went wrong. It is
// a transcript rather than a grid for the reason `ExecuteFileDialog`'s is — a statement laid out
// over five lines and a paragraph the server wrote are not rows and columns.
type TransferMessage struct {
	Text  string `json:"text"`
	Error bool   `json:"error"`
}

// ListTransferFormats answers the formats a wizard offers, in the order it lists them. Both
// directions are in the one list — `reads` and `writes` are what narrow it — because one list
// is what keeps a format from being added in two places and forgotten in one.
func (*ConnectionAPI) ListTransferFormats() []TransferFormat {
	formats := make([]TransferFormat, 0, len(transfer.Formats))
	for _, format := range transfer.Formats {
		formats = append(formats, TransferFormat{
			Value:     format.Name,
			Label:     format.Label,
			Extension: format.Extension,
			Delimited: format.Name == "csv" || format.Name == "txt",
			Text:      !format.Binary(),
			Reads:     format.Reads,
			Writes:    format.Writes,
		})
	}
	return formats
}

func (*ConnectionAPI) ListTransferEncodings() []TransferEncoding {
	encodings := make([]TransferEncoding, 0, len(transfer.Encodings))
	for _, one := range transfer.Encodings {
		encodings = append(encodings, TransferEncoding{Value: one.Name, Label: one.Label})
	}
	return encodings
}

// ChooseTransferFile raises the host's own file picker, filtered to the format the wizard is on. An
// empty path is a dismissed dialog rather than a failure, exactly as ChooseScriptFile reads.
func (a *ConnectionAPI) ChooseTransferFile(name string) (string, error) {
	format, err := transfer.FormatNamed(name)
	if err != nil {
		return "", wrapError(err)
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Import from " + format.Label,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: format.Label, Pattern: "*." + format.Extension},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// ChooseTransferFolder raises the host's own folder picker, which is what an export of several
// tables needs: one file per table, and the reader picks where they land rather than naming each.
func (a *ConnectionAPI) ChooseTransferFolder() (string, error) {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Export to folder",
	})
	if err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// ListWorksheets answers the sheets a workbook holds, so the wizard can offer them rather than make
// the reader type one. It returns nothing for a file that is not a workbook, and no error either:
// a file that cannot be opened is answered by the read that follows, in the step that is about it.
func (*ConnectionAPI) ListWorksheets(path string) []string {
	names := transfer.SheetNames(path)
	if names == nil {
		return []string{}
	}
	return names
}

// transferMessage is the shape every log line in both wizards is built with.
func transferMessage(text string) TransferMessage {
	return TransferMessage{Text: text}
}

// transferFailure is a failure as one line of a log. The wording is what the *wire* would carry —
// the server's own for a rejected statement — because it is the same failure the reader would have
// been shown had it been thrown instead of logged, and it should not read differently for having
// been one of a thousand.
func transferFailure(text string, err error) TransferMessage {
	_, message := wireFields(err)
	return TransferMessage{Text: text + ": " + message, Error: true}
}
