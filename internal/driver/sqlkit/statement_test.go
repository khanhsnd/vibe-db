package sqlkit

import "testing"

func TestLeadingKeyword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain select", "SELECT 1", "select"},
		{"lower cased", "select 1", "select"},
		{"leading whitespace", "\n\t  SELECT 1", "select"},
		{"parenthesised select", "(SELECT 1) UNION (SELECT 2)", "select"},
		{"line comment first", "-- pick one\nSELECT 1", "select"},
		{"hash comment first", "# pick one\nSELECT 1", "select"},
		{"block comment first", "/* pick one */ SELECT 1", "select"},
		{"stacked comments", "-- a\n/* b */\nUPDATE t SET a = 1", "update"},
		{"insert", "INSERT INTO t VALUES (1)", "insert"},
		{"cte", "WITH x AS (SELECT 1) SELECT * FROM x", "with"},
		{"empty", "   ", ""},
		{"unterminated block comment", "/* never closed", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LeadingKeyword(tt.in); got != tt.want {
				t.Errorf("LeadingKeyword(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTrimStatement(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"  SELECT 1 ;  ", "SELECT 1"},
		{"SELECT 1;;", "SELECT 1"},
		{" ; \n", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := TrimStatement(tt.in); got != tt.want {
			t.Errorf("TrimStatement(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
