package postgres

import (
	"context"
	"database/sql"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	maxOpenConns    = 4
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute
)

var dialect = sqlkit.Postgres

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EnginePostgres
}

func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
) (driver.Session, error) {
	s := &session{profile: profile, creds: creds, pools: map[string]*sql.DB{}}

	// The profile's own database is what the session is judged on: reaching it proves the
	// host, the credentials and the TLS mode, and it is where the database list is read.
	home, err := s.pool(profile.Database)
	if err != nil {
		return nil, err
	}
	if err := home.PingContext(ctx); err != nil {
		s.Close()
		return nil, translateError(err)
	}

	return s, nil
}

// session keeps one pool per database it has been asked about. A PostgreSQL connection is
// bound to the database it was opened with and cannot cross to another, so browsing a second
// database means dialling it — lazily, on first use, and closed with the session.
type session struct {
	profile domain.ConnectionProfile
	creds   driver.Credentials

	mu    sync.Mutex
	pools map[string]*sql.DB
}

func (s *session) pool(database string) (*sql.DB, error) {
	if database == "" {
		database = s.profile.Database
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if db, ok := s.pools[database]; ok {
		return db, nil
	}

	profile := s.profile
	profile.Database = database

	dsn, err := BuildDSN(profile, s.creds)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, translateError(err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	s.pools[database] = db
	return db, nil
}

func (s *session) Close() error {
	s.mu.Lock()
	pools := s.pools
	s.pools = map[string]*sql.DB{}
	s.mu.Unlock()

	var first error
	for _, db := range pools {
		if err := db.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *session) Ping(ctx context.Context) error {
	db, err := s.pool("")
	if err != nil {
		return err
	}
	return translateError(db.PingContext(ctx))
}

// ServerInfo reads the setting rather than version(), which answers with the whole build
// banner — compiler and platform included — where the status bar has room for a version.
// A packaged build still adds its own suffix, as MySQL's VERSION() does.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	db, err := s.pool("")
	if err != nil {
		return driver.ServerInfo{}, err
	}

	var version string
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&version); err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

// Databases lists what the server will let this user connect to. Templates are left out,
// since connecting to one is not what a template is for.
func (s *session) Databases(ctx context.Context) ([]string, error) {
	const query = `
		SELECT datname
		FROM pg_catalog.pg_database
		WHERE NOT datistemplate
		  AND datallowconn
		  AND has_database_privilege(oid, 'CONNECT')
		ORDER BY datname`

	return s.names(ctx, "", query)
}

// Schemas lists the schemas of one database, and needs a connection to that database to do
// it: pg_namespace only ever describes the database it is read from.
func (s *session) Schemas(ctx context.Context, database string) ([]string, error) {
	const query = `
		SELECT nspname
		FROM pg_catalog.pg_namespace
		WHERE nspname <> 'information_schema'
		  AND left(nspname, 3) <> 'pg_'
		  AND has_schema_privilege(oid, 'USAGE')
		ORDER BY nspname`

	return s.names(ctx, database, query)
}

func (s *session) Tables(ctx context.Context, ns driver.Namespace) ([]string, error) {
	const query = `
		SELECT c.relname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname`

	return s.names(ctx, ns.Database, query, ns.Schema)
}

func (s *session) names(
	ctx context.Context,
	database, query string,
	args ...any,
) ([]string, error) {
	db, err := s.pool(database)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, args...)
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
