// Package snippet keeps the statements a reader has named. It is history's opposite number and is
// shaped like it on purpose — one JSON document, read whole and written whole — but it holds a
// different kind of thing: history is a record of what ran and this is a library somebody built.
//
// Nothing here decides what a library holds after one more entry. That is domain.SaveSnippet's, for
// the reason domain.Remember owns the same question for history: a name replacing an entry, a rename
// letting go of the old one, and the order the list comes back in are one rule in one place.
package snippet

import "github.com/khanhsnd/vibe-db/internal/domain"

// Repository is the library. Save and Delete each answer with the library they leave behind, so a
// caller that has just written never has to read a second time to know what it now holds.
type Repository interface {
	List() ([]domain.Snippet, error)
	// Save puts one in, replacing whatever stood under its name — and under `previous`, which is the
	// name it was known by before a rename.
	Save(previous string, one domain.Snippet) ([]domain.Snippet, error)
	Delete(name string) ([]domain.Snippet, error)
}
