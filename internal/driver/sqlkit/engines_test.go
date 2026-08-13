package sqlkit

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// SQLite is exercised against a real engine — see `driver/sqlite`, which opens a file and runs
// these statements. SQL Server and Oracle are servers, so what can be checked without one is what
// this file checks: that the statements are spelled the way those engines spell them, which is
// where every difference between the five lives.

// A migration on SQL Server is where the most differences meet at once: a bracket instead of a
// quote, a name in three parts, an index dropped by naming its table, and a default that is a
// constraint rather than a fact of the column — which is why one change there is two statements.
func TestAlterStatementsOnSQLServer(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "dbo"}
	note := "'x'"

	changes := []driver.TableChange{
		{Kind: driver.ChangeDropForeignKey, Name: "users_city_fk"},
		{Kind: driver.ChangeDropIndex, Name: "by_email"},
		{
			Kind:   driver.ChangeAlterColumn,
			Name:   "name",
			Column: driver.ColumnSpec{Name: "name", Type: "nvarchar(100)", Default: &note},
		},
		{
			Kind:  driver.ChangeCreateIndex,
			Index: driver.IndexSpec{Name: "by_email", Columns: []string{"email"}, Unique: true},
		},
	}

	want := []string{
		"ALTER TABLE [shop].[dbo].[users] DROP CONSTRAINT [users_city_fk]",
		"DROP INDEX [by_email] ON [shop].[dbo].[users]",
		"ALTER TABLE [shop].[dbo].[users] ALTER COLUMN [name] nvarchar(100) NOT NULL",
		"ALTER TABLE [shop].[dbo].[users] ADD DEFAULT 'x' FOR [name]",
		"CREATE UNIQUE INDEX [by_email] ON [shop].[dbo].[users] ([email])",
	}
	assertStatements(t, "SQLServer", DDL{Dialect: SQLServer}.AlterStatements(ns, "users", changes), want)
}

// Oracle qualifies by one level, because its schema is what a Namespace's Database holds, and it
// qualifies the *index* as well — an index there is an object beside the table rather than part of
// it, and an unqualified name would put it in whichever schema the reader happened to connect as.
func TestAlterStatementsOnOracle(t *testing.T) {
	ns := driver.Namespace{Database: "HR"}
	note := "'x'"

	changes := []driver.TableChange{
		{Kind: driver.ChangeDropIndex, Name: "BY_EMAIL"},
		{
			Kind:   driver.ChangeAlterColumn,
			Name:   "NAME",
			Column: driver.ColumnSpec{Name: "NAME", Type: "VARCHAR2(100)", Default: &note},
		},
		{
			Kind:  driver.ChangeCreateIndex,
			Index: driver.IndexSpec{Name: "BY_EMAIL", Columns: []string{"EMAIL"}},
		},
	}

	// Oracle is the one engine that restates the column *and* writes its description separately,
	// which is why an altered column here is two statements and a blank description is still spelled:
	// leaving it out would leave whatever was written there before.
	want := []string{
		`DROP INDEX "HR"."BY_EMAIL"`,
		`ALTER TABLE "HR"."USERS" MODIFY "NAME" VARCHAR2(100) NOT NULL DEFAULT 'x'`,
		`COMMENT ON COLUMN "HR"."USERS"."NAME" IS ''`,
		`CREATE INDEX "HR"."BY_EMAIL" ON "HR"."USERS" ("EMAIL")`,
	}
	assertStatements(t, "Oracle", DDL{Dialect: Oracle}.AlterStatements(ns, "USERS", changes), want)
}

// SQLite spells nothing at all for a change to a column: a database that keeps its schema as the
// text of the statements that made it cannot re-parse that text on request, so a migration shows
// an empty script rather than one that would fail.
func TestSQLiteSpellsNoColumnChange(t *testing.T) {
	spec := driver.ColumnSpec{Name: "email", Type: "TEXT"}
	if got := SQLite.AlterColumn(driver.Namespace{}, "users", "email", spec); len(got) != 0 {
		t.Errorf("SQLite.AlterColumn = %v, want nothing", got)
	}
}

// A row that may already be there is written three ways. Two engines have a clause for it; the
// other two have none, so the insert looks first and an upsert puts an update in front of it.
func TestUpsertOnAnEngineWithNoMergeClause(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "dbo"}
	id, email := "7", "a@b.c"

	write := driver.RowWrite{
		Mode:    driver.WriteUpsert,
		Columns: []string{"id", "email"},
		Keys:    []string{"id"},
		Rows:    [][]*string{{&id, &email}},
		Batch:   200,
	}

	want := []string{
		"UPDATE [shop].[dbo].[users] SET [email] = 'a@b.c' WHERE [id] = '7'",
		"INSERT INTO [shop].[dbo].[users] ([id], [email]) SELECT '7', 'a@b.c'" +
			" WHERE NOT EXISTS (SELECT 1 FROM [shop].[dbo].[users] WHERE [id] = '7')",
	}
	assertStatements(t, "SQLServer", DDL{Dialect: SQLServer}.RowStatements(ns, "users", write), want)

	// A skip is the same insert without the update, and on Oracle a SELECT of literals still needs
	// a table to read them from.
	write.Mode = driver.WriteSkip
	oracle := []string{
		`INSERT INTO "HR"."USERS" ("id", "email") SELECT '7', 'a@b.c' FROM dual` +
			` WHERE NOT EXISTS (SELECT 1 FROM "HR"."USERS" WHERE "id" = '7')`,
	}
	assertStatements(t, "Oracle",
		DDL{Dialect: Oracle}.RowStatements(driver.Namespace{Database: "HR"}, "USERS", write), oracle)
}

// Oracle takes one tuple to an INSERT however large the batch, so a batch there is more statements
// rather than a longer one.
func TestOracleWritesOneRowPerInsert(t *testing.T) {
	one, two := "1", "2"
	write := driver.RowWrite{
		Mode:    driver.WriteAppend,
		Columns: []string{"id"},
		Rows:    [][]*string{{&one}, {&two}},
		Batch:   200,
	}

	got := DDL{Dialect: Oracle}.RowStatements(driver.Namespace{Database: "HR"}, "T", write)
	if len(got) != 2 {
		t.Fatalf("statements = %v, want one per row", got)
	}
}

// A copy of a table is spelled four ways, and the two engines with no LIKE read the other table
// for its shape — one of them putting the new name in the middle of that read.
func TestCopyTableOnTheEnginesWithNoLike(t *testing.T) {
	sqlite := SQLite.CopyTable(driver.Namespace{Database: "main"}, "users", "users_copy")
	if want := `CREATE TABLE "main"."users_copy" AS SELECT * FROM "main"."users" WHERE 1 = 0`; sqlite != want {
		t.Errorf("SQLite.CopyTable =\n%s\nwant\n%s", sqlite, want)
	}

	ns := driver.Namespace{Database: "shop", Schema: "dbo"}
	mssql := SQLServer.CopyTable(ns, "users", "users_copy")
	want := "SELECT * INTO [shop].[dbo].[users_copy] FROM [shop].[dbo].[users] WHERE 1 = 0"
	if mssql != want {
		t.Errorf("SQLServer.CopyTable =\n%s\nwant\n%s", mssql, want)
	}
}

// What a search matches is one `domain` rule; how it is *spelled* is the engine's, and the five
// spell it four ways — a function taking the sensitivity, an operator per answer, an operator whose
// pattern carries the answer, and a LIKE whose sensitive form has to say so.
func TestMatchAnySpellsTheSameRuleEachEnginesWay(t *testing.T) {
	columns := []driver.Column{{Name: "email"}}
	contains := domain.TextMatch{Text: "ann", Mode: domain.MatchContains}

	cases := []struct {
		name    string
		dialect Dialect
		match   domain.TextMatch
		want    string
	}{
		{
			"MySQL compares a sensitive pattern as bytes",
			MySQL, contains,
			"CAST(`email` AS CHAR) LIKE CAST('%ann%' AS BINARY)",
		},
		{
			"SQL Server names a collation that tells case apart",
			SQLServer, contains,
			"CAST([email] AS NVARCHAR(MAX)) COLLATE Latin1_General_BIN2 LIKE '%ann%'",
		},
		{
			"Oracle's LIKE is sensitive as it stands",
			Oracle, contains,
			`CAST("email" AS VARCHAR2(4000)) LIKE '%ann%'`,
		},
		{
			"SQLite spells every mode as the expression its own regexp reads",
			SQLite, contains,
			`CAST("email" AS TEXT) REGEXP 'ann'`,
		},
		{
			"and folds the case box into the pattern",
			SQLite, domain.TextMatch{Text: "ann", Mode: domain.MatchContains, CaseInsensitive: true},
			`CAST("email" AS TEXT) REGEXP '(?i)ann'`,
		},
		{
			"a whole word is a function on the three that have one",
			Oracle, domain.TextMatch{Text: "ann", Mode: domain.MatchWord},
			`REGEXP_LIKE(CAST("email" AS VARCHAR2(4000)), '\bann\b', 'c')`,
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got, err := one.dialect.MatchAny(one.match, columns)
			if err != nil {
				t.Fatalf("MatchAny: %v", err)
			}
			if want := " WHERE (" + one.want + ")"; got != want {
				t.Errorf("MatchAny =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func assertStatements(t *testing.T, engine string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s spelled %d statements, want %d:\n%s",
			engine, len(got), len(want), strings.Join(got, "\n"))
	}
	for at := range want {
		if got[at] != want[at] {
			t.Errorf("%s[%d] =\n%s\nwant\n%s", engine, at, got[at], want[at])
		}
	}
}
