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

	s := &session{db: db}
	s.DDL = sqlkit.DDL{Dialect: dialect, Run: s.run}
	return s, nil
}

type session struct {
	// The schema changes the two engines spell alike come from here; `ddl.go` overrides the
	// four that do not.
	sqlkit.DDL

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

	return s.names(ctx, query)
}

// Schemas has nothing to answer with: MySQL's schema *is* its database, so the tree stops
// at the level Databases already returned.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

// Objects reads the three folders of a namespace in one trip, since the tree needs every count
// the moment any one of them is opened.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	const tables = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	const views = `
		SELECT table_name
		FROM information_schema.views
		WHERE table_schema = ?
		ORDER BY table_name`

	const routines = `
		SELECT routine_name, routine_type = 'PROCEDURE'
		FROM information_schema.routines
		WHERE routine_schema = ?
		ORDER BY routine_name`

	found, err := s.names(ctx, tables, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	seen, err := s.names(ctx, views, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	rows, err := s.db.QueryContext(ctx, routines, ns.Database)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}
	defer rows.Close()

	// A MySQL routine name is unique within its database, so nothing has to carry an argument
	// list for the drop to find it again.
	stored := []driver.Routine{}
	for rows.Next() {
		var routine driver.Routine
		if err := rows.Scan(&routine.Name, &routine.Procedure); err != nil {
			return driver.Objects{}, translateError(err)
		}
		stored = append(stored, routine)
	}
	if err := rows.Err(); err != nil {
		return driver.Objects{}, translateError(err)
	}

	return driver.Objects{Tables: found, Views: seen, Routines: stored}, nil
}

// TableList reads what MySQL keeps about the tables of a database. The aliases are what the
// grid draws as its headers, and the size is spelled in KB here rather than in the reader:
// MySQL has no pg_size_pretty, so the arithmetic goes where the number already is.
func (s *session) TableList(
	ctx context.Context,
	ns driver.Namespace,
) (driver.TableList, error) {
	const query = "SELECT table_name AS `Name`," +
		" auto_increment AS `Auto Increment`," +
		" update_time AS `Modified Date`," +
		" CONCAT(FORMAT(data_length / 1024, 0), ' KB') AS `Data Length`," +
		" engine AS `Engine`," +
		" table_rows AS `Rows`," +
		" table_comment AS `Comment`" +
		" FROM information_schema.tables" +
		" WHERE table_schema = ? AND table_type = 'BASE TABLE'" +
		" ORDER BY table_name"

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil, ns.Database)
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

func (*session) Split(script string) []string {
	return sqlkit.Split(script, dialect)
}

func (s *session) Close() error {
	return s.db.Close()
}
