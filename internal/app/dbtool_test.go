package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/dbtool"
	"github.com/khanhsnd/vibe-db/internal/domain"
)

// The log opens with the command line, so it has to be one a reader can paste into a terminal.
func TestCommandTextQuotesWhatAShellWouldSplit(t *testing.T) {
	text := commandText(dbtool.Command{
		Program: dbtool.Program{Name: "mongodump", Path: `C:\Program Files\Tools\mongodump.exe`},
		Args:    []string{"--db=shop", `--archive=C:\My Dumps\shop.archive`},
	})

	want := `"C:\Program Files\Tools\mongodump.exe" --db=shop "--archive=C:\My Dumps\shop.archive"`
	if text != want {
		t.Errorf("got  %s\nwant %s", text, want)
	}
}

func TestWrapErrorNamesTheProgramThatIsMissing(t *testing.T) {
	wrapped := wrapError(domain.ToolNotFoundError{Tool: "mongorestore"})

	var wire wireError
	if !errors.As(wrapped, &wire) {
		t.Fatalf("not a wire error: %v", wrapped)
	}
	if wire.Code != CodeToolNotFound {
		t.Errorf("code is %s, wanted %s", wire.Code, CodeToolNotFound)
	}
	if wire.Field != "mongorestore" || !strings.Contains(wire.Message, "mongorestore") {
		t.Errorf("the reader is not told which program: %+v", wire)
	}
}

// A program that ran and refused answers in its own last words, exactly as a rejected statement
// answers in the server's.
func TestWrapErrorKeepsTheProgramsOwnWording(t *testing.T) {
	wrapped := wrapError(domain.ToolFailedError{
		Tool:    "pg_restore",
		Status:  1,
		Message: "error: could not execute query: relation already exists",
	})

	var wire wireError
	if !errors.As(wrapped, &wire) {
		t.Fatalf("not a wire error: %v", wrapped)
	}
	if wire.Code != CodeToolFailed {
		t.Errorf("code is %s, wanted %s", wire.Code, CodeToolFailed)
	}
	if !strings.Contains(wire.Message, "relation already exists") {
		t.Errorf("the program's own wording was lost: %q", wire.Message)
	}
	if !strings.Contains(wire.Detail, "exited 1") {
		t.Errorf("the exit status is not in the detail: %q", wire.Detail)
	}
}

// A program that ended badly without saying anything is still not an empty banner.
func TestWrapErrorFillsInASilentFailure(t *testing.T) {
	wrapped := wrapError(domain.ToolFailedError{Tool: "mysqldump", Status: 2})

	var wire wireError
	if !errors.As(wrapped, &wire) {
		t.Fatalf("not a wire error: %v", wrapped)
	}
	if wire.Message == "" {
		t.Error("a failure with no message reaches the reader as a blank banner")
	}
}
