package app

import (
	"bufio"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// A dump of a document store is a file of commands rather than of SQL, so calling it `.sql` would
// be a lie about its contents.
func TestDumpFileFollowsWhatTheEngineStores(t *testing.T) {
	if got := dumpFileFor(domain.EngineMySQL).extension; got != "sql" {
		t.Errorf("MySQL extension = %s, want sql", got)
	}
	if got := dumpFileFor(domain.EngineMongo).extension; got != "js" {
		t.Errorf("MongoDB extension = %s, want js", got)
	}
}

// What the save dialog suggests is the narrowest thing the dump is *of*: the table, then the schema
// the tables live in, then the database holding it.
func TestDumpFileNameNarrowsToWhatIsBeingDumped(t *testing.T) {
	tests := []struct {
		name  string
		input DumpInput
		want  string
	}{
		{
			name:  "one table",
			input: DumpInput{Namespace: Namespace{Database: "shop", Schema: "public"}, Table: "orders"},
			want:  "orders",
		},
		{
			name:  "a schema's tables",
			input: DumpInput{Namespace: Namespace{Database: "shop", Schema: "public"}},
			want:  "public",
		},
		{
			name:  "a database with no schemas",
			input: DumpInput{Namespace: Namespace{Database: "shop"}},
			want:  "shop",
		},
		{name: "nothing named at all", input: DumpInput{}, want: "dump"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.fileName(); got != tt.want {
				t.Errorf("fileName() = %s, want %s", got, tt.want)
			}
		})
	}
}

// Every statement is terminated, which is what both the SQL engines and the shell read back — a
// multi-line declaration included, since it is one statement however many lines it takes.
func TestWriteStatementsTerminatesEachOne(t *testing.T) {
	out := &strings.Builder{}
	writer := bufio.NewWriter(out)

	statements := []string{"CREATE TABLE `orders` (\n  `id` int\n)", "INSERT INTO `orders` VALUES (1)"}
	if err := writeStatements(writer, statements); err != nil {
		t.Fatalf("writeStatements: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := "CREATE TABLE `orders` (\n  `id` int\n);\nINSERT INTO `orders` VALUES (1);\n"
	if got := out.String(); got != want {
		t.Errorf("wrote\n%q\nwant\n%q", got, want)
	}
}
