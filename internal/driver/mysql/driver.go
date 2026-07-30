package mysql

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	maxOpenConns    = 4
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute
)

var dialect = sqlkit.MySQL

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineMySQL
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

	db, err := sql.Open("mysql", dsn)
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

func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	var version string
	err := s.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

func (s *session) Databases(ctx context.Context) ([]string, error) {
	const query = `
		SELECT schema_name
		FROM information_schema.schemata
		ORDER BY schema_name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var names []string
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

// Schemas has nothing to answer with: MySQL's schema *is* its database, so the tree stops
// at the level Databases already returned.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *session) Tables(ctx context.Context, ns driver.Namespace) ([]string, error) {
	const query = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := s.db.QueryContext(ctx, query, ns.Database)
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
