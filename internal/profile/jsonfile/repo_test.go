package jsonfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func newRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "connections.json")
	return New(path), path
}

func sample(id, name string) domain.ConnectionProfile {
	return domain.ConnectionProfile{
		ID:           id,
		Name:         name,
		Engine:       domain.EngineMySQL,
		Host:         "127.0.0.1",
		Port:         3306,
		User:         "root",
		Database:     "shop",
		TLSMode:      domain.TLSPreferred,
		SavePassword: true,
	}
}

func TestListOnMissingFileReturnsEmpty(t *testing.T) {
	repo, _ := newRepo(t)

	profiles, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("List() = %d profiles, want 0", len(profiles))
	}
}

func TestSaveListGetDeleteRoundTrip(t *testing.T) {
	repo, _ := newRepo(t)

	if err := repo.Save(sample("a", "local")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(sample("b", "staging")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profiles, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("List() = %d profiles, want 2", len(profiles))
	}

	got, err := repo.Get("b")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "staging" || got.Engine != domain.EngineMySQL || got.Port != 3306 {
		t.Errorf("Get() = %+v, want the staging profile intact", got)
	}

	if err := repo.Delete("a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get("a"); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Errorf("Get() after Delete error = %v, want ErrProfileNotFound", err)
	}
}

func TestSaveReplacesExistingID(t *testing.T) {
	repo, _ := newRepo(t)

	if err := repo.Save(sample("a", "before")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(sample("a", "after")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profiles, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("List() = %d profiles, want 1", len(profiles))
	}
	if profiles[0].Name != "after" {
		t.Errorf("Name = %q, want %q", profiles[0].Name, "after")
	}
}

func TestDeleteMissingIDReports(t *testing.T) {
	repo, _ := newRepo(t)

	if err := repo.Delete("nope"); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Errorf("Delete() error = %v, want ErrProfileNotFound", err)
	}
}

func TestWriteNeverStoresPassword(t *testing.T) {
	repo, path := newRepo(t)

	if err := repo.Save(sample("a", "local")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(strings.ToLower(string(data)), "password\"") &&
		!strings.Contains(string(data), "savePassword") {
		t.Errorf("file contains a password field:\n%s", data)
	}
	if strings.Contains(string(data), "secret") {
		t.Errorf("file contains a secret value:\n%s", data)
	}
}

func TestWriteLeavesNoTempFilesBehind(t *testing.T) {
	repo, path := newRepo(t)

	if err := repo.Save(sample("a", "local")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want just connections.json", len(entries))
	}
}

func TestReadRejectsCorruptFileWithoutLosingIt(t *testing.T) {
	repo, path := newRepo(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := repo.List(); err == nil {
		t.Fatal("List() on corrupt file = nil error, want a parse error")
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "{not json" {
		t.Errorf("corrupt file was modified; got %q, err %v", data, err)
	}
}
