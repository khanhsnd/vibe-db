package jsonfile

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	fileName = "snippets.json"
	version  = 1
)

// record is a snippet as the file spells it. Every field is a string, so the document is one a reader
// can open, read and edit in any text editor — which is the point of keeping a library in a file
// rather than in the browser's storage: it can be backed up, copied to another machine, and diffed.
type record struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Body        string `json:"body"`
	Engine      string `json:"engine,omitempty"`
}

type document struct {
	Version  int      `json:"version"`
	Snippets []record `json:"snippets"`
}

type Repo struct {
	file *jsonstore.File[document]
}

func New(path string) *Repo {
	return &Repo{file: jsonstore.New[document](path)}
}

func DefaultPath() (string, error) {
	return jsonstore.Path(fileName)
}

func (r *Repo) List() ([]domain.Snippet, error) {
	doc, err := r.file.Read()
	if err != nil {
		return nil, err
	}
	return domain.SortSnippets(toDomainAll(doc.Snippets)), nil
}

// Save hands the whole library to domain.SaveSnippet rather than pushing onto it here, so which
// entry a name replaces and what order the list comes back in are decided in one place.
func (r *Repo) Save(previous string, one domain.Snippet) ([]domain.Snippet, error) {
	return r.write(func(library []domain.Snippet) ([]domain.Snippet, error) {
		return domain.SaveSnippet(library, previous, one)
	})
}

func (r *Repo) Delete(name string) ([]domain.Snippet, error) {
	return r.write(func(library []domain.Snippet) ([]domain.Snippet, error) {
		return domain.DeleteSnippet(library, name), nil
	})
}

// write reads the document, changes the library and writes it back under one lock, answering with
// what the file now holds. A change that fails writes nothing, so a refused save leaves the library
// exactly as the reader last left it.
func (r *Repo) write(change func([]domain.Snippet) ([]domain.Snippet, error)) ([]domain.Snippet, error) {
	var saved []domain.Snippet

	err := r.file.Update(func(doc *document) error {
		library, err := change(toDomainAll(doc.Snippets))
		if err != nil {
			return err
		}

		doc.Version = version
		doc.Snippets = fromDomainAll(library)
		saved = library
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func toDomainAll(records []record) []domain.Snippet {
	library := make([]domain.Snippet, 0, len(records))
	for _, rec := range records {
		library = append(library, domain.Snippet{
			Name:        rec.Name,
			Description: rec.Description,
			Body:        rec.Body,
			Engine:      domain.Engine(rec.Engine),
		})
	}
	return library
}

func fromDomainAll(library []domain.Snippet) []record {
	records := make([]record, 0, len(library))
	for _, one := range library {
		records = append(records, record{
			Name:        one.Name,
			Description: one.Description,
			Body:        one.Body,
			Engine:      string(one.Engine),
		})
	}
	return records
}
