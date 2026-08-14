// Package oracle is the adapter for Oracle Database. The one structural thing to know about it is
// that Oracle has no level between a connection and a schema: a connection asks the listener for a
// *service* and everything inside it is owned by a user. So a `driver.Namespace`'s Database is the
// **schema**, and a table is qualified by one level exactly as a MySQL table is qualified by its
// database — which is why the tree here has the same shape as MySQL's and not PostgreSQL's.
package oracle

import (
	"context"
	"database/sql"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
	go_ora "github.com/sijms/go-ora/v2"
)

const (
	maxOpenConns    = 4
	maxIdleConns    = 2
	connMaxIdleTime = 5 * time.Minute
)

var dialect = sqlkit.Oracle

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineOracle
}

// Text is the lexical half of the engine, which needs no connection: Oracle quotes a name the way
// the standard does and reads no backslash as an escape, and `sqlkit.Oracle` is where that lives.
func (*Driver) Text() driver.Text {
	return dialect
}

// Open dials the listener. The connector is used rather than a bare `sql.Open` so the dialler and
// the TLS configuration can be values on it: neither has a text form, and half a connection in a
// string and half beside it is the arrangement options.go exists to avoid.
func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	url, err := buildURL(profile, creds)
	if err != nil {
		return nil, err
	}

	connector, ok := go_ora.NewConnector(url).(*go_ora.OracleConnector)
	if !ok {
		return nil, domain.Unsupported("this build of the Oracle driver")
	}
	if tunnel != nil {
		connector.Dialer(tunnelDialer{dial: tunnel})
	}

	tlsConfig, err := sqlkit.TLSConfig(profile.TLS, profile.Host)
	if err != nil {
		return nil, domain.FileError{Err: err}
	}
	if tlsConfig != nil {
		connector.WithTLSConfig(tlsConfig)
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, translateError(err)
	}

	s := &session{db: db}
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
	// The schema changes every SQL engine spells alike come from here; `ddl.go` overrides the few
	// that differ, and `Data` is the reads and writes of rows.
	sqlkit.DDL
	sqlkit.Data
	// Auth spells and runs every change to a user or a role; what `account.go` adds beside it is
	// the catalogue reads that say who exists and what they hold.
	sqlkit.Auth
	// Watch runs the one kill this engine has; what `monitor.go` adds beside it is the reads that say
	// what the instance is doing and what it was started with.
	sqlkit.Watch
	// Profile spells and runs every change to the slow query recording, of which this engine has
	// none; what `profile.go` adds beside it is the reads.
	sqlkit.Profile
	// Txn is the manual transaction, and while one is open it is what `DDL`, `Data`, `Auth` and the
	// query path all run on — see `sqlkit.Txn`.
	sqlkit.Txn

	db *sql.DB
}

func (s *session) Ping(ctx context.Context) error {
	return translateError(s.db.PingContext(ctx))
}

// ServerInfo reads the version out of the catalogue rather than out of a banner. `v$instance` would
// answer too and is the usual place to look, but it needs a privilege an ordinary schema owner has
// no reason to hold — and a status bar that says nothing because the reader is not a DBA would be
// worse than one that reads a view everybody can.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	const query = `
		SELECT version
		FROM product_component_version
		WHERE product LIKE 'Oracle%' AND ROWNUM = 1`

	var version string
	if err := s.db.QueryRowContext(ctx, query).Scan(&version); err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: version}, nil
}

// Databases is the schemas this connection can see, which is what stands where a database list
// stands on every other engine — see the note at the top of this file.
//
// It is read as the owners of things rather than as the list of users, which is the same answer
// with the noise taken out: an Oracle instance is installed with dozens of accounts that own
// nothing a reader would ever browse, and a schema with nothing in it is a folder that opens onto
// nothing.
func (s *session) Databases(ctx context.Context) ([]string, error) {
	const query = `
		SELECT DISTINCT owner
		FROM all_objects
		WHERE object_type IN ('TABLE', 'VIEW')
		ORDER BY owner`

	return s.names(ctx, query)
}

// Schemas has nothing to answer with: on Oracle the schema *is* what Databases returned, so the
// tree stops at the level above it.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

// Objects reads the three folders of a namespace in one trip, since the tree needs every count the
// moment any one of them is opened.
//
// A routine here needs no argument list to be dropped by: Oracle will not hold two standalone
// routines of one name in one schema, an overload living inside a package instead. A package is
// deliberately not listed — dropping one drops every routine in it, and a folder whose delete
// removes things it never showed is worse than a folder that does not claim them.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	// The container table a materialized view keeps its rows in is left out: it carries the view's
	// own name and is listed under Materialized Views instead, where dropping it means something.
	const tables = `
		SELECT t.table_name
		FROM all_tables t
		WHERE t.owner = :1
		  AND NOT EXISTS (
		    SELECT 1 FROM all_mviews m
		    WHERE m.owner = t.owner AND m.mview_name = t.table_name
		  )
		ORDER BY t.table_name`
	const views = `SELECT view_name FROM all_views WHERE owner = :1 ORDER BY view_name`

	const materialized = `
		SELECT mview_name FROM all_mviews WHERE owner = :1 ORDER BY mview_name`
	const sequences = `
		SELECT sequence_name
		FROM all_sequences
		WHERE sequence_owner = :1
		ORDER BY sequence_name`

	const routines = `
		SELECT object_name, object_type
		FROM all_objects
		WHERE owner = :1 AND object_type IN ('PROCEDURE', 'FUNCTION')
		ORDER BY object_name`

	// A trigger here may be on the schema or on the database rather than on a table, and that one
	// has no `table_name` — see sqlkit.ScanTriggers, which lists it by the name it has.
	const triggers = `
		SELECT trigger_name, table_name
		FROM all_triggers
		WHERE owner = :1
		ORDER BY trigger_name`

	found, err := s.names(ctx, tables, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	seen, err := s.names(ctx, views, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	kept, err := s.names(ctx, materialized, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	counting, err := s.names(ctx, sequences, ns.Database)
	if err != nil {
		return driver.Objects{}, err
	}

	rows, err := s.db.QueryContext(ctx, routines, ns.Database)
	if err != nil {
		return driver.Objects{}, translateError(err)
	}
	defer rows.Close()

	stored := []driver.Routine{}
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			return driver.Objects{}, translateError(err)
		}
		stored = append(stored, driver.Routine{Name: name, Procedure: kind == "PROCEDURE"})
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
		Tables:            found,
		Views:             seen,
		Routines:          stored,
		Triggers:          firing,
		MaterializedViews: kept,
		Sequences:         counting,
	}, nil
}

// TableList reads what Oracle keeps about the tables of a schema. The row count and the size are
// what the *optimizer* was last told, not what is there now: both are NULL until something has
// gathered statistics, and `(Null)` there says the server has no number — which is the truth of it,
// and the same bargain PostgreSQL's `reltuples` is.
func (s *session) TableList(ctx context.Context, ns driver.Namespace) (driver.TableList, error) {
	const query = `
		SELECT table_name AS "Name",
		       num_rows AS "Rows",
		       TO_CHAR(blocks * 8) || ' KB' AS "Data Length",
		       tablespace_name AS "Tablespace",
		       last_analyzed AS "Analyzed",
		       partitioned AS "Partitioned"
		FROM all_tables t
		WHERE t.owner = :1
		  AND NOT EXISTS (
		    SELECT 1 FROM all_mviews m
		    WHERE m.owner = t.owner AND m.mview_name = t.table_name
		  )
		ORDER BY table_name`

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
	s.Txn.Close()
	return s.db.Close()
}
