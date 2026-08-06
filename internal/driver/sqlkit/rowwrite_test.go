package sqlkit

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func cell(value string) *string { return &value }

var sample = driver.RowWrite{
	Columns: []string{"id", "name"},
	Keys:    []string{"id"},
	Rows: [][]*string{
		{cell("1"), cell("Ada")},
		{cell("2"), nil},
	},
}

func ddl(dialect Dialect) DDL { return DDL{Dialect: dialect} }

func statements(dialect Dialect, write driver.RowWrite) []string {
	return ddl(dialect).RowStatements(driver.Namespace{Database: "shop", Schema: "public"},
		"users", write)
}

func only(t *testing.T, got []string, want string) {
	t.Helper()

	if len(got) != 1 {
		t.Fatalf("got %d statements, want 1:\n%s", len(got), strings.Join(got, "\n"))
	}
	if got[0] != want {
		t.Errorf("statement =\n%s\nwant\n%s", got[0], want)
	}
}

// A batch of one is what a dump and a SQL export are, and it has to stay exactly the statement the
// old InsertStatements wrote — a dump written before this change and one written after must agree
// to the character, or every file anybody has kept becomes a file that reads differently.
func TestAppendWithABatchOfOneIsOneRowPerStatement(t *testing.T) {
	write := sample
	write.Mode, write.Batch = driver.WriteAppend, 1

	got := statements(MySQL, write)
	if len(got) != 2 {
		t.Fatalf("got %d statements, want 2", len(got))
	}
	want := "INSERT INTO `shop`.`users` (`id`, `name`) VALUES ('1', 'Ada')"
	if got[0] != want {
		t.Errorf("statement =\n%s\nwant\n%s", got[0], want)
	}
	if !strings.HasSuffix(got[1], "VALUES ('2', NULL)") {
		t.Errorf("a NULL did not travel as the keyword: %s", got[1])
	}
}

// A batch above one is what an import uses: the round trips are what makes a hundred thousand rows
// take minutes instead of hours.
func TestAppendBatchesRowsIntoOneStatement(t *testing.T) {
	write := sample
	write.Mode, write.Batch = driver.WriteAppend, 10

	only(t, statements(Postgres, write),
		`INSERT INTO "public"."users" ("id", "name") VALUES ('1', 'Ada'), ('2', NULL)`)
}

// The one place the two engines genuinely disagree: PostgreSQL has to be told which columns it is
// conflicting on, and MySQL is told nothing and finds whichever key was broken.
func TestUpsertIsSpelledTheEngineWay(t *testing.T) {
	write := sample
	write.Mode, write.Batch = driver.WriteUpsert, 10

	if !strings.HasSuffix(statements(MySQL, write)[0],
		"ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)") {
		t.Errorf("MySQL upsert = %s", statements(MySQL, write)[0])
	}
	if !strings.HasSuffix(statements(Postgres, write)[0],
		`ON CONFLICT ("id") DO UPDATE SET "name" = EXCLUDED."name"`) {
		t.Errorf("Postgres upsert = %s", statements(Postgres, write)[0])
	}
}

func TestSkipIsAVerbOnOneEngineAndAClauseOnTheOther(t *testing.T) {
	write := sample
	write.Mode, write.Batch = driver.WriteSkip, 10

	if !strings.HasPrefix(statements(MySQL, write)[0], "INSERT IGNORE INTO") {
		t.Errorf("MySQL skip = %s", statements(MySQL, write)[0])
	}
	if !strings.HasSuffix(statements(Postgres, write)[0], `ON CONFLICT ("id") DO NOTHING`) {
		t.Errorf("Postgres skip = %s", statements(Postgres, write)[0])
	}
}

// A key column is what the row was found by, so it is not also set. Updating it would be a rename
// asked for by a mapping that never said so.
func TestUpdateLeavesTheKeyOutOfTheSet(t *testing.T) {
	write := sample
	write.Mode, write.Rows = driver.WriteUpdate, sample.Rows[:1]

	only(t, statements(MySQL, write),
		"UPDATE `shop`.`users` SET `name` = 'Ada' WHERE `id` = '1'")
}

func TestDeleteFindsRowsByTheKeyAlone(t *testing.T) {
	write := sample
	write.Mode, write.Rows = driver.WriteDelete, sample.Rows[:1]

	only(t, statements(Postgres, write), `DELETE FROM "public"."users" WHERE "id" = '1'`)
}

// `= NULL` is true of nothing at all, so a key holding one has to be compared with IS NULL — without
// which a delete would report success having matched no row.
func TestANullKeyIsMatchedWithIsNull(t *testing.T) {
	write := driver.RowWrite{
		Mode:    driver.WriteDelete,
		Columns: []string{"id"},
		Keys:    []string{"id"},
		Rows:    [][]*string{{nil}},
	}

	only(t, statements(MySQL, write), "DELETE FROM `shop`.`users` WHERE `id` IS NULL")
}

// The statement that must never be written: a row with nothing to find it by would otherwise become
// a DELETE with no WHERE, which is every row of the table.
func TestARowWithNoKeyMatchesNothing(t *testing.T) {
	write := driver.RowWrite{
		Mode:    driver.WriteDelete,
		Columns: []string{"name"},
		Keys:    []string{"id"},
		Rows:    [][]*string{{cell("Ada")}},
	}

	only(t, statements(MySQL, write), "DELETE FROM `shop`.`users` WHERE 1 = 0")
}

// A value a reader typed reaches the statement as a literal and never as SQL, which is the whole of
// what stands between an imported file and the server.
func TestAValueIsQuotedAndNeverPasted(t *testing.T) {
	write := driver.RowWrite{
		Mode:    driver.WriteAppend,
		Batch:   1,
		Columns: []string{"name"},
		Rows:    [][]*string{{cell(`'); DROP TABLE users; --`)}},
	}

	only(t, statements(Postgres, write),
		`INSERT INTO "public"."users" ("name") VALUES ('''); DROP TABLE users; --')`)
}
