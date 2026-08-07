// Package sqlite is the adapter for a database that is one file on this machine. It is a
// `database/sql` adapter like the other three, and everything that makes it different follows from
// there being no server: nothing is dialled, nobody is authenticated, and the whole of the
// catalogue is a table called `sqlite_master` and a handful of pragmas.
package sqlite

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
	_ "modernc.org/sqlite"
)

const (
	// A file has no round trip to amortise and no server to be polite to, but SQLite writes are
	// serialised anyway, so a small pool is all a pool can be.
	maxOpenConns    = 4
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute

	// mainDatabase is the one namespace a connection has. SQLite has `temp` as well, and whatever
	// has been ATTACHed, but this app attaches nothing and a temporary table dies with the
	// connection that made it — so there is exactly one namespace worth drawing.
	mainDatabase = "main"
)

var dialect = sqlkit.SQLite

type Driver struct{}

// New builds the driver, and gives SQLite the `regexp` its grammar names and its library leaves
// out. It is done here rather than at first use because the function is registered onto the
// *driver*, so every connection opened afterwards has it and one opened before would not — see
// registerRegexp.
func New() *Driver {
	registerRegexp()
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineSQLite
}

// Text is the lexical half of the engine, which needs no connection: SQLite quotes a name the way
// the standard does and reads no backslash as an escape, and `sqlkit.SQLite` is where that lives.
func (*Driver) Text() driver.Text {
	return dialect
}

// Open opens the file the profile names. There is nothing to dial, so a tunnel is refused rather
// than quietly ignored: a reader who configured one is expecting the file to be somewhere else,
// and opening a local file of the same name instead would be the worst possible answer.
//
// The file has to be **there already**. SQLite would otherwise make an empty one at the first
// touch, so a mistyped path would open as a database with no tables in it rather than as the
// mistake it is.
func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	_ driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	if tunnel != nil {
		return nil, domain.Unsupported("reaching a file through an SSH tunnel")
	}
	if _, err := os.Stat(profile.Database); err != nil {
		return nil, domain.FileError{Err: err}
	}

	db, err := sql.Open("sqlite", dsn(profile.Database))
	if err != nil {
		return nil, translateError(err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, translateError(err)
	}

	s := &session{db: db}
	s.DDL = sqlkit.DDL{Dialect: dialect, Run: s.run}
	s.Data = sqlkit.Data{
		Dialect: dialect,
		Columns: s.columns,
		Open:    func(string) (sqlkit.Queryer, error) { return s.db, nil },
		Fail:    translateError,
	}
	return s, nil
}

// dsn spells the file as the URI the driver takes. It is a URI rather than a bare path so that
// `foreign_keys` can be asked for: SQLite enforces a referential constraint only when told to,
// per connection, and a client that showed the constraints and then let a write break them would
// be showing something untrue.
func dsn(path string) string {
	return "file:" + url.PathEscape(path) + "?_pragma=" + url.QueryEscape("foreign_keys(1)") +
		"&_pragma=" + url.QueryEscape("busy_timeout(5000)")
}

type session struct {
	// The schema changes every SQL engine spells alike come from here; `ddl.go` overrides what
	// SQLite has not got, and `Data` is the reads and writes of rows.
	sqlkit.DDL
	sqlkit.Data

	db *sql.DB
}

func (s *session) Ping(ctx context.Context) error {
	return translateError(s.db.PingContext(ctx))
}

func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	var version string
	if err := s.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

// Databases is the one namespace this file has. It is not read from anywhere: `main` is what
// SQLite calls the database a connection was opened with, on every version there has ever been.
func (*session) Databases(context.Context) ([]string, error) {
	return []string{mainDatabase}, nil
}

// Schemas has nothing to answer with: a SQLite database has no level below itself.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

// Objects reads the two folders a SQLite file has. There are no stored routines — a function here
// is registered by the program that opened the file and is gone when it closes — so nothing is
// read for them and the tree draws no folder.
//
// The `sqlite_` names are the engine's own bookkeeping, exactly as MongoDB's `system.` ones are,
// and are not listed.
func (s *session) Objects(ctx context.Context, _ driver.Namespace) (driver.Objects, error) {
	const query = `
		SELECT name
		FROM sqlite_master
		WHERE type = ? AND name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY name`

	tables, err := s.names(ctx, query, "table")
	if err != nil {
		return driver.Objects{}, err
	}

	views, err := s.names(ctx, query, "view")
	if err != nil {
		return driver.Objects{}, err
	}

	return driver.Objects{Tables: tables, Views: views, Routines: []driver.Routine{}}, nil
}

// TableList reads what SQLite keeps about its tables, which is less than any other engine keeps:
// there is no size, no owner and no modified date in the file's own catalogue, and a row count is
// a full scan rather than an estimate. What there is instead is the shape — how many columns and
// how many indexes — read from the pragmas the catalogue exposes as tables.
func (s *session) TableList(ctx context.Context, _ driver.Namespace) (driver.TableList, error) {
	const query = `
		SELECT m.name AS "Name",
		       (SELECT COUNT(*) FROM pragma_table_info(m.name)) AS "Columns",
		       (SELECT COUNT(*) FROM pragma_index_list(m.name)) AS "Indexes",
		       CASE WHEN EXISTS (SELECT 1 FROM pragma_foreign_key_list(m.name))
		            THEN 'yes' ELSE 'no' END AS "Foreign Keys",
		       m.sql AS "Definition"
		FROM sqlite_master m
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY m.name`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.TableList{}, translateError(err)
	}
	return driver.TableList{Columns: result.Columns, Rows: result.Rows}, nil
}

// names runs a one-column catalogue read, which is what every list this adapter answers with
// comes down to.
func (s *session) names(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, translateError(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return names, nil
}

func (s *session) Close() error {
	return s.db.Close()
}
