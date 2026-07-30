package postgres

import (
	"context"
	"database/sql"
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
	dsn, err := BuildDSN(profile, creds)
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

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, translateError(err)
	}

	return &session{db: db}, nil
}

type session struct {
	db *sql.DB
}

func (s *session) Ping(ctx context.Context) error {
	return translateError(s.db.PingContext(ctx))
}

// ServerInfo reads the setting rather than version(), which answers with the whole build
// banner — compiler and platform included — where the status bar has room for a version.
// A packaged build still adds its own suffix, as MySQL's VERSION() does.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	var version string
	err := s.db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&version)
	if err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

// Databases lists the schemas of the connected database, because a PostgreSQL session
// cannot cross to another database and the schema is what a table is qualified with. The
// catalogue's own schemas are left out, and so is anything the user cannot enter.
func (s *session) Databases(ctx context.Context) ([]string, error) {
	const query = `
		SELECT nspname
		FROM pg_catalog.pg_namespace
		WHERE nspname <> 'information_schema'
		  AND left(nspname, 3) <> 'pg_'
		  AND has_schema_privilege(oid, 'USAGE')
		ORDER BY nspname`

	return s.names(ctx, query)
}

func (s *session) Tables(ctx context.Context, schema string) ([]string, error) {
	const query = `
		SELECT c.relname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname`

	return s.names(ctx, query, schema)
}

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
