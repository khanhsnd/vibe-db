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
		Auth:         domain.AuthPassword,
		TLS:          domain.TLSOptions{Mode: domain.TLSPreferred},
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

func TestTunnelAndCertificatesSurviveARoundTrip(t *testing.T) {
	repo, _ := newRepo(t)

	saved := sample("a", "prod")
	saved.Environment = domain.EnvironmentProduction
	saved.Auth = domain.AuthAWSIAM
	saved.Region = "ap-southeast-1"
	saved.TLS = domain.TLSOptions{
		Mode:       domain.TLSVerifyFull,
		RootCert:   `C:\certs\rds-ca.pem`,
		ServerName: "shop.example.com",
	}
	saved.SSH = domain.SSHTunnel{
		Enabled:       true,
		Host:          "bastion",
		Port:          22,
		User:          "deploy",
		Auth:          domain.SSHKey,
		KeyPath:       `C:\keys\id_ed25519`,
		StrictHostKey: true,
	}

	if err := repo.Save(saved); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Get("a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Environment != saved.Environment || got.Auth != saved.Auth || got.Region != saved.Region {
		t.Errorf("Get() = %+v, want the environment, auth and region back", got)
	}
	if got.TLS != saved.TLS {
		t.Errorf("TLS = %+v, want %+v", got.TLS, saved.TLS)
	}
	if got.SSH != saved.SSH {
		t.Errorf("SSH = %+v, want %+v", got.SSH, saved.SSH)
	}
}

// A file written before there was anything but a password to name must still connect.
func TestProfileWithNoAuthReadsAsPassword(t *testing.T) {
	repo, path := newRepo(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	const old = `{"version":1,"profiles":[{"id":"a","name":"local","engine":"mysql",` +
		`"host":"127.0.0.1","port":3306,"user":"root","database":"shop",` +
		`"tlsMode":"preferred","savePassword":true}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := repo.Get("a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Auth != domain.AuthPassword {
		t.Errorf("Auth = %q, want %q", got.Auth, domain.AuthPassword)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Validate() = %v, want a profile that still connects", err)
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
