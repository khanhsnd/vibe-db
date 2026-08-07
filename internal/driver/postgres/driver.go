package postgres

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
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

// Text is the lexical half of the engine, which needs no connection: PostgreSQL reads a `$tag$`
// body as a string and a backslash as an ordinary character, and `sqlkit.Postgres` is where both
// of those live.
func (*Driver) Text() driver.Text {
	return dialect
}

func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	s := &session{
		profile: profile,
		creds:   creds,
		tunnel:  tunnel,
		pools:   map[string]*sql.DB{},
	}
	s.DDL = sqlkit.DDL{Dialect: dialect, Run: s.run}
	s.Data = sqlkit.Data{
		Dialect: dialect,
		Columns: s.columns,
		Open:    func(database string) (sqlkit.Queryer, error) { return s.pool(database) },
		Fail:    translateError,
	}

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
	// The schema changes the engines spell alike come from here; `ddl.go` overrides the few that
	// do not, and `Data` is the reads and writes of rows, which none of them spells differently
	// once the quoting and the paging are the dialect's.
	sqlkit.DDL
	sqlkit.Data

	profile domain.ConnectionProfile
	creds   driver.Credentials
	// tunnel is what every pool of this session dials through, or nil for a direct connection.
	// It is held rather than applied once because a pool is opened per database, lazily.
	tunnel driver.Dialer

	mu    sync.Mutex
	pools map[string]*sql.DB
	// registered names the connection configurations handed to `stdlib`, which keeps them in a
	// package-level map until they are given back. One per pool, forgotten with the session.
	registered []string
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

	// The DSN carries everything libpq has a keyword for. The two things it has none for — the
	// dialler under the connection, and a certificate belonging to a name other than the host
	// dialled — are set on the parsed configuration, which `stdlib` then takes by name.
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, translateError(err)
	}
	if s.tunnel != nil {
		config.DialFunc = pgconn.DialFunc(s.tunnel)
	}
	if name := s.profile.TLS.ServerName; name != "" {
		verifyAs(config, name)
	}

	registered := stdlib.RegisterConnConfig(config)

	db, err := sql.Open("pgx", registered)
	if err != nil {
		stdlib.UnregisterConnConfig(registered)
		return nil, translateError(err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	s.pools[database] = db
	s.registered = append(s.registered, registered)
	return db, nil
}

// verifyAs holds the server's certificate to a name other than the one dialled, which is what a
// tunnel makes ordinary: the connection goes to a local port and the certificate is still the
// database server's. Every fallback is changed too — `prefer` builds one per attempt, and a
// name applied to the first alone would be dropped the moment the first attempt failed.
func verifyAs(config *pgx.ConnConfig, name string) {
	if config.TLSConfig != nil {
		config.TLSConfig.ServerName = name
	}
	for _, fallback := range config.Fallbacks {
		if fallback.TLSConfig != nil {
			fallback.TLSConfig.ServerName = name
		}
	}
}

func (s *session) Close() error {
	s.mu.Lock()
	pools := s.pools
	registered := s.registered
	s.pools = map[string]*sql.DB{}
	s.registered = nil
	s.mu.Unlock()

	var first error
	for _, db := range pools {
		if err := db.Close(); err != nil && first == nil {
			first = err
		}
	}
	for _, name := range registered {
		stdlib.UnregisterConnConfig(name)
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

// Objects reads the three folders of a namespace in one trip, since the tree needs every count
// the moment any one of them is opened. Materialised views are left out of the view list: they
// are dropped by a statement of their own, and a list that cannot be acted on is worse than one
// that does not claim them.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	const tables = `
		SELECT c.relname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname`

	const views = `
		SELECT c.relname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'v'
		ORDER BY c.relname`

	// The identity arguments, not the full ones: a drop names the types alone, and defaults
	// would make the statement it is pasted into invalid.
	const routines = `
		SELECT p.proname,
		       p.prokind = 'p',
		       pg_catalog.pg_get_function_identity_arguments(p.oid)
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND p.prokind IN ('f', 'p')
		ORDER BY p.proname`

	found, err := s.names(ctx, ns.Database, tables, ns.Schema)
	if err != nil {
		return driver.Objects{}, err
	}

	seen, err := s.names(ctx, ns.Database, views, ns.Schema)
	if err != nil {
		return driver.Objects{}, err
	}

	db, err := s.pool(ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	rows, err := db.QueryContext(ctx, routines, ns.Schema)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}
	defer rows.Close()

	stored := []driver.Routine{}
	for rows.Next() {
		var routine driver.Routine
		if err := rows.Scan(&routine.Name, &routine.Procedure, &routine.Arguments); err != nil {
			return driver.Objects{}, translateError(err)
		}
		stored = append(stored, routine)
	}
	if err := rows.Err(); err != nil {
		return driver.Objects{}, translateError(err)
	}

	return driver.Objects{Tables: found, Views: seen, Routines: stored}, nil
}

// TableList reads what PostgreSQL keeps about the tables of a schema. There is no modified date
// among them, because the server keeps none, and the row estimate is NULL until something has
// analysed the table — `(Null)` there says the server has no number, which is the truth of it.
func (s *session) TableList(
	ctx context.Context,
	ns driver.Namespace,
) (driver.TableList, error) {
	const query = `
		SELECT c.relname AS "Name",
		       pg_catalog.pg_get_userbyid(c.relowner) AS "Owner",
		       am.amname AS "Access Method",
		       pg_size_pretty(pg_catalog.pg_table_size(c.oid)) AS "Data Length",
		       NULLIF(c.reltuples, -1)::bigint AS "Rows",
		       pg_catalog.obj_description(c.oid, 'pg_class') AS "Comment"
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_am am ON am.oid = c.relam
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname`

	db, err := s.pool(ns.Database)
	if err != nil {
		return driver.TableList{}, err
	}

	result, err := sqlkit.Read(ctx, db, query, 0, nil, ns.Schema)
	if err != nil {
		return driver.TableList{}, translateError(err)
	}
	return driver.TableList{Columns: result.Columns, Rows: result.Rows}, nil
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
