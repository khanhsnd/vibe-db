package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Everything about *who* a PostgreSQL cluster knows. The statements that change one are
// `sqlkit.Auth`'s, which this session embeds; what is here is catalogue reads.
//
// **A role is the cluster's and its grants are a database's**, and that split is the one fact that
// shapes this whole file. `pg_roles` is the same list whichever database you ask from, but a table
// grant lives in `pg_class.relacl` inside the database that holds the table — so a session bound to
// `shop` cannot see what the same role holds in `warehouse` without dialling it. That is the same
// fact that already gives this adapter a pool per database, and it is why `AccountRef.Database` is
// filled here: it says *which database's grants these are*, and the editor's database picker is
// therefore reading something real rather than decorating a list.
//
// **A role and a user are one object.** `CREATE USER` is `CREATE ROLE … LOGIN` and nothing else, so
// there is no second kind of thing to list — `LOGIN` is an attribute, and it is the one the list
// draws its `Kind` column from.

// The privileges PostgreSQL grants at each level. They are a fixed list rather than a read because
// the server does not enumerate them anywhere a query can reach: unlike MySQL's `SHOW PRIVILEGES`,
// there is no catalogue of what the grammar accepts.
//
// **There is no server level.** What another engine keeps as a privilege over the whole server is a
// role *attribute* here — `SUPERUSER`, `CREATEDB` — and what is left is granted as a **predefined
// role**, `pg_read_all_data` and its kin, which the Roles tab already offers. A server tab drawn
// here would be a tab with nothing in it.
var privilegeSets = []driver.PrivilegeSet{
	{Level: driver.GrantDatabase, Names: []string{"CONNECT", "CREATE", "TEMPORARY"}},
	{Level: driver.GrantSchema, Names: []string{"USAGE", "CREATE"}},
	{Level: driver.GrantTable, Names: []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER",
	}},
	{Level: driver.GrantColumn, Names: []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}},
}

func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	const query = `
		SELECT rolname AS "Name",
		       CASE WHEN rolcanlogin THEN 'user' ELSE 'role' END AS "Kind",
		       rolsuper AS "Superuser",
		       rolcreatedb AS "Create DB",
		       rolcreaterole AS "Create Role",
		       rolreplication AS "Replication",
		       rolconnlimit AS "Connection Limit",
		       COALESCE(rolvaliduntil::text, '') AS "Valid Until"
		FROM pg_roles
		ORDER BY rolname`

	db, err := s.pool("")
	if err != nil {
		return driver.AccountList{}, err
	}

	result, err := sqlkit.Read(ctx, db, query, 0, nil)
	if err != nil {
		return driver.AccountList{}, translateError(err)
	}
	return driver.AccountList{
		Columns: result.Columns,
		Rows:    result.Rows,
		// No host column: an account here is named by one word, and asking for a column that does
		// not exist answers with an empty string rather than shifting what the next one means.
		Refs: sqlkit.AccountRefs(result.Columns, result.Rows, "Name", ""),
	}, nil
}

// Account reads one role whole. The attributes and the database grants come from the cluster-wide
// catalogues, which any pool can answer; everything below a database is read **inside** the database
// the reference names, that being the only place those grants exist.
func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	detail := driver.AccountDetail{AccountRef: ref, Attributes: map[string]string{}}
	if detail.Database == "" {
		detail.Database = s.profile.Database
	}

	home, err := s.pool("")
	if err != nil {
		return driver.AccountDetail{}, err
	}

	const attributes = `
		SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolinherit,
		       rolreplication, rolbypassrls, rolconnlimit,
		       COALESCE(rolvaliduntil::text, '')
		FROM pg_roles WHERE rolname = $1`

	var login, super, createdb, createrole, inherit, replication, bypassrls bool
	var limit int
	var validUntil string
	err = home.QueryRowContext(ctx, attributes, ref.Name).Scan(
		&login, &super, &createdb, &createrole, &inherit,
		&replication, &bypassrls, &limit, &validUntil,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return driver.AccountDetail{}, domain.ErrUnknownAccount
	}
	if err != nil {
		return driver.AccountDetail{}, translateError(err)
	}

	detail.Role = !login
	detail.Attributes = map[string]string{
		"login":           yesNo(login),
		"superuser":       yesNo(super),
		"createdb":        yesNo(createdb),
		"createrole":      yesNo(createrole),
		"inherit":         yesNo(inherit),
		"replication":     yesNo(replication),
		"bypassrls":       yesNo(bypassrls),
		"connectionLimit": strconv.Itoa(limit),
		"validUntil":      validUntil,
	}

	if detail.Grants, err = s.grantsOf(ctx, ref.Name, detail.Database); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Roles, err = s.rolesOf(ctx, ref.Name); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Source, err = s.aclOf(ctx, ref.Name, detail.Database); err != nil {
		return driver.AccountDetail{}, err
	}
	return detail, nil
}

// grantsOf reads the four levels this role holds anything at. The database level is read from the
// cluster-wide `pg_database`; the three below it are read **inside** `database`, which is where
// those catalogues live.
func (s *session) grantsOf(ctx context.Context, role, database string) ([]driver.Grant, error) {
	const databases = `
		SELECT d.datname, a.privilege_type, a.is_grantable
		FROM pg_database d
		CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) a
		JOIN pg_roles g ON g.oid = a.grantee
		WHERE g.rolname = $1
		ORDER BY d.datname, a.privilege_type`

	grants, err := s.scanGrants(ctx, "", databases, driver.GrantDatabase, role)
	if err != nil {
		return nil, err
	}

	// A schema's ACL is not in information_schema at all, so it is read where PostgreSQL keeps it.
	// The catalogue's own schemas are left out: nobody manages a grant on `pg_catalog` from here.
	const schemas = `
		SELECT n.nspname, a.privilege_type, a.is_grantable
		FROM pg_namespace n
		CROSS JOIN LATERAL aclexplode(COALESCE(n.nspacl, acldefault('n', n.nspowner))) a
		JOIN pg_roles g ON g.oid = a.grantee
		WHERE g.rolname = $1
		  AND n.nspname NOT LIKE 'pg\_%' AND n.nspname <> 'information_schema'
		ORDER BY n.nspname, a.privilege_type`

	found, err := s.scanGrants(ctx, database, schemas, driver.GrantSchema, role)
	if err != nil {
		return nil, err
	}
	grants = append(grants, found...)

	const tables = `
		SELECT table_schema, table_name, privilege_type, is_grantable
		FROM information_schema.role_table_grants
		WHERE grantee = $1
		ORDER BY table_schema, table_name, privilege_type`

	if found, err = s.scanObjectGrants(ctx, database, tables, role); err != nil {
		return nil, err
	}
	grants = append(grants, found...)

	if found, err = s.columnGrants(ctx, database, role); err != nil {
		return nil, err
	}
	return append(grants, found...), nil
}

// scanGrants reads a level whose object is the namespace itself — a database or a schema.
func (s *session) scanGrants(
	ctx context.Context,
	in, query string,
	level driver.GrantLevel,
	role string,
) ([]driver.Grant, error) {
	db, err := s.pool(in)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var name, privilege string
		var grantable bool
		if err := rows.Scan(&name, &privilege, &grantable); err != nil {
			return nil, translateError(err)
		}

		grant := driver.Grant{Level: level, Privilege: privilege, Grantable: grantable}
		if level == driver.GrantDatabase {
			grant.Database = name
		} else {
			grant.Database, grant.Schema = in, name
		}
		grants = append(grants, grant)
	}
	return grants, translateError(rows.Err())
}

func (s *session) scanObjectGrants(
	ctx context.Context,
	in, query, role string,
) ([]driver.Grant, error) {
	db, err := s.pool(in)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var schema, table, privilege string
		var grantable bool
		if err := rows.Scan(&schema, &table, &privilege, &grantable); err != nil {
			return nil, translateError(err)
		}
		grants = append(grants, driver.Grant{
			Level:     driver.GrantTable,
			Namespace: driver.Namespace{Database: in, Schema: schema},
			Object:    table,
			Privilege: privilege,
			Grantable: grantable,
		})
	}
	return grants, translateError(rows.Err())
}

// columnGrants reads `pg_attribute.attacl` rather than `information_schema.column_privileges`,
// and the difference matters: that view reports a table-level grant once per column of the table,
// so a role with plain `SELECT` on a forty-column table would come back as forty column grants the
// reader never made. `attacl` is null unless somebody granted on the column itself.
func (s *session) columnGrants(ctx context.Context, in, role string) ([]driver.Grant, error) {
	const query = `
		SELECT n.nspname, c.relname, a.privilege_type, a.is_grantable, at.attname
		FROM pg_attribute at
		JOIN pg_class c ON c.oid = at.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		CROSS JOIN LATERAL aclexplode(at.attacl) a
		JOIN pg_roles g ON g.oid = a.grantee
		WHERE at.attacl IS NOT NULL AND g.rolname = $1
		ORDER BY n.nspname, c.relname, a.privilege_type, at.attname`

	db, err := s.pool(in)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	// One row per column, folded into one Grant per privilege: two columns of one privilege are one
	// thing the reader granted and one row of the editor's grid.
	grants := []driver.Grant{}
	at := map[string]int{}
	for rows.Next() {
		var schema, table, privilege, column string
		var grantable bool
		if err := rows.Scan(&schema, &table, &privilege, &grantable, &column); err != nil {
			return nil, translateError(err)
		}

		key := schema + "\x00" + table + "\x00" + privilege
		if seen, ok := at[key]; ok {
			grants[seen].Columns = append(grants[seen].Columns, column)
			continue
		}
		at[key] = len(grants)
		grants = append(grants, driver.Grant{
			Level:     driver.GrantColumn,
			Namespace: driver.Namespace{Database: in, Schema: schema},
			Object:    table,
			Columns:   []string{column},
			Privilege: privilege,
			Grantable: grantable,
		})
	}
	return grants, translateError(rows.Err())
}

func (s *session) rolesOf(ctx context.Context, role string) ([]string, error) {
	const query = `
		SELECT granted.rolname
		FROM pg_auth_members m
		JOIN pg_roles granted ON granted.oid = m.roleid
		JOIN pg_roles member ON member.oid = m.member
		WHERE member.rolname = $1
		ORDER BY granted.rolname`

	return s.names(ctx, "", query, role)
}

// aclOf is the server's own words. PostgreSQL has no `SHOW GRANTS`, so what stands in is the thing
// it actually keeps: an `aclitem` per object, whose text — `alice=arwdDxt/postgres` — is the
// server's own notation for exactly these privileges. One line per object that mentions the role,
// labelled with what it is on, and nothing about it re-spelled.
func (s *session) aclOf(ctx context.Context, role, database string) ([]string, error) {
	const query = `
		SELECT 'DATABASE ' || d.datname || ': ' || item::text
		FROM pg_database d, unnest(d.datacl) item
		WHERE item::text LIKE $1
		UNION ALL
		SELECT 'SCHEMA ' || n.nspname || ': ' || item::text
		FROM pg_namespace n, unnest(n.nspacl) item
		WHERE item::text LIKE $1
		UNION ALL
		SELECT 'TABLE ' || n.nspname || '.' || c.relname || ': ' || item::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		CROSS JOIN unnest(c.relacl) item
		WHERE item::text LIKE $1
		ORDER BY 1`

	lines, err := s.names(ctx, database, query, quoteLike(role)+"=%")
	if err != nil {
		return []string{}, nil
	}
	return lines, nil
}

// AccountOptions is what the editor is filled from. The privileges are fixed — see privilegeSets —
// and everything else is a read.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: sqlkit.Attributes(dialect),
		Privileges: privilegeSets,
		Roles:      []string{},
		Databases:  []string{},
		// No Hosts: an account here is one name, and a box for a host would be a box for something
		// this engine has no idea of.
		Hosts: []string{},
	}

	roles, err := s.names(ctx, "", "SELECT rolname FROM pg_roles ORDER BY rolname")
	if err != nil {
		return driver.AccountOptions{}, err
	}
	options.Roles = roles

	if databases, err := s.Databases(ctx); err == nil {
		options.Databases = databases
	}
	return options, nil
}

// quoteLike escapes what a role's name would otherwise mean inside a LIKE. The pattern is built
// here rather than bound whole because the `=` that follows it is what anchors the match to the
// grantee rather than to the grantor after the slash.
func quoteLike(value string) string {
	return likeEscape.Replace(value)
}

var likeEscape = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func yesNo(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
