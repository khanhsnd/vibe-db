package jsonfile

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "nested", "snippets.json"))
}

func named(name, body string) domain.Snippet {
	return domain.Snippet{Name: name, Body: body}
}

func TestListOnMissingFileReturnsEmpty(t *testing.T) {
	library, err := newRepo(t).List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(library) != 0 {
		t.Errorf("List() = %d snippets, want 0", len(library))
	}
}

func TestSaveAnswersWithTheLibraryItLeavesBehind(t *testing.T) {
	repo := newRepo(t)

	library, err := repo.Save("", domain.Snippet{
		Name:        "recent orders",
		Description: "the last week",
		Body:        "SELECT * FROM orders WHERE placed > ${date}",
		Engine:      domain.EngineMySQL,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if len(library) != 1 {
		t.Fatalf("Save() = %d snippets, want the whole library back", len(library))
	}

	read, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(read) != 1 || read[0] != library[0] {
		t.Errorf("List() = %+v, want exactly what Save answered with", read)
	}
}

// The body is the reader's own text and travels verbatim: the placeholder notation, the newlines and
// the indentation are all what will be put back in the editor.
func TestSaveKeepsTheBodyByteForByte(t *testing.T) {
	repo := newRepo(t)
	body := "SELECT ${columns}\nFROM ${table}\n\tWHERE id = ${id};"

	if _, err := repo.Save("", named("shape", body)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	library, _ := repo.List()
	if library[0].Body != body {
		t.Errorf("Body = %q, want %q", library[0].Body, body)
	}
}

func TestSaveThatIsRefusedWritesNothing(t *testing.T) {
	repo := newRepo(t)
	if _, err := repo.Save("", named("orders", "SELECT 1")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := repo.Save("", named("", "SELECT 2")); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("Save() error = %v, want ErrInvalidName", err)
	}

	library, _ := repo.List()
	if len(library) != 1 || library[0].Name != "orders" {
		t.Errorf("library = %+v, want the refused save to have left the file as it was", library)
	}
}

func TestDeleteLeavesTheRest(t *testing.T) {
	repo := newRepo(t)
	_, _ = repo.Save("", named("a", "SELECT 1"))
	_, _ = repo.Save("", named("b", "SELECT 2"))

	left, err := repo.Delete("a")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(left) != 1 || left[0].Name != "b" {
		t.Fatalf("Delete() = %+v, want only b left", left)
	}

	read, _ := repo.List()
	if len(read) != 1 || read[0].Name != "b" {
		t.Errorf("List() = %+v, want the delete to have reached the file", read)
	}
}

// A file somebody has edited by hand is still read as a library, which is what keeps a snippet typed
// in at the bottom of it from being listed last.
func TestListSortsWhateverTheFileHeld(t *testing.T) {
	repo := newRepo(t)
	_, _ = repo.Save("", named("zebra", "SELECT 1"))
	_, _ = repo.Save("", named("apple", "SELECT 2"))

	library, _ := repo.List()
	if library[0].Name != "apple" {
		t.Errorf("first = %q, want the library sorted by name", library[0].Name)
	}
}
