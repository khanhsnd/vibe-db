package transfer

import (
	"bufio"
	"html"
	"io"
	"strings"
)

// The two formats a table is written *to be looked at* rather than to be read back: a web page and
// a Markdown table. Neither is offered on the way in — see `Format.Reads` — because reading one back
// would mean guessing which of a document's tables was the data, and a document that was laid out
// for a person is exactly the one where that guess goes wrong.
//
// A NULL is `Options.Null` in both, for the reason a CSV's is: neither format has a null of its own,
// and an empty cell is a value the table could genuinely hold.

type documentWriter struct {
	out     *bufio.Writer
	file    io.Closer
	options Options
	columns []string
	// cell writes one value, escaped for this format.
	cell func(string) string
	// tail is the document's closing, written by Close.
	tail string
}

func (w *documentWriter) Close() error {
	if _, err := w.out.WriteString(w.tail); err != nil {
		w.file.Close()
		return err
	}
	if err := w.out.Flush(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

type htmlWriter struct{ documentWriter }

func newHTMLWriter(encoded io.Writer, file io.Closer, options Options) Writer {
	return &htmlWriter{documentWriter{
		out:     bufio.NewWriter(encoded),
		file:    file,
		options: options,
		cell:    html.EscapeString,
		tail:    "</tbody>\n</table>\n</body>\n</html>\n",
	}}
}

// The style is inline and tiny on purpose: a page that has to fetch a stylesheet to be legible is a
// page that is illegible on the machine it was mailed to.
const htmlHead = `<!DOCTYPE html>
<html>
<head>
<meta charset="%charset%">
<title>%title%</title>
<style>
body { font: 13px/1.5 system-ui, sans-serif; margin: 2rem; }
table { border-collapse: collapse; }
th, td { border: 1px solid #ccc; padding: 4px 8px; text-align: left; vertical-align: top; }
th { background: #f3f4f6; font-weight: 600; }
tr:nth-child(even) td { background: #fafafa; }
</style>
</head>
<body>
<h1>%title%</h1>
<table>
<thead>
<tr>`

func (w *htmlWriter) Begin(table string, columns []string) error {
	w.columns = columns

	charset := "UTF-8"
	if chosen, err := EncodingNamed(w.options.Encoding); err == nil {
		charset = chosen.Charset
	}

	head := &strings.Builder{}
	head.WriteString(strings.NewReplacer(
		"%charset%", charset,
		"%title%", html.EscapeString(table),
	).Replace(htmlHead))

	for _, name := range columns {
		head.WriteString("<th>")
		head.WriteString(html.EscapeString(name))
		head.WriteString("</th>")
	}
	head.WriteString("</tr>\n</thead>\n<tbody>\n")

	_, err := w.out.WriteString(head.String())
	return err
}

func (w *htmlWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		line := &strings.Builder{}
		line.WriteString("<tr>")
		for at := range w.columns {
			line.WriteString("<td>")
			line.WriteString(w.cell(text(value(row, at), w.options)))
			line.WriteString("</td>")
		}
		line.WriteString("</tr>\n")

		if _, err := w.out.WriteString(line.String()); err != nil {
			return err
		}
	}
	return nil
}

type markdownWriter struct{ documentWriter }

func newMarkdownWriter(encoded io.Writer, file io.Closer, options Options) Writer {
	return &markdownWriter{documentWriter{
		out:     bufio.NewWriter(encoded),
		file:    file,
		options: options,
		cell:    markdownCell,
	}}
}

func (w *markdownWriter) Begin(table string, columns []string) error {
	w.columns = columns

	names := make([]string, len(columns))
	rule := make([]string, len(columns))
	for at, name := range columns {
		names[at] = markdownCell(name)
		rule[at] = "---"
	}

	_, err := w.out.WriteString(
		"# " + markdownCell(table) + "\n\n" + w.row(names) + w.row(rule))
	return err
}

func (w *markdownWriter) Rows(rows [][]*string) error {
	for _, row := range rows {
		fields := make([]string, len(w.columns))
		for at := range w.columns {
			fields[at] = markdownCell(text(value(row, at), w.options))
		}
		if _, err := w.out.WriteString(w.row(fields)); err != nil {
			return err
		}
	}
	return nil
}

func (*markdownWriter) row(fields []string) string {
	return "| " + strings.Join(fields, " | ") + " |\n"
}

// markdownCell keeps a value inside its own cell: a pipe would open the next one and a line break
// would end the row, so both are written as what they are rather than as what they would do.
func markdownCell(value string) string {
	replaced := strings.NewReplacer("|", `\|`, "\r\n", "<br>", "\n", "<br>", "\r", "<br>")
	return replaced.Replace(value)
}
