package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	dbdriver "github.com/khanhsnd/vibe-db/internal/driver"
)

// A file engine is the one engine a test can hold the whole of, so these run against a real
// database — which is what a transaction has to be proved against. What is really being proved is
// the *shared* half: `sqlkit.Txn` holds the connection and `sqlkit.Data` and `sqlkit.DDL` ask it
// before they reach for a pool, so a row written inside one and read back through the ordinary read
// is the whole plumbing working end to end.

func countUsers(t *testing.T, s *session, ns dbdriver.Namespace) int64 {
	t.Helper()

	rows, err := s.TableRows(context.Background(), dbdriver.TablePage{
		Namespace: ns,
		Table:     "users",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("TableRows: %v", err)
	}
	return rows.Total
}

func insertUser(t *testing.T, s *session, ns dbdriver.Namespace, id, email string) {
	t.Helper()

	err := s.InsertRow(context.Background(), dbdriver.RowInsert{
		Namespace: ns,
		Table:     "users",
		Values:    map[string]*string{"id": &id, "email": &email},
	})
	if err != nil {
		t.Fatalf("InsertRow: %v", err)
	}
}

func TestRollbackUndoesWhatWasWrittenInsideTheTransaction(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	before := countUsers(t, s, ns)

	if err := s.Begin(ctx, ns); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if !s.InTransaction() {
		t.Fatal("InTransaction = false right after Begin")
	}

	insertUser(t, s, ns, "3", "cat@example.com")

	// The read has to see it: an uncommitted row exists on the transaction's connection and
	// nowhere else, so a grid reading off the pool would be showing the reader nothing happened.
	if got := countUsers(t, s, ns); got != before+1 {
		t.Fatalf("rows inside the transaction = %d, want %d", got, before+1)
	}

	if err := s.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if s.InTransaction() {
		t.Fatal("InTransaction = true after Rollback")
	}
	if got := countUsers(t, s, ns); got != before {
		t.Fatalf("rows after Rollback = %d, want %d", got, before)
	}
}

func TestCommitKeepsWhatWasWrittenInsideTheTransaction(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	before := countUsers(t, s, ns)

	if err := s.Begin(ctx, ns); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	insertUser(t, s, ns, "4", "dot@example.com")

	if err := s.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if s.InTransaction() {
		t.Fatal("InTransaction = true after Commit")
	}
	if got := countUsers(t, s, ns); got != before+1 {
		t.Fatalf("rows after Commit = %d, want %d", got, before+1)
	}
}

// A schema change goes inside the transaction like everything else, which is the one rule this
// feature has. SQLite's DDL is transactional, so rolling one back takes the column with it.
func TestASchemaChangeGoesInsideTheTransaction(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	if err := s.Begin(ctx, ns); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	spec := dbdriver.ColumnSpec{Name: "nickname", Type: "TEXT", Nullable: true}
	if err := s.AddColumn(ctx, ns, "users", spec); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}
	if err := s.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	structure, err := s.TableStructure(ctx, ns, "users")
	if err != nil {
		t.Fatalf("TableStructure: %v", err)
	}
	for _, column := range structure.Columns {
		if column.Name == "nickname" {
			t.Fatal("the rolled-back column is still on the table")
		}
	}
}

func TestBeginRefusesASecondTransaction(t *testing.T) {
	s, ns := openTestFile(t)
	ctx := context.Background()

	if err := s.Begin(ctx, ns); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := s.Begin(ctx, ns); !errors.Is(err, domain.ErrTransactionActive) {
		t.Fatalf("second Begin = %v, want ErrTransactionActive", err)
	}
}

func TestCommitAndRollbackRefuseWhenNothingIsOpen(t *testing.T) {
	s, _ := openTestFile(t)
	ctx := context.Background()

	if err := s.Commit(ctx); !errors.Is(err, domain.ErrNoTransaction) {
		t.Fatalf("Commit = %v, want ErrNoTransaction", err)
	}
	if err := s.Rollback(ctx); !errors.Is(err, domain.ErrNoTransaction) {
		t.Fatalf("Rollback = %v, want ErrNoTransaction", err)
	}
}
