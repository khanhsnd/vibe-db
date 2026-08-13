package sqlkit

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A partition is one intention with three grammars, which is what PartitionStyle exists to tell
// apart — and the one engine where a piece *is* a table is the one whose statements never say the
// word PARTITION at all.
func TestPartitionStatementsPerStyle(t *testing.T) {
	mysqlNS := driver.Namespace{Database: "shop"}
	pgNS := driver.Namespace{Database: "shop", Schema: "public"}
	oracleNS := driver.Namespace{Database: "HR"}

	spec := driver.PartitionSpec{Name: "p2027", Bounds: "VALUES LESS THAN (2028)"}

	assertStatements(t, "MySQL",
		[]string{
			MySQL.AddPartition(mysqlNS, "sales", spec),
			MySQL.DropPartition(mysqlNS, "sales", "p2026"),
			MySQL.TruncatePartition(mysqlNS, "sales", "p2026"),
		},
		[]string{
			"ALTER TABLE `shop`.`sales` ADD PARTITION " +
				"(PARTITION `p2027` VALUES LESS THAN (2028))",
			"ALTER TABLE `shop`.`sales` DROP PARTITION `p2026`",
			"ALTER TABLE `shop`.`sales` TRUNCATE PARTITION `p2026`",
		})

	// The bounds are the reader's own clause, so PostgreSQL's own grammar goes in unchanged.
	ranged := driver.PartitionSpec{
		Name:   "p2027",
		Bounds: "FOR VALUES FROM ('2027-01-01') TO ('2028-01-01')",
	}
	assertStatements(t, "PostgreSQL",
		[]string{
			Postgres.AddPartition(pgNS, "sales", ranged),
			Postgres.DropPartition(pgNS, "sales", "p2026"),
			Postgres.TruncatePartition(pgNS, "sales", "p2026"),
		},
		[]string{
			`CREATE TABLE "public"."p2027" PARTITION OF "public"."sales" ` +
				`FOR VALUES FROM ('2027-01-01') TO ('2028-01-01')`,
			`DROP TABLE "public"."p2026"`,
			`TRUNCATE TABLE "public"."p2026"`,
		})

	assertStatements(t, "Oracle",
		[]string{
			Oracle.AddPartition(oracleNS, "SALES", spec),
			Oracle.DropPartition(oracleNS, "SALES", "P2026"),
			Oracle.TruncatePartition(oracleNS, "SALES", "P2026"),
		},
		[]string{
			`ALTER TABLE "HR"."SALES" ADD PARTITION "p2027" VALUES LESS THAN (2028)`,
			`ALTER TABLE "HR"."SALES" DROP PARTITION "P2026"`,
			`ALTER TABLE "HR"."SALES" TRUNCATE PARTITION "P2026"`,
		})
}

// An engine with no partition this app can address spells nothing rather than a statement that
// could only fail — the rule every other unsupported change in this package follows, and what the
// run beside it answers `domain.Unsupported` for.
func TestPartitionSpellsNothingWhereThereIsNone(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "dbo"}
	spec := driver.PartitionSpec{Name: "p1", Bounds: "VALUES LESS THAN (1)"}

	for name, dialect := range map[string]Dialect{"SQLServer": SQLServer, "SQLite": SQLite} {
		if got := dialect.AddPartition(ns, "sales", spec); got != "" {
			t.Errorf("%s.AddPartition = %s, want nothing", name, got)
		}
		if got := dialect.DropPartition(ns, "sales", "p1"); got != "" {
			t.Errorf("%s.DropPartition = %s, want nothing", name, got)
		}
		if got := dialect.TruncatePartition(ns, "sales", "p1"); got != "" {
			t.Errorf("%s.TruncatePartition = %s, want nothing", name, got)
		}
	}
}

// The one statement in this package that names an object inside a **string literal**, because on
// that engine a refresh is a procedure call rather than a statement of its own.
func TestRefreshNamesTheViewTheWayEachEngineTakesIt(t *testing.T) {
	pg := Postgres.Refresh(driver.Namespace{Database: "shop", Schema: "public"}, "sales_by_month")
	if want := `REFRESH MATERIALIZED VIEW "public"."sales_by_month"`; pg != want {
		t.Errorf("Postgres.Refresh =\n%s\nwant\n%s", pg, want)
	}

	oracle := Oracle.Refresh(driver.Namespace{Database: "HR"}, "SALES_BY_MONTH")
	if want := `BEGIN DBMS_MVIEW.REFRESH('HR.SALES_BY_MONTH'); END;`; oracle != want {
		t.Errorf("Oracle.Refresh =\n%s\nwant\n%s", oracle, want)
	}

	if got := MySQL.Refresh(driver.Namespace{Database: "shop"}, "sales"); got != "" {
		t.Errorf("MySQL.Refresh = %s, want nothing — that engine has no such view", got)
	}
}

// A description of a column is a *clause* on one engine and a *statement* on two others, which is
// the whole of what the two flags say — and on the two that have neither it is nothing at all.
func TestColumnCommentIsAClauseOrAStatement(t *testing.T) {
	spec := driver.ColumnSpec{Name: "email", Type: "varchar(255)", Comment: "where to write"}

	mysql := MySQL.ColumnClause(spec)
	want := "`email` varchar(255) NOT NULL COMMENT 'where to write'"
	if mysql != want {
		t.Errorf("MySQL.ColumnClause =\n%s\nwant\n%s", mysql, want)
	}
	if got := MySQL.ColumnComment(driver.Namespace{Database: "shop"}, "users", "email", "x"); got != "" {
		t.Error("MySQL spelled a COMMENT ON as well as the clause, which would say it twice")
	}

	ns := driver.Namespace{Database: "shop", Schema: "public"}
	if got := Postgres.ColumnClause(spec); got != `"email" varchar(255) NOT NULL` {
		t.Errorf("PostgreSQL put the description in the clause: %s", got)
	}

	statement := Postgres.ColumnComment(ns, "users", "email", "where to write")
	wantStatement := `COMMENT ON COLUMN "public"."users"."email" IS 'where to write'`
	if statement != wantStatement {
		t.Errorf("Postgres.ColumnComment =\n%s\nwant\n%s", statement, wantStatement)
	}

	// Neither flag: the description is dropped rather than spelled as something the server would
	// refuse, and the engine says so to the frontend with `hasColumnComments`.
	if got := SQLServer.ColumnComment(ns, "users", "email", "x"); got != "" {
		t.Errorf("SQLServer.ColumnComment = %s, want nothing", got)
	}
	if got := SQLServer.ColumnClause(spec); got != "[email] varchar(255) NOT NULL" {
		t.Errorf("SQLServer.ColumnClause = %s", got)
	}
}

// A CREATE TABLE and the descriptions that follow it are one method, so a dump and a create run the
// same statements. A column nobody described contributes nothing — there is no earlier description
// on a table that does not exist yet for an empty literal to clear.
func TestCreateTableStatementsCarryTheDescriptions(t *testing.T) {
	spec := driver.TableSpec{
		Namespace: driver.Namespace{Database: "shop", Schema: "public"},
		Table:     "users",
		Columns: []driver.ColumnSpec{
			{Name: "id", Type: "integer", PrimaryKey: true},
			{Name: "email", Type: "text", Nullable: true, Comment: "where to write"},
		},
	}

	got := Postgres.CreateTableStatements(spec)
	if len(got) != 2 {
		t.Fatalf("spelled %d statements, want the table and one description: %v", len(got), got)
	}
	want := `COMMENT ON COLUMN "public"."users"."email" IS 'where to write'`
	if got[1] != want {
		t.Errorf("description =\n%s\nwant\n%s", got[1], want)
	}

	// One engine over, the same table is one statement: the descriptions are already in it.
	if mysql := MySQL.CreateTableStatements(spec); len(mysql) != 1 {
		t.Errorf("MySQL spelled %d statements, want 1: %v", len(mysql), mysql)
	}
}

// The three kinds only some engines have are dropped by the same speller as the first four, and
// differ from them in exactly one word — which is the whole reason `SourceKind.Keyword` exists.
func TestDropSourceNamesTheNewKinds(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	cases := map[driver.SourceKind]string{
		driver.SourceMaterializedView: `DROP MATERIALIZED VIEW "public"."sales"`,
		driver.SourceSequence:         `DROP SEQUENCE "public"."sales"`,
	}
	for kind, want := range cases {
		got := Postgres.DropSource(ns, driver.SourceRef{Kind: kind, Name: "sales"})
		if got != want {
			t.Errorf("DropSource(%s) =\n%s\nwant\n%s", kind, got, want)
		}
	}

	event := MySQL.DropSource(
		driver.Namespace{Database: "shop"},
		driver.SourceRef{Kind: driver.SourceEvent, Name: "nightly"},
	)
	if want := "DROP EVENT `shop`.`nightly`"; event != want {
		t.Errorf("DropSource(event) =\n%s\nwant\n%s", event, want)
	}
}

// A starter is the shape of a declaration with nothing in it yet, and an engine with no such object
// answers with nothing — which is what the frontend reads as there being no New to offer.
func TestStartersOnlyWhereTheEngineHasTheObject(t *testing.T) {
	ns := driver.Namespace{Database: "shop", Schema: "public"}

	held := map[string]struct {
		dialect Dialect
		kind    driver.SourceKind
		want    bool
	}{
		"PostgreSQL materialized view": {Postgres, driver.SourceMaterializedView, true},
		"PostgreSQL sequence":          {Postgres, driver.SourceSequence, true},
		"PostgreSQL event":             {Postgres, driver.SourceEvent, false},
		"MySQL event":                  {MySQL, driver.SourceEvent, true},
		"MySQL sequence":               {MySQL, driver.SourceSequence, false},
		"MySQL materialized view":      {MySQL, driver.SourceMaterializedView, false},
		"SQL Server sequence":          {SQLServer, driver.SourceSequence, true},
		"SQLite sequence":              {SQLite, driver.SourceSequence, false},
		"Oracle materialized view":     {Oracle, driver.SourceMaterializedView, true},
	}

	for name, one := range held {
		got := one.dialect.NewSource(ns, driver.SourceRef{Kind: one.kind, Name: "thing"})
		if (got != "") != one.want {
			t.Errorf("%s starter = %q, want one: %v", name, got, one.want)
		}
	}
}

// AlterStatements is what a migration shows and what its Apply runs, so a partition change has to
// reach it from the same speller every other change does.
func TestAlterStatementsSpellsPartitionChanges(t *testing.T) {
	ns := driver.Namespace{Database: "shop"}

	changes := []driver.TableChange{
		{
			Kind:      driver.ChangeAddPartition,
			Partition: driver.PartitionSpec{Name: "p2027", Bounds: "VALUES LESS THAN (2028)"},
		},
		{Kind: driver.ChangeTruncatePartition, Name: "p2025"},
		{Kind: driver.ChangeDropPartition, Name: "p2024"},
	}

	assertStatements(t, "MySQL", DDL{Dialect: MySQL}.AlterStatements(ns, "sales", changes), []string{
		"ALTER TABLE `shop`.`sales` ADD PARTITION (PARTITION `p2027` VALUES LESS THAN (2028))",
		"ALTER TABLE `shop`.`sales` TRUNCATE PARTITION `p2025`",
		"ALTER TABLE `shop`.`sales` DROP PARTITION `p2024`",
	})

	// An engine with no partition to address contributes nothing rather than three statements it
	// could not run — the same answer SQLite gives for a column change.
	if got := (DDL{Dialect: SQLite}).AlterStatements(ns, "sales", changes); len(got) != 0 {
		t.Errorf("SQLite spelled %v, want nothing", got)
	}
}
