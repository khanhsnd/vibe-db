package mysql

import (
	"database/sql"
	"testing"
)

// MySQL reports a literal default as the bare value and only marks an *expression* one
// DEFAULT_GENERATED. Everywhere else in the app a default is an expression — it goes back into an
// ALTER TABLE, a dump and a migration's script — so a text literal has to be quoted on the way in
// or the statement built from it names an identifier that is not there.
func TestDefaultExpressionQuotesALiteralAndLeavesAnExpressionAlone(t *testing.T) {
	value := func(text string) sql.NullString { return sql.NullString{String: text, Valid: true} }

	cases := []struct {
		name     string
		dataType string
		stored   sql.NullString
		extra    string
		want     string
	}{
		{"a text literal is quoted", "varchar", value("x"), "", "'x'"},
		{"a quote inside it is doubled", "varchar", value("it's"), "", "'it''s'"},
		{"a number is written bare", "int", value("0"), "", "0"},
		{"an expression travels verbatim", "varchar", value("(now())"), generatedDefault, "(now())"},
		{"CURRENT_TIMESTAMP is not a string", "timestamp", value("CURRENT_TIMESTAMP"), "", "CURRENT_TIMESTAMP"},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := defaultExpression(one.dataType, one.stored, one.extra)
			if got == nil || *got != one.want {
				t.Errorf("defaultExpression = %v, want %s", got, one.want)
			}
		})
	}

	if got := defaultExpression("varchar", sql.NullString{}, ""); got != nil {
		t.Errorf("a column with no default = %v, want nil", got)
	}
}
