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

// Text is the lexical half of the engine, which needs no connection: MySQL reads `#` as a
// comment and a backslash as an escape, and `sqlkit.MySQL` is where both of those live.
func (*Driver) Text() driver.Text {
	return dialect
}

func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	transport, err := dial(profile, tunnel)
	if err != nil {
		return nil, err
	}

	dsn, err := BuildDSN(profile, creds, transport)
	if err != nil {
		transport.release()
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		transport.release()
		return nil, translateError(err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		transport.release()
		return nil, translateError(err)
	}

	s := &session{db: db, release: transport.release}
	s.Txn = sqlkit.Txn{Pin: s.pinned, Translate: translateError}
	s.DDL = sqlkit.DDL{
		Dialect:   dialect,
		Run:       s.run,
		Pin:       s.Pinned,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	s.Data = sqlkit.Data{
		Dialect: dialect,
		Columns: s.columns,
		Open:    func(string) (sqlkit.Queryer, error) { return s.db, nil },
		Fail:    translateError,
		Tx:      &s.Txn,
	}
	s.Auth = sqlkit.Auth{
		Dialect:   dialect,
		Run:       s.run,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	s.Watch = sqlkit.Watch{
		Dialect:   dialect,
		Run:       s.run,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	s.Profile = sqlkit.Profile{
		Dialect:   dialect,
		Run:       s.run,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	return s, nil
}

type session struct {
	// The schema changes the engines spell alike come from here; `ddl.go` overrides the few that
	// do not, and `Data` is the reads and writes of rows, which none of them spells differently
	// once the quoting and the paging are the dialect's.
	sqlkit.DDL
	sqlkit.Data
	// Auth spells and runs every change to an account; what `account.go` adds beside it is the
	// catalogue reads that say who exists and what they hold.
	sqlkit.Auth
	// Watch spells and runs every kill there is; what `monitor.go` adds beside it is the reads that
	// say what the server is doing and what it was started with.
	sqlkit.Watch
	// Profile spells and runs every change to the slow query recording; what `profile.go` adds
	// beside it is the reads that say what ran slowly and how the recording is set.
	sqlkit.Profile
	// Txn is the manual transaction, and while one is open it is what `DDL`, `Data`, `Auth` and the
	// query path above all run on — see `sqlkit.Txn`.
	sqlkit.Txn

	db *sql.DB
	// release forgets the dialler and the TLS configuration this session registered with
	// go-sql-driver, which are named globally and would otherwise outlive it.
	release func()
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

// Objects reads the four folders of a namespace in one trip, since the tree needs every count
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

	const triggers = `
		SELECT trigger_name, event_object_table
		FROM information_schema.triggers
		WHERE trigger_schema = ?
		ORDER BY trigger_name`

	// The one engine here with an event, which is a statement the server runs on a schedule of its
	// own. There is no materialized view and no sequence to read: a number generator on MySQL is a
	// column's `AUTO_INCREMENT` rather than an object of the schema.
	const events = `
		SELECT event_name
		FROM information_schema.events
		WHERE event_schema = ?
		ORDER BY event_name`

	found, err := s.names(ctx, tables, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	seen, err := s.names(ctx, views, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	scheduled, err := s.names(ctx, events, ns.Database)
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

	fired, err := s.db.QueryContext(ctx, triggers, ns.Database)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}

	firing, err := sqlkit.ScanTriggers(fired)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}

	return driver.Objects{
		Tables:   found,
		Views:    seen,
		Routines: stored,
		Triggers: firing,
		Events:   scheduled,
	}, nil
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

// Close rolls back whatever transaction is still open before the pool goes, a transaction nobody
// can reach any more being one the server would hold every lock of until the connection died.
func (s *session) Close() error {
	defer s.release()
	s.Txn.Close()
	return s.db.Close()
}
