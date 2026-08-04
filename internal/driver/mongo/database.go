package mongo

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// firstCollection is the collection a new database is made with when the dialog named none.
//
// **MongoDB has no CREATE DATABASE.** A database exists exactly as long as it holds a collection, and
// there is no command that makes an empty one — so creating a database means creating its first
// collection, which is why the dialog asks for one and why the preview says which. A blank answer
// takes this rather than failing: a reader who only wanted the database is better served by a
// collection they can rename than by an error about a field they did not know was required.
const firstCollection = "data"

// CreateDatabaseSQL is what CreateDatabase will run, and is also what the dialog previews — one
// method spells it, so what is shown and what runs cannot drift apart. It is spelled in the shell
// rather than in SQL for the reason every statement in this adapter is: that is the language the
// reader would type it in.
func (s *session) CreateDatabaseSQL(spec driver.DatabaseSpec) []driver.DatabaseStatement {
	name := spec.Name
	if name == "" {
		return []driver.DatabaseStatement{}
	}

	return []driver.DatabaseStatement{{
		In:  name,
		SQL: "db.getSiblingDB(" + quote(name) + ").createCollection(" + quote(newCollection(spec)) + ")",
	}}
}

func (s *session) CreateDatabase(ctx context.Context, spec driver.DatabaseSpec) error {
	if err := domain.RequireName("database", spec.Name); err != nil {
		return err
	}

	ns := driver.Namespace{Database: spec.Name}
	err := s.database(ns).CreateCollection(ctx, newCollection(spec))
	return translateError(err)
}

func newCollection(spec driver.DatabaseSpec) string {
	if spec.FirstCollection != "" {
		return spec.FirstCollection
	}
	return firstCollection
}

func (s *session) DropDatabase(ctx context.Context, name string) error {
	if err := domain.RequireName("database", name); err != nil {
		return err
	}
	return translateError(s.database(driver.Namespace{Database: name}).Drop(ctx))
}

// DatabaseOptions has nothing to answer with. A MongoDB database is a namespace and nothing more —
// no owner, no template, no encoding, no locale, no tablespace, no extensions — so every list stays
// empty and the dialog then knows not to draw any of them. The one thing it does ask for is the
// name of the first collection, which is a name the reader types rather than one the server offers.
func (*session) DatabaseOptions(context.Context) (driver.DatabaseOptions, error) {
	return driver.DatabaseOptions{}, nil
}
