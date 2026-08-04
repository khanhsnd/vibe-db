package sqlkit

import "strings"

// Span is where one statement sits in the script it was cut from. It is what lets a warning point
// back at the statement that earned it in the text the reader is looking at, rather than at a
// copy of it — Start and End are byte offsets, and End is exclusive.
type Span struct {
	Start int
	End   int
}

// Spans cuts a script into the statements it holds. What ends a statement is a semicolon that is
// not *inside* something, and what counts as inside is lexical and therefore the dialect's — see
// Lex, which is where that is settled once for the three things built on it.
//
// A chunk with no statement in it is dropped rather than returned: trailing whitespace, and the
// comment a script ends on, are not statements the server has anything to do with.
func (d Dialect) Spans(script string) []Span {
	spans := []Span{}
	start := 0

	keep := func(end int) {
		if span, ok := trimSpan(script, start, end); ok {
			spans = append(spans, span)
		}
	}

	for _, one := range d.Lex(script) {
		if one.Kind == TokenPunct && one.Text == ";" {
			keep(one.Start)
			start = one.End
		}
	}
	keep(len(script))

	return spans
}

// Split is Spans read as text, which is what everything that only wants to *run* the statements
// asks for.
func (d Dialect) Split(script string) []string {
	spans := d.Spans(script)

	statements := make([]string, 0, len(spans))
	for _, span := range spans {
		statements = append(statements, script[span.Start:span.End])
	}
	return statements
}

// trimSpan narrows a chunk to the text worth sending and says whether there is any: a chunk whose
// leading keyword is empty holds nothing but whitespace and comments.
func trimSpan(script string, start, end int) (Span, bool) {
	const blank = " \t\r\n"

	chunk := script[start:end]
	if LeadingKeyword(chunk) == "" {
		return Span{}, false
	}

	lead := len(chunk) - len(strings.TrimLeft(chunk, blank))
	tail := len(chunk) - len(strings.TrimRight(chunk, blank))
	return Span{Start: start + lead, End: end - tail}, true
}
