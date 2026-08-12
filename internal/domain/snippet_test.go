package domain

import (
	"errors"
	"testing"
)

func snippet(name, body string) Snippet {
	return Snippet{Name: name, Body: body}
}

func names(library []Snippet) []string {
	out := make([]string, 0, len(library))
	for _, one := range library {
		out = append(out, one.Name)
	}
	return out
}

func TestSaveSnippetKeepsTheLibrarySortedByName(t *testing.T) {
	library, err := SaveSnippet(nil, "", snippet("stale sessions", "SELECT 1"))
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}
	library, err = SaveSnippet(library, "", snippet("Active orders", "SELECT 2"))
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}

	if got := names(library); got[0] != "Active orders" || got[1] != "stale sessions" {
		t.Errorf("names = %v, want them folded and sorted", got)
	}
}

func TestSaveSnippetReplacesTheEntryOfTheSameName(t *testing.T) {
	library, _ := SaveSnippet(nil, "", snippet("orders", "SELECT 1"))
	library, err := SaveSnippet(library, "orders", snippet("ORDERS", "SELECT 2"))
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}

	if len(library) != 1 {
		t.Fatalf("library = %v, want one entry — a name is what identifies a snippet", names(library))
	}
	if library[0].Body != "SELECT 2" || library[0].Name != "ORDERS" {
		t.Errorf("kept = %+v, want the newer name and body", library[0])
	}
}

// A rename has to let go of the old entry in the same write, or the library ends up holding both.
func TestSaveSnippetUnderANewNameDropsTheOldOne(t *testing.T) {
	library, _ := SaveSnippet(nil, "", snippet("orders", "SELECT 1"))
	library, err := SaveSnippet(library, "orders", snippet("recent orders", "SELECT 1"))
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}

	if got := names(library); len(got) != 1 || got[0] != "recent orders" {
		t.Errorf("names = %v, want the renamed entry alone", got)
	}
}

func TestSaveSnippetTrimsWhatItKeeps(t *testing.T) {
	library, err := SaveSnippet(nil, "", Snippet{
		Name:        "  orders  ",
		Description: "  what shipped  ",
		Body:        "\n\nSELECT 1\n\n",
	})
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}

	want := Snippet{Name: "orders", Description: "what shipped", Body: "SELECT 1"}
	if library[0] != want {
		t.Errorf("kept = %+v, want %+v", library[0], want)
	}
}

func TestSaveSnippetRefusesOneWithNoName(t *testing.T) {
	if _, err := SaveSnippet(nil, "", snippet("   ", "SELECT 1")); !errors.Is(err, ErrInvalidName) {
		t.Errorf("error = %v, want ErrInvalidName", err)
	}
}

func TestSaveSnippetRefusesOneWithNoBody(t *testing.T) {
	if _, err := SaveSnippet(nil, "", snippet("orders", " \n ")); !errors.Is(err, ErrEmptySnippet) {
		t.Errorf("error = %v, want ErrEmptySnippet", err)
	}
}

func TestSaveSnippetRefusesAnEngineThisAppDoesNotKnow(t *testing.T) {
	one := Snippet{Name: "orders", Body: "SELECT 1", Engine: "cassandra"}
	if _, err := SaveSnippet(nil, "", one); !errors.Is(err, ErrInvalidProfile) {
		t.Errorf("error = %v, want a validation failure", err)
	}
}

func TestSaveSnippetAcceptsNoEngineAtAll(t *testing.T) {
	if _, err := SaveSnippet(nil, "", snippet("orders", "SELECT 1")); err != nil {
		t.Errorf("SaveSnippet() error = %v, want a snippet offered on every engine to be allowed", err)
	}
}

func TestDeleteSnippetIgnoresTheCaseOfTheName(t *testing.T) {
	library, _ := SaveSnippet(nil, "", snippet("Orders", "SELECT 1"))
	if left := DeleteSnippet(library, "orders"); len(left) != 0 {
		t.Errorf("left = %v, want the snippet gone", names(left))
	}
}

func TestDeleteSnippetIsNotAFailureForANameThatIsNotThere(t *testing.T) {
	library, _ := SaveSnippet(nil, "", snippet("orders", "SELECT 1"))
	if left := DeleteSnippet(library, "nothing"); len(left) != 1 {
		t.Errorf("left = %v, want the library untouched", names(left))
	}
}
