package dbtool

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// executable makes a file the operating system will agree is runnable, which on Windows means the
// extension and everywhere else means the mode.
func executable(t *testing.T, dir, name string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("stand-in"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func locator(t *testing.T) *Locator {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "tools.json"))
}

// Choosing one program sets that one and **nothing else**, even where the other is sitting right
// beside it: a path nobody chose is a path nobody thinks to check when the wrong version runs.
func TestChoosingOneProgramLeavesTheOtherAlone(t *testing.T) {
	tools := locator(t)
	dir := t.TempDir()

	dump := executable(t, dir, "mongodump")
	executable(t, dir, "mongorestore")

	kit, err := tools.Choose(domain.EngineMongo, DumpProgram, dump)
	if err != nil {
		t.Fatal(err)
	}

	if !kit.Dump.Found() || kit.Dump.Path != dump {
		t.Errorf("the picked program is %+v, wanted %s", kit.Dump, dump)
	}
	if kit.Restore.Chosen != "" {
		t.Errorf("mongorestore was filled in by nobody: %+v", kit.Restore)
	}
}

// Each is remembered on its own, so the two can live in two places.
func TestTheTwoProgramsAreRememberedApart(t *testing.T) {
	tools := locator(t)
	first, second := t.TempDir(), t.TempDir()

	dump := executable(t, first, "mongodump")
	restore := executable(t, second, "mongorestore")

	if _, err := tools.Choose(domain.EngineMongo, DumpProgram, dump); err != nil {
		t.Fatal(err)
	}
	kit, err := tools.Choose(domain.EngineMongo, RestoreProgram, restore)
	if err != nil {
		t.Fatal(err)
	}

	if kit.Dump.Path != dump {
		t.Errorf("mongodump moved when mongorestore was chosen: %s", kit.Dump.Path)
	}
	if kit.Restore.Path != restore {
		t.Errorf("mongorestore is %s, wanted %s", kit.Restore.Path, restore)
	}
}

// The one state worth drawing differently: the file the reader picked has gone, which is a
// different sentence from never having picked one — and quietly running a different copy off the
// PATH instead would be the worst of the three.
func TestAChosenProgramThatHasGoneIsNamedRatherThanForgotten(t *testing.T) {
	tools := locator(t)
	dir := t.TempDir()

	dump := executable(t, dir, "mongodump")
	if _, err := tools.Choose(domain.EngineMongo, DumpProgram, dump); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dump); err != nil {
		t.Fatal(err)
	}

	kit, err := tools.Kit(domain.EngineMongo)
	if err != nil {
		t.Fatal(err)
	}

	if kit.Dump.Found() {
		t.Error("a program that is not there was reported as found")
	}
	if kit.Dump.Chosen != dump {
		t.Errorf("the file the reader picked is not named: %+v", kit.Dump)
	}
}

// Forgetting is the way back from having pointed at the wrong copy.
func TestForgettingAProgramGoesBackToThePath(t *testing.T) {
	tools := locator(t)
	dir := t.TempDir()

	if _, err := tools.Choose(domain.EngineMongo, DumpProgram, executable(t, dir, "mongodump")); err != nil {
		t.Fatal(err)
	}

	kit, err := tools.Choose(domain.EngineMongo, DumpProgram, "")
	if err != nil {
		t.Fatal(err)
	}
	if kit.Dump.Chosen != "" {
		t.Errorf("the chosen path outlived being forgotten: %+v", kit.Dump)
	}
}

func TestAnEngineWithNoToolkitIsRefusedByTheLocator(t *testing.T) {
	tools := locator(t)

	if _, err := tools.Kit(domain.EngineRedis); err == nil {
		t.Error("Redis was handed a toolkit it does not have")
	}
	if _, err := tools.Choose(domain.EngineSQLite, DumpProgram, "anything"); err == nil {
		t.Error("SQLite was allowed to remember a program it never runs")
	}
}
