package dbtool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// The programs this package runs are not on every machine and never will be, so what is tested
// here is **the running of one** — the lines coming out in order, the exit status becoming a
// failure the reader can read, a stop killing the child, and the credential file being gone
// afterwards. The test binary stands in for the program: it is the one executable that is
// certainly here.

const helperVar = "VIBE_DB_TOOL_HELPER"

// TestHelperProcess is not a test. It is what the tests below run *as* a child process, and it
// returns immediately when it is reached as part of an ordinary run.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv(helperVar) {
	case "":
		return

	case "talks":
		fmt.Fprintln(os.Stdout, "first line")
		fmt.Fprintln(os.Stderr, "second line")
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stderr, "third line")
		os.Exit(0)

	case "refuses":
		fmt.Fprintln(os.Stderr, "Failed: error connecting to db server: no reachable servers")
		os.Exit(3)

	case "waits":
		time.Sleep(time.Minute)
		os.Exit(0)

	case "reads-secret":
		// Whatever the credential file turned out to be called, the child can read it — which is
		// the whole point of writing one — and it says so by printing what is in it.
		for _, arg := range os.Args {
			if path, found := strings.CutPrefix(arg, "--config="); found {
				body, err := os.ReadFile(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, "cannot read the credential file:", err)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stdout, "read:"+strings.TrimSpace(string(body)))
				fmt.Fprintln(os.Stdout, "at:"+path)
				os.Exit(0)
			}
		}
		fmt.Fprintln(os.Stderr, "no credential file was named")
		os.Exit(1)
	}
}

// helper is the test binary, standing in for one of the programs. The trailing `--` is what stops
// the testing package's own flag parser, so whatever `withSecret` appends after it reaches the
// child as an ordinary argument instead of being rejected as an unknown flag.
func helper(mode string) Command {
	return Command{
		Program: Program{Name: "helper", Path: os.Args[0]},
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     []string{helperVar + "=" + mode},
	}
}

func collect(t *testing.T, command Command) ([]string, error) {
	t.Helper()

	var lines []string
	err := Run(context.Background(), command, func(text string) { lines = append(lines, text) })
	return lines, err
}

func TestRunCarriesEveryLineInOrder(t *testing.T) {
	lines, err := collect(t, helper("talks"))
	if err != nil {
		t.Fatalf("a program that ended well was reported as a failure: %v", err)
	}

	// Both streams, interleaved as they were written, and the blank line dropped.
	want := []string{"first line", "second line", "third line"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, wanted %d: %q", len(lines), len(want), lines)
	}
	for at, text := range want {
		if lines[at] != text {
			t.Errorf("line %d is %q, wanted %q", at, lines[at], text)
		}
	}
}

// A program that ran and refused answers in its own last words, which are the only useful ones.
func TestRunReportsWhatTheProgramSaid(t *testing.T) {
	lines, err := collect(t, helper("refuses"))

	var failed domain.ToolFailedError
	if !errors.As(err, &failed) {
		t.Fatalf("a non-zero exit is not a ToolFailedError: %v", err)
	}
	if failed.Status != 3 {
		t.Errorf("the exit status is %d, wanted 3", failed.Status)
	}
	if !strings.Contains(failed.Message, "no reachable servers") {
		t.Errorf("the program's own last words were lost: %q", failed.Message)
	}
	if len(lines) == 0 {
		t.Error("a failed run carried none of its output back")
	}
}

// Stopping a run kills the child, and reads as cancelled rather than as a program that refused.
func TestStoppingARunKillsTheProgram(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- Run(ctx, helper("waits"), func(string) {}) }()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, domain.ErrQueryCancelled) {
			t.Errorf("a stopped run is %v, wanted a cancellation", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the child outlived its context")
	}
}

// The credential file exists for exactly as long as the run, and the child can read it while it
// does. Both halves matter: a file the program cannot open is a login that fails, and one left
// behind is a password on the disk.
func TestTheCredentialFileLivesExactlyAsLongAsTheRun(t *testing.T) {
	command := helper("reads-secret")
	command.Secret = Secret{Body: "password: \"hunter2\"", Extension: ".yaml", Arg: "--config=%s"}

	lines, err := collect(t, command)
	if err != nil {
		t.Fatalf("the child could not use the credential file: %v (%q)", err, lines)
	}

	var read, at string
	for _, line := range lines {
		if value, found := strings.CutPrefix(line, "read:"); found {
			read = value
		}
		if value, found := strings.CutPrefix(line, "at:"); found {
			at = value
		}
	}

	if read != `password: "hunter2"` {
		t.Errorf("the child read %q out of the credential file", read)
	}
	if at == "" {
		t.Fatal("the child was never told where the credential file was")
	}
	if _, err := os.Stat(at); !os.IsNotExist(err) {
		t.Errorf("the credential file outlived the run: %v", err)
	}
}

func TestRunRefusesAProgramThatIsNotHere(t *testing.T) {
	err := Run(context.Background(), Command{Program: Program{Name: "mongodump"}}, func(string) {})

	var missing domain.ToolNotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("a missing program is %v, wanted a ToolNotFoundError", err)
	}
	if missing.Tool != "mongodump" {
		t.Errorf("the missing program is named %q", missing.Tool)
	}
}
