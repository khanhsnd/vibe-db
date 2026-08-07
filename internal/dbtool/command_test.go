package dbtool

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func kitFor(t *testing.T, engine domain.Engine) Kit {
	t.Helper()

	toolkit, ok := engine.Toolkit()
	if !ok {
		t.Fatalf("%s has no toolkit", engine)
	}
	return Kit{
		Toolkit: toolkit,
		Dump:    Program{Name: toolkit.Dump, Path: "/usr/bin/" + toolkit.Dump},
		Restore: Program{Name: toolkit.Restore, Path: "/usr/bin/" + toolkit.Restore},
	}
}

func request(engine domain.Engine, password string) Request {
	return Request{
		Profile: domain.ConnectionProfile{
			Engine: engine,
			Host:   "db.example.com",
			Port:   engine.DefaultPort(),
			User:   "reader",
			TLS:    domain.TLSOptions{Mode: domain.TLSPreferred},
		},
		Password:  password,
		Database:  "shop",
		Path:      filepath.Join(t0dir(), "out"),
		Structure: true,
		Data:      true,
	}
}

// t0dir keeps the fixture paths off any real disk while still being a path.
func t0dir() string { return string(filepath.Separator) + "tmp" }

func has(args []string, want string) bool {
	return slices.Contains(args, want)
}

// The one rule that matters more than any argument being right: a command line is readable by
// every process on the machine, so the password must never be on one.
func TestSecretIsNeverAnArgument(t *testing.T) {
	const password = "s3cr3t-p@ss"

	for _, engine := range []domain.Engine{domain.EngineMySQL, domain.EnginePostgres, domain.EngineMongo} {
		kit := kitFor(t, engine)

		dump, err := Dump(kit, request(engine, password))
		if err != nil {
			t.Fatalf("%s dump: %v", engine, err)
		}

		line := strings.Join(append(dump.Args, dump.Env...), " ")
		if strings.Contains(line, password) {
			t.Errorf("%s: the password reached the command line or the environment: %s", engine, line)
		}
		if !strings.Contains(dump.Secret.Body, password) {
			t.Errorf("%s: the password is not in the credential file either", engine)
		}
	}
}

// mysqldump reads its defaults file before it parses anything else and refuses it anywhere but
// first, which is the one thing about this pair that is not obvious from the flag's name.
func TestMySQLDefaultsFileComesFirst(t *testing.T) {
	kit := kitFor(t, domain.EngineMySQL)

	command, err := Dump(kit, request(domain.EngineMySQL, "pw"))
	if err != nil {
		t.Fatal(err)
	}

	cleanup, err := withSecret(&command)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if len(command.Args) == 0 || !strings.HasPrefix(command.Args[0], "--defaults-file=") {
		t.Fatalf("--defaults-file is not the first argument: %v", command.Args)
	}

	path := strings.TrimPrefix(command.Args[0], "--defaults-file=")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the credential file was not written: %v", err)
	}
	if !strings.HasPrefix(string(body), "[client]") {
		t.Errorf("the defaults file is not a client group: %q", body)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the credential file outlived the run: %v", err)
	}
}

func TestMySQLDumpNarrowsByTable(t *testing.T) {
	kit := kitFor(t, domain.EngineMySQL)

	req := request(domain.EngineMySQL, "")
	req.Include = []string{"orders", "customers"}
	req.Data = false

	command, err := Dump(kit, req)
	if err != nil {
		t.Fatal(err)
	}

	if !has(command.Args, "--no-data") {
		t.Errorf("structure only did not reach mysqldump: %v", command.Args)
	}
	// The database, then the tables, and in that order — mysqldump takes them positionally.
	tail := command.Args[len(command.Args)-3:]
	if tail[0] != "shop" || tail[1] != "orders" || tail[2] != "customers" {
		t.Errorf("the namespace and its tables are not the last arguments: %v", tail)
	}
}

// pg_dump is told about TLS and its password through the environment alone, which is what keeps
// both off the command line.
func TestPostgresCarriesItsSecretInTheEnvironment(t *testing.T) {
	kit := kitFor(t, domain.EnginePostgres)

	req := request(domain.EnginePostgres, "pa:ss\\word")
	req.Schema = "public"
	req.Include = []string{"orders"}

	command, err := Dump(kit, req)
	if err != nil {
		t.Fatal(err)
	}

	if command.Secret.EnvVar != "PGPASSFILE" {
		t.Errorf("the password is not a pgpass file: %+v", command.Secret)
	}
	// The separator and the backslash are the two characters a pgpass line gives a meaning to.
	if !strings.Contains(command.Secret.Body, `pa\:ss\\word`) {
		t.Errorf("the pgpass line is not escaped: %q", command.Secret.Body)
	}
	if !has(command.Args, "--table=public.orders") {
		t.Errorf("the table is not qualified by its schema: %v", command.Args)
	}
	if !has(command.Args, "--format=custom") {
		t.Errorf("the dump is not the custom format, so pg_restore could not be selective: %v", command.Args)
	}
	if !has(command.Env, "PGSSLMODE=prefer") {
		t.Errorf("the TLS mode did not reach libpq: %v", command.Env)
	}
}

func TestPostgresRestoreAlwaysPairsCleanWithIfExists(t *testing.T) {
	kit := kitFor(t, domain.EnginePostgres)

	req := request(domain.EnginePostgres, "")
	req.Drop = true

	command, err := Restore(kit, req)
	if err != nil {
		t.Fatal(err)
	}

	if !has(command.Args, "--clean") || !has(command.Args, "--if-exists") {
		t.Errorf("--clean without --if-exists fails on the first object that was not there: %v", command.Args)
	}
	if command.Args[len(command.Args)-1] != req.Path {
		t.Errorf("pg_restore takes its file last and without a flag: %v", command.Args)
	}
}

// A collection is narrowed by what is left out, because that is the only narrowing mongodump has.
func TestMongoDumpExcludesRatherThanIncludes(t *testing.T) {
	kit := kitFor(t, domain.EngineMongo)

	req := request(domain.EngineMongo, "pw")
	req.Exclude = []string{"logs", "sessions"}
	req.Compress = true

	command, err := Dump(kit, req)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"--db=shop",
		"--archive=" + req.Path,
		"--gzip",
		"--excludeCollection=logs",
		"--excludeCollection=sessions",
		// Without this these programs authenticate against the database being dumped, while the
		// app's own driver authenticates against admin.
		"--authenticationDatabase=admin",
	} {
		if !has(command.Args, want) {
			t.Errorf("missing %s: %v", want, command.Args)
		}
	}
}

// Whether an archive is compressed is read off the file rather than off a box the reader has to
// remember the answer to a month later.
func TestMongoRestoreReadsGzipOffTheFile(t *testing.T) {
	kit := kitFor(t, domain.EngineMongo)
	dir := t.TempDir()

	plain := filepath.Join(dir, "plain.archive")
	if err := os.WriteFile(plain, []byte("not gzip at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	zipped := filepath.Join(dir, "zipped.archive")
	if err := os.WriteFile(zipped, []byte{0x1f, 0x8b, 0x08, 0x00}, 0o600); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]bool{plain: false, zipped: true} {
		req := request(domain.EngineMongo, "")
		req.Path = path

		command, err := Restore(kit, req)
		if err != nil {
			t.Fatal(err)
		}
		if got := has(command.Args, "--gzip"); got != want {
			t.Errorf("%s: --gzip is %v, wanted %v", filepath.Base(path), got, want)
		}
	}
}

func TestEngineWithoutToolsIsRefusedRatherThanRun(t *testing.T) {
	for _, engine := range []domain.Engine{domain.EngineSQLite, domain.EngineSQLServer, domain.EngineOracle, domain.EngineRedis} {
		if _, err := Dump(Kit{}, request(engine, "")); err == nil {
			t.Errorf("%s: a dump was built for an engine that ships no such program", engine)
		}
	}
}
