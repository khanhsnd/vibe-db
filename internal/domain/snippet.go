package domain

import (
	"errors"
	"slices"
	"strings"
)

// ErrEmptySnippet is a snippet saved with nothing in it. It is its own sentinel rather than
// ErrEmptyStatement because nothing was being run: a library entry with no body is a name the reader
// would reach for and get nothing back from, and the box to fill in is in front of them.
var ErrEmptySnippet = errors.New("snippet has no body")

// Snippet is a statement the reader has named and kept. What separates it from a HistoryEntry is who
// decided it was worth keeping: history is everything that ran, and a snippet is the one somebody
// meant to come back to.
//
// So it carries no outcome, no namespace and no connection. A statement worth naming is one worth
// running in more than one place, and pinning it to the database it was first typed against would
// make a library of one connection's statements rather than of the reader's.
//
// Engine is the only narrowing there is, and "" is every engine. It exists because the library is
// offered by the editor's own completion: `SELECT … LIMIT 10` in front of a Redis tab is noise, and
// a document store's commands are not SQL at all.
type Snippet struct {
	Name string
	// Description is what the name does not say — a sentence the library lists beside it. It is
	// never required: a snippet called `orders by month` has already described itself.
	Description string
	// Body is the text put in the editor, verbatim. `${…}` in it marks a field the caret jumps
	// between with Tab, which is the notation the built-in snippets already use — see
	// docs/editor.md. Nothing here reads it: what a body *means* is the engine's, and a snippet is
	// not a statement until the reader runs it.
	Body   string
	Engine Engine
}

// SaveSnippet puts one in the library and answers with the library it makes.
//
// A snippet is identified by its **name**, folded, that being what the reader types to reach one:
// saving under a name the library already holds replaces that entry instead of standing a second
// beside it, which is what makes editing one and saving it a single gesture. `previous` is the name
// it was known by — "" for a snippet that is new — so a rename lets go of the old entry in the same
// write, rather than in a second call that could fail on its own and leave two.
func SaveSnippet(library []Snippet, previous string, one Snippet) ([]Snippet, error) {
	one.Name = strings.TrimSpace(one.Name)
	one.Description = strings.TrimSpace(one.Description)
	one.Body = strings.TrimSpace(one.Body)

	if err := RequireName("snippet", one.Name); err != nil {
		return nil, err
	}
	if one.Body == "" {
		return nil, ErrEmptySnippet
	}
	if one.Engine != "" && !one.Engine.Valid() {
		return nil, ValidationError{Field: "engine", Message: "is not one this app knows"}
	}

	kept := make([]Snippet, 0, len(library)+1)
	kept = append(kept, one)
	for _, held := range library {
		if isSnippet(held.Name, one.Name) || isSnippet(held.Name, previous) {
			continue
		}
		kept = append(kept, held)
	}

	return SortSnippets(kept), nil
}

// DeleteSnippet takes one out by name and answers with what is left. A name the library has not got
// is not a failure: what the caller asked for — that it not be there — is already true.
func DeleteSnippet(library []Snippet, name string) []Snippet {
	kept := make([]Snippet, 0, len(library))
	for _, held := range library {
		if isSnippet(held.Name, name) {
			continue
		}
		kept = append(kept, held)
	}
	return kept
}

// SortSnippets is the order a library is read in: by name, folded. A library kept in the order
// things were added to it is a pile, and it is sorted on the way *out* as well as on the way in so
// that a file somebody has edited by hand still reads as a library.
func SortSnippets(library []Snippet) []Snippet {
	slices.SortFunc(library, func(a, b Snippet) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return library
}

// isSnippet is when a held name is the one being named. It folds, because completion does: a reader
// who typed `Recent Orders` and later `recent orders` meant the one snippet both times. A blank name
// matches nothing, which is what lets an empty `previous` mean "this one is new".
func isSnippet(held, name string) bool {
	return name != "" && strings.EqualFold(held, name)
}
