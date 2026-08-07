package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

var target = Target{ProfileID: "abc123", Database: "shop", Schema: "public"}

func TestFolderKeepsSchemasApart(t *testing.T) {
	store := New(t.TempDir())

	one, err := store.Folder(target)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	other, err := store.Folder(Target{ProfileID: "abc123", Database: "shop", Schema: "audit"})
	if err != nil {
		t.Fatalf("folder: %v", err)
	}

	if one == other {
		t.Fatalf("two schemas of one database share a folder: %s", one)
	}
	if filepath.Base(one) != "shop@public" {
		t.Fatalf("folder = %s, want it named for the namespace", filepath.Base(one))
	}
}

// A name that is a path must still be one directory: the whole point of `segment` is that nothing
// a profile or a database is called can address a folder outside the root.
func TestFolderIsAlwaysTwoLevelsInsideTheRoot(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	for _, escape := range []Target{
		{ProfileID: "../../etc", Database: "a/b"},
		{ProfileID: "..", Database: ".."},
		{ProfileID: ".", Database: ""},
	} {
		dir, err := store.Folder(escape)
		if err != nil {
			t.Fatalf("folder(%+v): %v", escape, err)
		}
		if parent := filepath.Dir(filepath.Dir(dir)); parent != root {
			t.Fatalf("folder(%+v) = %s, which is not two levels inside %s", escape, dir, root)
		}
	}
}

func TestPathRefusesANameThatIsAPath(t *testing.T) {
	store := New(t.TempDir())

	for _, name := range []string{"", "../escape", `sub\file`, "a/b"} {
		if _, err := store.Path(target, name); err == nil {
			t.Fatalf("Path accepted %q", name)
		} else if !errors.As(err, &domain.NameError{}) {
			t.Fatalf("Path(%q) = %v, want a NameError", name, err)
		}
	}
}

func TestCreateWritesAHeaderThatReadsBack(t *testing.T) {
	store := New(t.TempDir())
	created := time.Date(2026, 4, 29, 11, 3, 21, 0, time.UTC)

	file, err := store.Create(target, "20260429110321", Header{
		Server:   "fim-dev-postgres",
		Database: "shop/public",
		Created:  created,
		Comment:  "before the migration\nsecond line",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := file.WriteString("SELECT 1;\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	file.Close()

	header, err := ReadHeader(file.Name())
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if header.Server != "fim-dev-postgres" || header.Database != "shop/public" {
		t.Fatalf("header = %+v", header)
	}
	if header.Comment != "before the migration\nsecond line" {
		t.Fatalf("comment = %q, want the newline kept", header.Comment)
	}
	if !header.Created.Equal(created) {
		t.Fatalf("created = %v, want %v", header.Created, created)
	}
}

// A second backup inside one second must not be the first one's file with new contents.
func TestCreateNeverWritesOverOne(t *testing.T) {
	store := New(t.TempDir())

	first, err := store.Create(target, "20260429110321", Header{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	first.Close()

	second, err := store.Create(target, "20260429110321", Header{})
	if err != nil {
		t.Fatalf("create again: %v", err)
	}
	second.Close()

	if first.Name() == second.Name() {
		t.Fatalf("two backups share a path: %s", first.Name())
	}
	if Name(second.Name()) != "20260429110321-2" {
		t.Fatalf("second name = %s", Name(second.Name()))
	}
}

func TestListIsNewestFirstAndIgnoresWhatIsNotABackup(t *testing.T) {
	store := New(t.TempDir())

	dir, err := store.Folder(target)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for at, name := range []string{"20260101000000", "20260202000000"} {
		file, err := store.Create(target, name, Header{Comment: name})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		file.Close()
		// The list orders by modification time, which is what a file dropped in from elsewhere has
		// and a name it did not come with does not.
		when := time.Date(2026, 1, 1+at, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(file.Name(), when, when); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	files, err := store.List(target)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("listed %d files, want 2", len(files))
	}
	if files[0].Name != "20260202000000" {
		t.Fatalf("first = %s, want the newest", files[0].Name)
	}
	if files[0].Comment != "20260202000000" {
		t.Fatalf("comment = %q, want it read from the header", files[0].Comment)
	}
}

// A file the reader dropped in by hand still restores, so it still lists — with no comment rather
// than not at all.
func TestListKeepsAFileWithNoHeader(t *testing.T) {
	store := New(t.TempDir())

	dir, err := store.Folder(target)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "handmade.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	files, err := store.List(target)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 || files[0].Name != "handmade" || files[0].Comment != "" {
		t.Fatalf("files = %+v", files)
	}
}

func TestRemove(t *testing.T) {
	store := New(t.TempDir())

	file, err := store.Create(target, "20260429110321", Header{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	file.Close()

	if err := store.Remove(target, "20260429110321"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(file.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file is still there: %v", err)
	}
}

func TestCopyNamesTheCopyAndNeverOverwrites(t *testing.T) {
	store := New(t.TempDir())

	file, err := store.Create(target, "20260429110321", Header{Comment: "original"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	file.WriteString("SELECT 1;\n")
	file.Close()

	first, err := store.Copy(target, "20260429110321")
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if first != "20260429110321-copy" {
		t.Fatalf("copy = %s", first)
	}

	second, err := store.Copy(target, "20260429110321")
	if err != nil {
		t.Fatalf("copy again: %v", err)
	}
	if second != "20260429110321-copy-2" {
		t.Fatalf("second copy = %s", second)
	}

	// The copy is the file, header and all — it is a backup, not a note about one.
	header, err := ReadHeader(mustPath(t, store, first))
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if header.Comment != "original" {
		t.Fatalf("comment = %q", header.Comment)
	}

	body, err := os.ReadFile(mustPath(t, store, second))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := "SELECT 1;\n"; string(body[len(body)-len(want):]) != want {
		t.Fatalf("the copy lost the statements: %q", body)
	}
}

func TestRenameRefusesANameAlreadyTaken(t *testing.T) {
	store := New(t.TempDir())

	for _, name := range []string{"one", "two"} {
		file, err := store.Create(target, name, Header{})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		file.Close()
	}

	if err := store.Rename(target, "one", "two"); err == nil {
		t.Fatal("rename wrote over an existing backup")
	} else if !errors.As(err, &domain.ValidationError{}) {
		t.Fatalf("rename = %v, want a ValidationError naming the box", err)
	}

	if err := store.Rename(target, "one", "three"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	files, err := store.List(target)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := []string{}
	for _, file := range files {
		names = append(names, file.Name)
	}
	if len(names) != 2 {
		t.Fatalf("names = %v, want two of them", names)
	}
	for _, name := range names {
		if name == "one" {
			t.Fatalf("the old name is still there: %v", names)
		}
	}
}

// The header is what the restore dialog reads its table list off, so it has to survive the round
// trip whole — a name that is not an identifier included.
func TestHeaderCarriesWhatTheFileHolds(t *testing.T) {
	store := New(t.TempDir())

	file, err := store.Create(target, "20260429110321", Header{
		Tables: []string{"users", `odd "name"`},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	file.Close()

	files, err := store.List(target)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("listed %d files, want 1", len(files))
	}
	if got := files[0].Tables; len(got) != 2 || got[0] != "users" || got[1] != `odd "name"` {
		t.Fatalf("tables = %q", got)
	}
}

func TestOnlyKeepsThePartsTheTablesNamed(t *testing.T) {
	var written strings.Builder
	written.WriteString("-- vibe-db-backup {\"comment\":\"x\"}\n\n")
	for _, table := range []string{"users", `odd "name"`, "orders"} {
		if err := MarkTable(&written, table); err != nil {
			t.Fatalf("mark: %v", err)
		}
		fmt.Fprintf(&written, "INSERT INTO %s VALUES (1);\n\n", table)
	}
	text := written.String()

	// Empty is every table, which is what a restore the reader narrowed nothing of asks for.
	if Only(text, nil) != text {
		t.Fatal("Only(nil) did not answer with the whole file")
	}

	kept := Only(text, []string{"users", "orders"})
	if !strings.Contains(kept, "INSERT INTO users") || !strings.Contains(kept, "INSERT INTO orders") {
		t.Fatalf("a chosen table was dropped: %q", kept)
	}
	if strings.Contains(kept, `INSERT INTO odd "name"`) {
		t.Fatalf("an unchosen table was kept: %q", kept)
	}
	// The header runs whatever was picked: it is what the file says about itself, and it is above
	// every marker there is.
	if !strings.HasPrefix(kept, marker) {
		t.Fatalf("the header was dropped: %q", kept)
	}

	// A name that is not an identifier is matched exactly, the marker having carried it as JSON.
	if odd := Only(text, []string{`odd "name"`}); !strings.Contains(odd, `INSERT INTO odd "name"`) {
		t.Fatalf("an odd name did not match its own marker: %q", odd)
	}
}

// A file this app did not write has no markers to narrow by. It still restores — whole — which is
// the same answer a listing gives it.
func TestOnlyLeavesAFileWithNoMarkersAlone(t *testing.T) {
	text := "SELECT 1;\nSELECT 2;\n"

	if got := Only(text, []string{"users"}); got != text {
		t.Fatalf("Only = %q, want the file untouched", got)
	}
}

func mustPath(t *testing.T, store *Store, name string) string {
	t.Helper()
	path, err := store.Path(target, name)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	return path
}
