package fonts

import "testing"

func TestFamilyOf(t *testing.T) {
	cases := map[string]string{
		"Consolas (TrueType)":                   "Consolas",
		"Cascadia Mono Regular (TrueType)":      "Cascadia Mono",
		"JetBrains Mono Bold Italic (TrueType)": "JetBrains Mono",
		"Arial Bold (TrueType)":                 "Arial",
		"Cambria & Cambria Math (TrueType)":     "Cambria",
		"Small Fonts (VGA res)":                 "Small Fonts",
		// A weight that is a family of its own, so it survives whole.
		"Segoe UI Light (TrueType)": "Segoe UI Light",
		"Arial Black (TrueType)":    "Arial Black",
	}

	for entry, want := range cases {
		if got := familyOf(entry); got != want {
			t.Errorf("familyOf(%q) = %q, want %q", entry, got, want)
		}
	}
}
