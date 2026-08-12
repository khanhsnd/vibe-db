// Package sqlserver is the adapter for Microsoft SQL Server. It is the one SQL engine here whose
// names carry *both* halves of a namespace — `[shop].[dbo].[users]` — which is why a single pool
// reads every database on the server where PostgreSQL, whose names stop at the schema, needs one
// pool per database.
package sqlserver

import (
	"context"
	"database/sql"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
	mssql "github.com/microsoft/go-mssqldb"
)

const (
	maxOpenConns    = 4
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute
)

var dialect = sqlkit.SQLServer

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineSQLServer
}

// Text is the lexical half of the engine, which needs no connection: SQL Server writes a name
// inside brackets and reads no backslash as an escape, and `sqlkit.SQLServer` is where that lives.
func (*Driver) Text() driver.Text {
	return dialect
}

// Open dials the server through the driver's own connector rather than through a connection
// string, so the dialler and the TLS configuration are values on it — see options.go, which is
// where everything about the connection is decided.
func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	config, err := buildConfig(profile, creds)
	if err != nil {
		return nil, err
	}

	connector := mssql.NewConnectorConfig(config)
	if tunnel != nil {
		connector.Dialer = tunnelDialer{dial: tunnel, host: profile.Host}
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, translateError(err)
	}

	s := &session{db: db, home: profile.Database}
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
	// Account statements go through `runIn` rather than `run`: they are the only ones this adapter
	// writes that do not name their own database — see `account.go`.
	s.Auth = sqlkit.Auth{
		Dialect:   dialect,
		Run:       s.runIn,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	// A kill names no database and belongs to none — a session is the server's — so this one goes
	// through `run` like every other statement.
	s.Watch = sqlkit.Watch{
		Dialect:   dialect,
		Run:       s.run,
		Translate: translateError,
		Tx:        &s.Txn,
	}
	return s, nil
}

type session struct {
	// The schema changes every SQL engine spells alike come from here; `ddl.go` overrides the few
	// that differ, and `Data` is the reads and writes of rows.
	sqlkit.DDL
	sqlkit.Data
	// Auth spells and runs every change to a login or a user; what `account.go` adds beside it is
	// the catalogue reads that say who exists and what they hold.
	sqlkit.Auth
	// Watch runs the one kill this engine has; what `monitor.go` adds beside it is the reads that say
	// what the server is doing and what it was started with.
	sqlkit.Watch
	// Txn is the manual transaction, and while one is open it is what `DDL`, `Data`, `Auth` and the
	// query path all run on — see `sqlkit.Txn`.
	sqlkit.Txn

	db *sql.DB
	// home is the database the connection was opened with, and is where a catalogue read goes when
	// the caller named none — a server-wide read like the database list has to go *somewhere*.
	home string
}

func (s *session) Ping(ctx context.Context) error {
	return translateError(s.db.PingContext(ctx))
}

// ServerInfo reads the product version rather than @@VERSION, which answers with the whole banner —
// the edition, the build date and the operating system — where the status bar has room for a
// version.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	const query = "SELECT CAST(SERVERPROPERTY('ProductVersion') AS NVARCHAR(128))"

	var version string
	if err := s.db.QueryRowContext(ctx, query).Scan(&version); err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

// Databases lists what this login may actually open. A database that is offline or restoring is
// left out, since connecting to one is not something the reader can do, and HAS_DBACCESS answers
// the rest.
func (s *session) Databases(ctx context.Context) ([]string, error) {
	const query = `
		SELECT name
		FROM sys.databases
		WHERE state = 0 AND HAS_DBACCESS(name) = 1
		ORDER BY name`

	return s.names(ctx, query)
}

// Schemas lists the schemas of one database. Unlike PostgreSQL this needs no connection to that
// database — a three-part name reaches its catalogue from here — which is the whole reason this
// adapter has one pool.
//
// The built-in schemas that exist only to own a fixed database role are left out: `db_owner` and
// its eight relatives hold nothing and never will.
func (s *session) Schemas(ctx context.Context, database string) ([]string, error) {
	query := `
		SELECT s.name
		FROM ` + dialect.QuoteIdent(database) + `.sys.schemas s
		JOIN ` + dialect.QuoteIdent(database) + `.sys.database_principals p
		  ON p.principal_id = s.principal_id
		WHERE p.type NOT IN ('R')
		  AND s.name NOT IN ('sys', 'INFORMATION_SCHEMA')
		ORDER BY s.name`

	return s.names(ctx, query)
}

// Objects reads the three folders of a namespace in one trip, since the tree needs every count the
// moment any one of them is opened.
//
// A routine here needs no argument list to be dropped by: a name is unique within its schema, which
// is MySQL's arrangement rather than PostgreSQL's.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	tables := `
		SELECT t.name
		FROM ` + catalogue + `.sys.tables t
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		WHERE s.name = @p1
		ORDER BY t.name`

	views := `
		SELECT v.name
		FROM ` + catalogue + `.sys.views v
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = v.schema_id
		WHERE s.name = @p1
		ORDER BY v.name`

	// `P` is a stored procedure and the three `F` kinds are the flavours of function — scalar,
	// inline table-valued and table-valued. A CLR routine is deliberately not among them: it is
	// dropped by the same statement but its body is an assembly this app cannot show.
	routines := `
		SELECT o.name, o.type = 'P'
		FROM ` + catalogue + `.sys.objects o
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = o.schema_id
		WHERE s.name = @p1 AND o.type IN ('P', 'FN', 'IF', 'TF')
		ORDER BY o.name`

	// `parent_class = 1` is a trigger on a table or a view, which is the only kind that belongs to
	// a schema; a DDL trigger is on the database or the server and is listed under neither. A trigger
	// the *server* shipped is left out for the reason an internal PostgreSQL one is.
	triggers := `
		SELECT t.name, o.name
		FROM ` + catalogue + `.sys.triggers t
		JOIN ` + catalogue + `.sys.objects o ON o.object_id = t.parent_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = o.schema_id
		WHERE s.name = @p1 AND t.parent_class = 1 AND t.is_ms_shipped = 0
		ORDER BY t.name`

	found, err := s.names(ctx, tables, ns.Schema)
	if err != nil {
		return driver.Objects{}, err
	}

	seen, err := s.names(ctx, views, ns.Schema)
	if err != nil {
		return driver.Objects{}, err
	}

	rows, err := s.db.QueryContext(ctx, routines, ns.Schema)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}
	defer rows.Close()

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

	fired, err := s.db.QueryContext(ctx, triggers, ns.Schema)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}

	firing, err := sqlkit.ScanTriggers(fired)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}

	return driver.Objects{Tables: found, Views: seen, Routines: stored, Triggers: firing}, nil
}

// TableList reads what SQL Server keeps about the tables of a schema. The row count and the size
// come from the partition statistics, which is where the server keeps a number it maintains rather
// than one it would have to count — the same bargain MySQL's `table_rows` and PostgreSQL's
// `reltuples` are.
func (s *session) TableList(ctx context.Context, ns driver.Namespace) (driver.TableList, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT t.name AS [Name],
		       p.rows AS [Rows],
		       CAST(p.pages * 8 AS NVARCHAR(32)) + N' KB' AS [Data Length],
		       t.create_date AS [Created Date],
		       t.modify_date AS [Modified Date]
		FROM ` + catalogue + `.sys.tables t
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		OUTER APPLY (
		    SELECT SUM(ps.row_count) AS rows, SUM(ps.used_page_count) AS pages
		    FROM ` + catalogue + `.sys.dm_db_partition_stats ps
		    WHERE ps.object_id = t.object_id AND ps.index_id IN (0, 1)
		) p
		WHERE s.name = @p1
		ORDER BY t.name`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil, ns.Schema)
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
	s.Txn.Close()
	return s.db.Close()
}
