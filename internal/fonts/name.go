// Package fonts answers what the host machine can draw with, which is what the font picker in
// Settings offers.
package fonts

import "strings"

// The tag Windows appends to a registered font.
var tags = []string{" (TrueType)", " (OpenType)", " (All res)", " (VGA res)"}

// The styles a family's variants are registered under. Only these four are stripped, because only
// these four resolve through the base family: `Segoe UI Light` and `Arial Black` are families a
// stylesheet can name, where `Arial Bold` is Arial asked for in bold.
var styles = []string{" Bold Italic", " Bold", " Italic", " Oblique", " Regular"}

// familyOf reads the family out of one registered name: `Cascadia Mono Regular (TrueType)` is
// `Cascadia Mono`, and a collection registered under several names — `Cambria & Cambria Math` —
// is the first of them.
func familyOf(entry string) string {
	name := entry
	for _, tag := range tags {
		name = strings.TrimSuffix(name, tag)
	}

	name, _, _ = strings.Cut(name, " & ")

	for _, style := range styles {
		if trimmed, cut := strings.CutSuffix(name, style); cut {
			name = trimmed
			break
		}
	}

	return strings.TrimSpace(name)
}
