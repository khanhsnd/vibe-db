package app

// The snippet library, as three bound methods. Nothing here knows what a snippet *means*: a body is
// the reader's own text and is never read, split or laid out on its way in or out — it becomes a
// statement only when the editor it is put into runs it, on the path every statement takes.
//
// None of the three takes a session. A snippet is kept to be run in more than one place, so it names
// no connection and no namespace, and the library is there to read before anything has been dialled.
//
// **Each write answers with the library it leaves behind**, which is why there is no method that only
// saves. The frontend holds the list in a store shared by every query tab and the editor's own
// completion; a save that answered with nothing would leave that store to read again, and a reader
// whose completion offered yesterday's names until it did.
//
// See docs/features/snippets.md.

// ListSnippets is every statement the reader has named, by name. It is not narrowed by engine here:
// the whole library is what the editor's completion draws from, and which of them belong in front of
// which engine is a fact each snippet carries.
func (a *QueryAPI) ListSnippets() ([]Snippet, error) {
	library, err := a.snippets.List()
	if err != nil {
		return nil, wrapError(err)
	}
	return toSnippetDTOs(library), nil
}

// SaveSnippet keeps one under its name, replacing whatever stood there — and whatever stood under the
// name it was known by, so a rename does not leave the old entry behind.
func (a *QueryAPI) SaveSnippet(input SnippetInput) ([]Snippet, error) {
	library, err := a.snippets.Save(input.Previous, input.toDomain())
	if err != nil {
		return nil, wrapError(err)
	}
	return toSnippetDTOs(library), nil
}

// DeleteSnippet forgets one. A name the library has not got is not a failure — what was asked for is
// already true — so two windows deleting the same snippet do not race into an error.
func (a *QueryAPI) DeleteSnippet(name string) ([]Snippet, error) {
	library, err := a.snippets.Delete(name)
	if err != nil {
		return nil, wrapError(err)
	}
	return toSnippetDTOs(library), nil
}
