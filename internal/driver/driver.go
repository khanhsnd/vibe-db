package driver

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

type Credentials struct {
	Password string
}

type ServerInfo struct {
	Version string
}

type Column struct {
	Name       string
	Type       string
	FullType   string
	Nullable   bool
	PrimaryKey bool
	Default    *string
}

// Namespace is where a table lives. MySQL has one level and fills Database alone;
// PostgreSQL has two and fills both, since a table is addressed by a schema inside the
// database the session is bound to.
type Namespace struct {
	Database string
	Schema   string
}

// TableSort orders a table read by one of its columns. An empty Column leaves the primary
// key in force, which is what a page is cut from when the reader has chosen nothing.
type TableSort struct {
	Column     string
	Descending bool
}

// TablePage is one window onto a table's rows. A Limit of zero or less reads every row and
// makes Offset meaningless, the same convention as Query.MaxRows.
type TablePage struct {
	Namespace
	Table  string
	Limit  int
	Offset int
	Sort   TableSort
}

// TableSchema is the completion-sized view of a table: names only, no types, because
// the editor needs every table of a database at once and nothing more.
type TableSchema struct {
	Name    string
	Columns []string
}

type TableRows struct {
	Columns []Column
	Rows    [][]*string
	Total   int64
}

type Query struct {
	Namespace
	SQL     string
	MaxRows int
	Explain bool
}

// QueryResult carries its Namespace and Table only when its rows map one-to-one onto the
// rows of a single table; both stay empty for a result that cannot be written back.
type QueryResult struct {
	Namespace
	Columns      []Column
	Rows         [][]*string
	RowsAffected int64
	Truncated    bool
	Table        string
}

type CellEdit struct {
	Namespace
	Table  string
	Column string
	Value  *string
	Key    map[string]string
}

// RowInsert carries only the columns a new row actually filled in, so a column left alone
// takes the server's own default instead of an explicit NULL.
type RowInsert struct {
	Namespace
	Table  string
	Values map[string]*string
}

// RowDelete names one stored row the way CellEdit does, by its primary key.
type RowDelete struct {
	Namespace
	Table string
	Key   map[string]string
}

type Driver interface {
	Engine() domain.Engine
	Open(ctx context.Context, profile domain.ConnectionProfile, creds Credentials) (Session, error)
}

type Session interface {
	Ping(ctx context.Context) error
	ServerInfo(ctx context.Context) (ServerInfo, error)
	Databases(ctx context.Context) ([]string, error)
	// Schemas lists the level below a database, and returns nothing for an engine that has
	// none. It is read lazily, one database at a time, because PostgreSQL can only see the
	// schemas of a database something is connected to.
	Schemas(ctx context.Context, database string) ([]string, error)
	Tables(ctx context.Context, ns Namespace) ([]string, error)
	TableSchemas(ctx context.Context, ns Namespace) ([]TableSchema, error)
	TableRows(ctx context.Context, page TablePage) (TableRows, error)
	UpdateCell(ctx context.Context, edit CellEdit) error
	InsertRow(ctx context.Context, insert RowInsert) error
	DeleteRow(ctx context.Context, removal RowDelete) error
	Query(ctx context.Context, query Query) (QueryResult, error)
	Close() error
}

type Registry struct {
	drivers map[domain.Engine]Driver
}

func NewRegistry(drivers ...Driver) *Registry {
	r := &Registry{drivers: make(map[domain.Engine]Driver, len(drivers))}
	for _, d := range drivers {
		r.drivers[d.Engine()] = d
	}
	return r
}

func (r *Registry) For(engine domain.Engine) (Driver, error) {
	d, ok := r.drivers[engine]
	if !ok {
		return nil, domain.ValidationError{Field: "engine", Message: "is not supported"}
	}
	return d, nil
}
