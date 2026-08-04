// Package mongo is the MongoDB adapter: a `driver.Driver` over the official Go driver.
//
// It is the first adapter that is not a `database/sql` one, and the three places that shows are the
// three places a document store really differs from a table store:
//
//   - **A collection declares no fields.** What a table's catalogue read answers with, `fields`
//     infers from the documents themselves — see documents.go, and docs/grid.md for what that costs.
//   - **A filter is a document, not a clause.** Nothing a reader typed is ever spelled into a
//     query, so there is no quoting in this package at all: `filter.go` builds `bson.D`.
//   - **The verb is a method, not a first word.** `text.go` reads a command whole, because the
//     grammar is small enough to; what the command then *does* is still `domain.MongoHazard`'s rule.
//
// Everything else is the same adapter every other engine is, and everything lexical is still read by
// `sqlkit.Lex` — see the dependency rule in docs/architecture.md.
package mongo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// adminDatabase is where a command that is about the *server* goes, and where a session with no
// database of its own is pointed. Every MongoDB deployment has it.
const adminDatabase = "admin"

// The adapter is held to both halves of the port at compile time, because the two are wide and a
// method missed here would otherwise only be missed where the registry is built.
var (
	_ driver.Driver  = (*Driver)(nil)
	_ driver.Session = (*session)(nil)
)

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineMongo
}

// Text is the lexical half of the engine, which needs no connection: `//` opens a comment, a brace
// is a value and the verb is a method. See text.go.
func (*Driver) Text() driver.Text {
	return text{}
}

func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	opts, err := clientOptions(profile, creds, tunnel)
	if err != nil {
		return nil, err
	}

	client, err := mgo.Connect(opts)
	if err != nil {
		return nil, translateError(err)
	}

	// Connect does not dial: the driver is lazy about topology, so nothing here has been reached
	// until something is asked of it. A profile with the wrong host would otherwise open cleanly
	// and fail on the first read.
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, translateError(err)
	}

	return &session{client: client, home: profile.Database}, nil
}

type session struct {
	client *mgo.Client
	// home is the database the profile named, and is where a command carrying no namespace goes. It
	// is a *starting point* and not a binding, unlike PostgreSQL's: one MongoDB connection reads
	// every database on the server, which is why this adapter needs one pool and not one per
	// database.
	home string
}

func (s *session) Ping(ctx context.Context) error {
	return translateError(s.client.Ping(ctx, nil))
}

// ServerInfo reads the build. It goes to whichever database the session starts in rather than always
// to `admin`, because a reader whose credentials are scoped to one database can read `buildInfo`
// there and cannot read anything in `admin` at all.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	var build struct {
		Version string `bson:"version"`
	}

	err := s.database(driver.Namespace{}).
		RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).
		Decode(&build)
	if err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: build.Version}, nil
}

func (s *session) Databases(ctx context.Context) ([]string, error) {
	names, err := s.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		return nil, translateError(err)
	}
	sort.Strings(names)
	return names, nil
}

// Schemas has nothing to answer with: a MongoDB database holds collections directly, so the tree
// stops at the level Databases already returned — the same shape MySQL has.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

// Objects reads the two folders a MongoDB namespace has. A view is a collection whose contents are
// an aggregation over another, which is why one read answers both — and there is no third folder:
// server-side JavaScript was removed from the server, so there are no stored routines to list.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	found, err := s.collections(ctx, ns)
	if err != nil {
		return driver.Objects{}, err
	}

	collections, views := []string{}, []string{}
	for _, one := range found {
		if one.Type == "view" {
			views = append(views, one.Name)
			continue
		}
		collections = append(collections, one.Name)
	}

	return driver.Objects{Tables: collections, Views: views, Routines: []driver.Routine{}}, nil
}

// systemPrefix marks the collections the server keeps for its own bookkeeping. `system.views` appears
// the moment a view is made and `system.profile` the moment profiling is turned on, and neither is
// something the reader created or can usefully act on — a rename or a drop of one is a corrupted
// database — so they are not listed. It is the prefix the server reserves, so this is the whole rule.
const systemPrefix = "system."

// collections lists what a database holds, in name order, with the kind of each. `listCollections`
// is the one read in this adapter that says whether something is a view, and both TableList and
// Objects need that: `$collStats` refuses a view, having no storage to report.
func (s *session) collections(
	ctx context.Context,
	ns driver.Namespace,
) ([]mgo.CollectionSpecification, error) {
	all, err := s.database(ns).ListCollectionSpecifications(ctx, bson.D{})
	if err != nil {
		return nil, translateError(err)
	}

	found := make([]mgo.CollectionSpecification, 0, len(all))
	for _, one := range all {
		if !strings.HasPrefix(one.Name, systemPrefix) {
			found = append(found, one)
		}
	}

	sort.Slice(found, func(a, b int) bool { return found[a].Name < found[b].Name })
	return found, nil
}

// database is the database a call is addressed to: the namespace's own, the one the profile named,
// or `admin`.
func (s *session) database(ns driver.Namespace) *mgo.Database {
	return s.client.Database(s.databaseName(ns))
}

func (s *session) databaseName(ns driver.Namespace) string {
	switch {
	case ns.Database != "":
		return ns.Database
	case s.home != "":
		return s.home
	default:
		return adminDatabase
	}
}

func (s *session) collection(ns driver.Namespace, name string) *mgo.Collection {
	return s.database(ns).Collection(name)
}

// qualified is how the server names a collection in a command that crosses databases — a rename —
// and is also what an index reports its own namespace as.
func (s *session) qualified(ns driver.Namespace, name string) string {
	return s.databaseName(ns) + "." + name
}

func (s *session) Close() error {
	// Disconnect waits for in-flight work, and the context it is given is the one bound to that
	// wait rather than to any caller: Close takes none, and a pool that will not shut down is not
	// something the reader who pressed Disconnect can do anything about.
	return translateError(s.client.Disconnect(context.Background()))
}

// runCommand runs one command against a database and hands back what the server said, which is what
// every DDL in this adapter that has no helper of its own is built on.
func (s *session) runCommand(ctx context.Context, ns driver.Namespace, command bson.D) (bson.Raw, error) {
	raw, err := s.database(ns).RunCommand(ctx, command).Raw()
	if err != nil {
		return nil, translateError(err)
	}
	return raw, nil
}

// sizeText spells a byte count the way a reader reads one. It is done here rather than in the grid
// for the reason MySQL's KB arithmetic is done in its query: the number belongs beside the number.
func sizeText(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}
