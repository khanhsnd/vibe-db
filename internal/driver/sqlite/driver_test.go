package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	dbdriver "github.com/khanhsnd/vibe-db/internal/driver"
)

// A file engine is the one engine a test can hold the whole of, so this is an end-to-end run
// against a real database rather than a check of the statements that would have been sent. What it
// is really proving is the *shared* half: `sqlkit.Data` builds every read and write below, and this
// is where a mistake in the paging, the placeholders or the quoting shows up as a failure instead
// of as a string that merely looks right.
func openTestFile(t *testing.T) (*session, dbdriver.Namespace) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "shop.db")
	seed, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		t.Fatalf("open seed: %v", err)
	}

	const schema = `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			note TEXT DEFAULT 'none'
		);
		CREATE INDEX users_email ON users (email);
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY,
			user_id INTEGER REFERENCES users (id) ON DELETE CASCADE
		);
		CREATE VIEW recent AS SELECT * FROM orders;
		INSERT INTO users (id, email) VALUES (1, 'ann@example.com'), (2, 'BOB@example.com');`

	if _, err := seed.Exec(schema); err != nil {
		t.Fatalf("seed: %v", err)
	}
	seed.Close()

	profile := domain.ConnectionProfile{Engine: domain.EngineSQLite, Database: path}
	opened, err := New().Open(context.Background(), profile, dbdriver.Credentials{}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { opened.Close() })

	return opened.(*session), dbdriver.Namespace{Database: mainDatabase}
}

func TestOpenRefusesAFileThatIsNotThere(t *testing.T) {
	profile := domain.ConnectionProfile{
		Engine:   domain.EngineSQLite,
		Database: filepath.Join(t.TempDir(), "absent.db"),
	}

	_, err := New().Open(context.Background(), profile, dbdriver.Credentials{}, nil)
	if !errors.Is(err, domain.ErrFileUnreadable) {
		t.Fatalf("Open = %v, want ErrFileUnreadable — SQLite would otherwise make an empty file", err)
	}
	if _, statErr := os.Stat(profile.Database); statErr == nil {
		t.Error("the refused open still created the file")
	}
}

func TestObjectsListsTablesAndViewsButNotTheEnginesOwn(t *testing.T) {
	s, ns := openTestFile(t)

	objects, err := s.Objects(context.Background(), ns)
	if err != nil {
		t.Fatalf("Objects: %v", err)
	}

	if got, want := len(objects.Tables), 2; got != want {
		t.Errorf("tables = %v, want %d of them", objects.Tables, want)
	}
	if got, want := len(objects.Views), 1; got != want {
		t.Errorf("views = %v, want %d", objects.Views, want)
	}
	if len(objects.Routines) != 0 {
		t.Errorf("routines = %v, want none: SQLite stores no routine", objects.Routines)
	}
}

func TestTableStructureReadsTheKeyTheIndexAndTheConstraint(t *testing.T) {
	s, ns := openTestFile(t)

	structure, err := s.TableStructure(context.Background(), ns, "users")
	if err != nil {
		t.Fatalf("TableStructure: %v", err)
	}

	if len(structure.Columns) != 3 || !structure.Columns[0].PrimaryKey {
		t.Fatalf("columns = %+v, want three with the first keyed", structure.Columns)
	}
	if structure.Columns[1].Nullable {
		t.Error("email is declared NOT NULL and was read as nullable")
	}
	if got := structure.Columns[2].Default; got == nil || *got != "'none'" {
		t.Errorf("note default = %v, want the expression the declaration carried", got)
	}
	if len(structure.Indexes) != 1 || structure.Indexes[0].Name != "users_email" {
		t.Errorf("indexes = %+v, want the one that was declared", structure.Indexes)
	}

	orders, err := s.TableStructure(context.Background(), ns, "orders")
	if err != nil {
		t.Fatalf("TableStructure(orders): %v", err)
	}
	if len(orders.ForeignKeys) != 1 {
		t.Fatalf("foreign keys = %+v, want one", orders.ForeignKeys)
	}
	if got := orders.ForeignKeys[0]; got.ReferencedTable != "users" || got.OnDelete != "CASCADE" {
		t.Errorf("foreign key = %+v, want users ON DELETE CASCADE", got)
	}
}

// The page, its total and the statement behind it all come out of `sqlkit.Data`, so this is the
// shared half being exercised rather than anything in this package.
func TestTableRowsPagesAndCounts(t *testing.T) {
	s, ns := openTestFile(t)

	page := dbdriver.TablePage{Namespace: ns, Table: "users", Limit: 1, Offset: 1}
	rows, err := s.TableRows(context.Background(), page)
	if err != nil {
		t.Fatalf("TableRows: %v", err)
	}

	if rows.Total != 2 {
		t.Errorf("total = %d, want 2", rows.Total)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("rows = %d, want the one the page asked for", len(rows.Rows))
	}
	if got := *rows.Rows[0][0]; got != "2" {
		t.Errorf("first row id = %s, want 2 — the page is cut after the key's order", got)
	}
}

func TestWritingARowGoesThroughTheKey(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	value := "carol@example.com"
	insert := dbdriver.RowInsert{
		Namespace: ns,
		Table:     "users",
		Values:    map[string]*string{"email": &value},
	}
	if err := s.InsertRow(ctx, insert); err != nil {
		t.Fatalf("InsertRow: %v", err)
	}

	changed := "carol@elsewhere.com"
	edit := dbdriver.CellEdit{
		Namespace: ns,
		Table:     "users",
		Column:    "email",
		Value:     &changed,
		Key:       map[string]string{"id": "3"},
	}
	if err := s.UpdateCell(ctx, edit); err != nil {
		t.Fatalf("UpdateCell: %v", err)
	}

	removal := dbdriver.RowDelete{Namespace: ns, Table: "users", Key: map[string]string{"id": "3"}}
	if err := s.DeleteRow(ctx, removal); err != nil {
		t.Fatalf("DeleteRow: %v", err)
	}

	rows, err := s.TableRows(ctx, dbdriver.TablePage{Namespace: ns, Table: "users"})
	if err != nil {
		t.Fatalf("TableRows: %v", err)
	}
	if rows.Total != 2 {
		t.Errorf("total = %d, want 2 again once the row that was added is gone", rows.Total)
	}
}

// The `regexp` this adapter registers is what makes every search mode answerable here, and the
// case box with it: SQLite's own LIKE ignores case for ASCII and for nothing else, which is why
// none of these four goes through one.
func TestMatchingRowsAnswersEveryModeThroughTheRegisteredRegexp(t *testing.T) {
	s, ns := openTestFile(t)

	cases := []struct {
		name  string
		match domain.TextMatch
		want  int64
	}{
		{"contains", domain.TextMatch{Text: "ann", Mode: domain.MatchContains}, 1},
		{
			"contains is blind to case when asked",
			domain.TextMatch{Text: "bob", Mode: domain.MatchContains, CaseInsensitive: true},
			1,
		},
		{
			"and is not when it is not",
			domain.TextMatch{Text: "bob", Mode: domain.MatchContains},
			0,
		},
		{"prefix", domain.TextMatch{Text: "ann@", Mode: domain.MatchPrefix}, 1},
		{"whole word", domain.TextMatch{Text: "example", Mode: domain.MatchWord}, 2},
		{"expression", domain.TextMatch{Text: "^[a-z]+@", Mode: domain.MatchRegex}, 1},
		{"a number is found as text", domain.TextMatch{Text: "2", Mode: domain.MatchContains}, 1},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			search := dbdriver.RowSearch{Namespace: ns, Table: "users", Match: one.match}
			found, err := s.MatchingRows(context.Background(), search)
			if err != nil {
				t.Fatalf("MatchingRows: %v", err)
			}
			if found.Count != one.want {
				t.Errorf("count = %d, want %d (%s)", found.Count, one.want, found.Statement)
			}
		})
	}
}

// A dump here is the engine's own words: SQLite keeps the text of the statement that made each
// object, so nothing is described back out of a catalogue.
func TestCreateStatementsAreTheDeclarationsTheFileHolds(t *testing.T) {
	s, ns := openTestFile(t)

	statements, err := s.CreateStatements(context.Background(), ns, "users")
	if err != nil {
		t.Fatalf("CreateStatements: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("statements = %v, want the table and its index", statements)
	}
	if got := statements[0]; got[:12] != "CREATE TABLE" {
		t.Errorf("first statement = %q, want the table before its index", got)
	}
}

// What SQLite has not got says which thing it has not got, rather than being left for the engine
// to refuse or approximated by a rebuild the reader did not ask for.
func TestWhatAFileCannotDoNamesWhatItCannotDo(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	refusals := map[string]error{
		"truncate":    s.TruncateTable(ctx, ns, "users"),
		"alter":       s.AlterColumn(ctx, ns, "users", "email", dbdriver.ColumnSpec{Type: "TEXT"}),
		"primary key": s.SetPrimaryKey(ctx, ns, "users", []string{"email"}),
		"database":    s.CreateDatabase(ctx, dbdriver.DatabaseSpec{Name: "other"}),
	}

	for what, err := range refusals {
		if !errors.Is(err, domain.ErrUnsupported) {
			t.Errorf("%s = %v, want ErrUnsupported naming what is missing", what, err)
		}
	}

	if got := s.AlterStatements(ns, "users", []dbdriver.TableChange{{
		Kind: dbdriver.ChangeAlterColumn, Name: "email",
	}}); len(got) != 0 {
		t.Errorf("AlterStatements = %v, want nothing: a change with no statement shows none", got)
	}
}

// A rename and a new column are the two changes a file does take, and both go through the shared
// speller — so this is `sqlkit.DDL` running against a real engine.
func TestTheTwoChangesAFileTakes(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	spec := dbdriver.ColumnSpec{Name: "nickname", Type: "TEXT", Nullable: true}
	if err := s.AddColumn(ctx, ns, "users", spec); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}
	if err := s.RenameColumn(ctx, ns, "users", "nickname", "handle"); err != nil {
		t.Fatalf("RenameColumn: %v", err)
	}
	if err := s.RenameTable(ctx, ns, "users", "people"); err != nil {
		t.Fatalf("RenameTable: %v", err)
	}

	columns, err := s.columns(ctx, ns, "people")
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if got := columns[len(columns)-1].Name; got != "handle" {
		t.Errorf("last column = %q, want handle", got)
	}
}

// An index and a copied table are the two statements SQLite spells differently enough from every
// other engine to be worth running rather than reading: the namespace of a CREATE INDEX goes on the
// *index* and the table is then bare, and a copy is a read of the other table because there is no
// LIKE at all.
func TestTheStatementsSQLiteSpellsItsOwnWay(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	spec := dbdriver.IndexSpec{Name: "users_note", Columns: []string{"note"}}
	if err := s.CreateIndex(ctx, ns, "users", spec); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	if err := s.DropIndex(ctx, ns, "users", "users_note"); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}

	if err := s.DuplicateTable(ctx, ns, "users", "users_copy", true); err != nil {
		t.Fatalf("DuplicateTable: %v", err)
	}

	rows, err := s.TableRows(ctx, dbdriver.TablePage{Namespace: ns, Table: "users_copy"})
	if err != nil {
		t.Fatalf("TableRows: %v", err)
	}
	if rows.Total != 2 {
		t.Errorf("copied rows = %d, want the two the original held", rows.Total)
	}
}

// Every mode of a row write is one list of statements from one speller — an import, a SQL export
// and the data half of a dump are the same question — so running all five against a real engine is
// the cheapest proof that `sqlkit.DDL.RowStatements` spells statements a server accepts.
func TestEveryRowWriteModeRunsAsSpelled(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	row := func(id, email string) []*string { return []*string{&id, &email} }
	write := func(mode dbdriver.RowWriteMode, rows [][]*string) dbdriver.RowWrite {
		return dbdriver.RowWrite{
			Mode:    mode,
			Columns: []string{"id", "email"},
			Keys:    []string{"id"},
			Rows:    rows,
			Batch:   2,
		}
	}

	runs := []struct {
		name  string
		write dbdriver.RowWrite
		total int64
	}{
		{"append", write(dbdriver.WriteAppend, [][]*string{
			row("10", "j@example.com"), row("11", "k@example.com"),
		}), 4},
		{"skip leaves what is there", write(dbdriver.WriteSkip, [][]*string{
			row("10", "changed@example.com"),
		}), 4},
		{"upsert changes it and adds what is missing", write(dbdriver.WriteUpsert, [][]*string{
			row("10", "changed@example.com"), row("12", "l@example.com"),
		}), 5},
		{"update changes only", write(dbdriver.WriteUpdate, [][]*string{
			row("12", "again@example.com"),
		}), 5},
		{"delete removes by the key", write(dbdriver.WriteDelete, [][]*string{
			row("10", ""), row("11", ""), row("12", ""),
		}), 2},
	}

	for _, one := range runs {
		t.Run(one.name, func(t *testing.T) {
			for _, statement := range s.RowStatements(ns, "users", one.write) {
				if _, err := s.db.ExecContext(ctx, statement); err != nil {
					t.Fatalf("%s\n%v", statement, err)
				}
			}

			rows, err := s.TableRows(ctx, dbdriver.TablePage{Namespace: ns, Table: "users"})
			if err != nil {
				t.Fatalf("TableRows: %v", err)
			}
			if rows.Total != one.total {
				t.Errorf("total = %d, want %d", rows.Total, one.total)
			}
		})
	}
}

func TestExplainReadsTheTreeAsSteps(t *testing.T) {
	s, ns := openTestFile(t)

	query := dbdriver.Query{
		Namespace: ns,
		SQL:       "SELECT * FROM users WHERE email = 'ann@example.com'",
		Explain:   true,
	}
	result, err := s.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if result.Plan == nil || len(result.Plan.Nodes) == 0 {
		t.Fatal("no plan: EXPLAIN QUERY PLAN answers with a row per step")
	}
	if result.Plan.Costed {
		t.Error("SQLite puts no number on a step and the plan must not claim it does")
	}
	if got := result.Plan.Nodes[0].Type; got != "SEARCH" && got != "SCAN" {
		t.Errorf("first step = %q, want the engine's own word for what it does", got)
	}
	if len(result.Rows) != 1 {
		t.Errorf("rows = %d, want the statement to have run as well as been explained", len(result.Rows))
	}
}

// A result the grid can write back to is one whose rows map onto stored rows one for one, and it
// is proven rather than assumed — see attachSource.
func TestQueryMarksASingleTableReadWritable(t *testing.T) {
	s, ns := openTestFile(t)

	result, err := s.Query(context.Background(), dbdriver.Query{
		Namespace: ns,
		SQL:       "SELECT * FROM users",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Table != "users" {
		t.Errorf("table = %q, want users", result.Table)
	}

	joined, err := s.Query(context.Background(), dbdriver.Query{
		Namespace: ns,
		SQL:       "SELECT u.id FROM users u JOIN orders o ON o.user_id = u.id",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if joined.Table != "" {
		t.Errorf("table = %q, want none: a join maps onto no single stored row", joined.Table)
	}
}

// The file stores the statement that made the view, so what a designer opens on is the reader's
// own declaration rather than a description of it — and saving it back is a drop and that same
// text, which has to leave the view readable again.
func TestSourceReadsAViewAndSavesItBack(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()
	ref := dbdriver.SourceRef{Kind: dbdriver.SourceView, Name: "recent"}

	source, err := s.Source(ctx, ns, ref)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	if source.Definition != "CREATE VIEW recent AS SELECT * FROM orders" {
		t.Errorf("Definition = %q", source.Definition)
	}

	statements := s.SourceStatements(ns, dbdriver.SourceSave{
		Replaces:   ref,
		Definition: source.Definition,
	})
	want := []string{`DROP VIEW "main"."recent"`, source.Definition}
	if len(statements) != 2 || statements[0] != want[0] || statements[1] != want[1] {
		t.Fatalf("SourceStatements = %#v, want %#v", statements, want)
	}

	err = s.SaveSource(ctx, ns, dbdriver.SourceSave{Replaces: ref, Definition: source.Definition})
	if err != nil {
		t.Fatalf("SaveSource: %v", err)
	}
	if _, err := s.Source(ctx, ns, ref); err != nil {
		t.Fatalf("Source after save: %v", err)
	}
}

// A view that is not there is said to be missing rather than answered with an empty definition,
// which is what a save would then have wiped the real one out with.
func TestSourceRefusesAViewThatIsNotThere(t *testing.T) {
	s, ns := openTestFile(t)

	ref := dbdriver.SourceRef{Kind: dbdriver.SourceView, Name: "absent"}
	_, err := s.Source(context.Background(), ns, ref)
	if !errors.Is(err, domain.ErrUnknownDefinition) {
		t.Fatalf("Source = %v, want ErrUnknownDefinition", err)
	}
}

// There is no stored routine in a SQLite file — a function belongs to the program that opened it —
// so a definition for one says which thing is missing rather than coming back blank.
func TestSourceRefusesARoutine(t *testing.T) {
	s, ns := openTestFile(t)

	ref := dbdriver.SourceRef{Kind: dbdriver.SourceFunction, Name: "tax"}
	_, err := s.Source(context.Background(), ns, ref)
	if !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("Source = %v, want ErrUnsupported", err)
	}
}
