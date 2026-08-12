package sqlserver

import (
	"context"
	"database/sql"
	"errors"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Everything about *who* a SQL Server instance knows. The statements are `sqlkit.Auth`'s, which
// this session embeds; what is here is catalogue reads.
//
// **An account here is two objects, and that is the whole shape of this file.** A **login** belongs
// to the server and is what signs in; a **user** belongs to one database and is what may touch
// anything inside it. A login with no user in a database can connect to the instance and reach
// nothing in that database, which is not a failure the reader would ever guess at — so the list is
// of logins, and everything a database knows about one is read against the database the reference
// names.
//
// It is also the reason this adapter is the only one whose account statements are **pinned**: every
// other statement it writes carries a three-part name that reaches any database from one pool, and
// `CREATE USER` and `GRANT … ON OBJECT::` do not — a grant is about the database it runs in.

func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	const query = `
		SELECT p.name AS [Name],
		       CASE WHEN p.type = 'R' THEN 'role' ELSE 'user' END AS [Kind],
		       p.type_desc AS [Type],
		       p.is_disabled AS [Disabled],
		       ISNULL(p.default_database_name, '') AS [Default Database],
		       p.create_date AS [Created],
		       p.modify_date AS [Modified]
		FROM sys.server_principals p
		WHERE p.name NOT LIKE '##%'
		ORDER BY p.name`

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.AccountList{}, translateError(err)
	}
	return driver.AccountList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.AccountRefs(result.Columns, result.Rows, "Name", ""),
	}, nil
}

// Account reads one login whole, together with what the named database knows about the user of the
// same name. A login with no user there is not a failure: the attributes and the server privileges
// are still true, and the database half is simply empty — which is exactly what the reader needs to
// see to know they have to make one.
func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	database := ref.Database
	if database == "" {
		database = s.home
	}

	detail := driver.AccountDetail{
		AccountRef: driver.AccountRef{Name: ref.Name, Database: database},
		Attributes: map[string]string{},
	}

	const login = `
		SELECT p.type = 'R', p.is_disabled, ISNULL(p.default_database_name, ''),
		       ISNULL(l.is_policy_checked, 0)
		FROM sys.server_principals p
		LEFT JOIN sys.sql_logins l ON l.principal_id = p.principal_id
		WHERE p.name = @p1`

	var isRole, disabled, policy bool
	var defaultDatabase string
	err := s.db.QueryRowContext(ctx, login, ref.Name).
		Scan(&isRole, &disabled, &defaultDatabase, &policy)
	if errors.Is(err, sql.ErrNoRows) {
		return driver.AccountDetail{}, domain.ErrUnknownAccount
	}
	if err != nil {
		return driver.AccountDetail{}, translateError(err)
	}

	detail.Role = isRole
	detail.Attributes["disabled"] = yesNo(disabled)
	detail.Attributes["defaultDatabase"] = defaultDatabase
	detail.Attributes["checkPolicy"] = yesNo(policy)

	if schema, err := s.defaultSchema(ctx, database, ref.Name); err == nil {
		detail.Attributes["defaultSchema"] = schema
	}

	if detail.Grants, err = s.grantsOf(ctx, database, ref.Name); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Roles, err = s.rolesOf(ctx, database, ref.Name); err != nil {
		return driver.AccountDetail{}, err
	}
	// There is no `SHOW GRANTS` here and nothing that stands in for one: what SQL Server keeps is
	// `sys.database_permissions`, which the Privileges tab already draws row for row. An invented
	// rendering put in this pane would be this app's words dressed as the server's.
	detail.Source = []string{}
	return detail, nil
}

func (s *session) defaultSchema(ctx context.Context, database, name string) (string, error) {
	query := "SELECT ISNULL(default_schema_name, '') FROM " +
		dialect.QuoteIdent(database) + ".sys.database_principals WHERE name = @p1"

	var schema string
	err := s.db.QueryRowContext(ctx, query, name).Scan(&schema)
	return schema, translateError(err)
}

// grantsOf reads both halves: what the *server* granted the login, and what the named database
// granted the user of that name.
func (s *session) grantsOf(ctx context.Context, database, name string) ([]driver.Grant, error) {
	const server = `
		SELECT pe.permission_name, pe.state_desc
		FROM sys.server_permissions pe
		JOIN sys.server_principals p ON p.principal_id = pe.grantee_principal_id
		WHERE p.name = @p1 AND pe.state_desc <> 'REVOKE'
		ORDER BY pe.permission_name`

	rows, err := s.db.QueryContext(ctx, server, name)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var privilege, state string
		if err := rows.Scan(&privilege, &state); err != nil {
			return nil, translateError(err)
		}
		grants = append(grants, driver.Grant{
			Level:     driver.GrantServer,
			Privilege: privilege,
			Grantable: state == "GRANT_WITH_GRANT_OPTION",
			Denied:    state == "DENY",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	inside, err := s.databaseGrants(ctx, database, name)
	if err != nil {
		// A login with no user in this database has nothing here to read, which is not a failure —
		// see Account.
		return grants, nil
	}
	return append(grants, inside...), nil
}

// databaseGrants reads what one database granted. The class says which level it is: 0 is the
// database itself, 3 is a schema, and 1 is an object — or a **column** of one, which is what a
// minor id above zero means.
func (s *session) databaseGrants(
	ctx context.Context,
	database, name string,
) ([]driver.Grant, error) {
	query := `
		SELECT pe.class, pe.minor_id, pe.permission_name, pe.state_desc,
		       ISNULL(ISNULL(s.name, sc.name), '') AS [schema],
		       ISNULL(o.name, '') AS [object],
		       ISNULL(c.name, '') AS [column]
		FROM ` + dialect.QuoteIdent(database) + `.sys.database_permissions pe
		JOIN ` + dialect.QuoteIdent(database) + `.sys.database_principals dp
		  ON dp.principal_id = pe.grantee_principal_id
		LEFT JOIN ` + dialect.QuoteIdent(database) + `.sys.objects o
		  ON pe.class = 1 AND o.object_id = pe.major_id
		LEFT JOIN ` + dialect.QuoteIdent(database) + `.sys.schemas s
		  ON s.schema_id = o.schema_id
		LEFT JOIN ` + dialect.QuoteIdent(database) + `.sys.schemas sc
		  ON pe.class = 3 AND sc.schema_id = pe.major_id
		LEFT JOIN ` + dialect.QuoteIdent(database) + `.sys.columns c
		  ON pe.class = 1 AND pe.minor_id > 0
		 AND c.object_id = pe.major_id AND c.column_id = pe.minor_id
		WHERE dp.name = @p1 AND pe.state_desc <> 'REVOKE'
		ORDER BY pe.class, [schema], [object], pe.permission_name`

	rows, err := s.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var class, minor int
		var privilege, state, schema, object, column string
		err := rows.Scan(&class, &minor, &privilege, &state, &schema, &object, &column)
		if err != nil {
			return nil, translateError(err)
		}

		grant := driver.Grant{
			Namespace: driver.Namespace{Database: database, Schema: schema},
			Object:    object,
			Privilege: privilege,
			Grantable: state == "GRANT_WITH_GRANT_OPTION",
			Denied:    state == "DENY",
		}
		switch {
		case class == 0:
			grant.Level, grant.Schema, grant.Object = driver.GrantDatabase, "", ""
		case class == 3:
			grant.Level, grant.Object = driver.GrantSchema, ""
		case minor > 0:
			grant.Level, grant.Columns = driver.GrantColumn, []string{column}
		default:
			grant.Level = driver.GrantTable
		}
		grants = append(grants, grant)
	}
	return grants, translateError(rows.Err())
}

// rolesOf reads both kinds, and they are two different things wearing one word: a **server** role
// is held by the login and a **database** role by the user. Both are listed, because a reader
// managing an account cares which groups it is in and not which catalogue keeps the fact.
func (s *session) rolesOf(ctx context.Context, database, name string) ([]string, error) {
	const server = `
		SELECT r.name
		FROM sys.server_role_members m
		JOIN sys.server_principals r ON r.principal_id = m.role_principal_id
		JOIN sys.server_principals p ON p.principal_id = m.member_principal_id
		WHERE p.name = @p1
		ORDER BY r.name`

	roles, err := s.names(ctx, server, name)
	if err != nil {
		return nil, err
	}

	quoted := dialect.QuoteIdent(database)
	inside := `
		SELECT r.name
		FROM ` + quoted + `.sys.database_role_members m
		JOIN ` + quoted + `.sys.database_principals r ON r.principal_id = m.role_principal_id
		JOIN ` + quoted + `.sys.database_principals p ON p.principal_id = m.member_principal_id
		WHERE p.name = @p1
		ORDER BY r.name`

	found, err := s.names(ctx, inside, name)
	if err != nil {
		return roles, nil
	}
	return append(roles, found...), nil
}

// AccountOptions reads what this instance understands. `sys.fn_builtin_permissions` is the server's
// own enumeration of every permission there is, sorted by the class it applies to — which is the
// same read MySQL's `SHOW PRIVILEGES` is, and the reason neither engine's list is written out here.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: sqlkit.Attributes(dialect),
		Roles:      []string{},
		Hosts:      []string{},
		Databases:  []string{},
	}

	const query = `
		SELECT class_desc, permission_name
		FROM sys.fn_builtin_permissions(DEFAULT)
		WHERE class_desc IN ('SERVER', 'DATABASE', 'SCHEMA', 'OBJECT')
		ORDER BY class_desc, permission_name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return driver.AccountOptions{}, translateError(err)
	}
	defer rows.Close()

	byLevel := map[driver.GrantLevel][]string{}
	for rows.Next() {
		var class, permission string
		if err := rows.Scan(&class, &permission); err != nil {
			return driver.AccountOptions{}, translateError(err)
		}
		byLevel[levelOfClass(class)] = append(byLevel[levelOfClass(class)], permission)
	}
	if err := rows.Err(); err != nil {
		return driver.AccountOptions{}, translateError(err)
	}

	options.Privileges = []driver.PrivilegeSet{
		{Level: driver.GrantServer, Names: byLevel[driver.GrantServer]},
		{Level: driver.GrantDatabase, Names: byLevel[driver.GrantDatabase]},
		{Level: driver.GrantSchema, Names: byLevel[driver.GrantSchema]},
		{Level: driver.GrantTable, Names: byLevel[driver.GrantTable]},
		// A column takes four of the object permissions and no others, which is the one part of
		// this the builtin list does not separate out.
		{Level: driver.GrantColumn, Names: []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}},
	}

	if databases, err := s.Databases(ctx); err == nil {
		options.Databases = databases
	}
	// Both kinds of role are offered, for the reason both kinds are listed on an account.
	roles, err := s.names(ctx, `
		SELECT name FROM sys.server_principals WHERE type = 'R' AND name NOT LIKE '##%'
		UNION SELECT name FROM sys.database_principals WHERE type = 'R'
		ORDER BY name`)
	if err == nil {
		options.Roles = roles
	}
	return options, nil
}

func levelOfClass(class string) driver.GrantLevel {
	switch class {
	case "SERVER":
		return driver.GrantServer
	case "DATABASE":
		return driver.GrantDatabase
	case "SCHEMA":
		return driver.GrantSchema
	default:
		return driver.GrantTable
	}
}

// runIn is what account statements go through, and the one place this adapter selects a database.
// A grant names its object in **two** parts — `ON OBJECT::[dbo].[orders]` — so unlike every other
// statement here it does not carry the database it is about, and `CREATE USER` is about the
// database it runs in by definition.
//
// It is only ever reached outside a transaction: `sqlkit.Auth` runs everything inside an open one on
// the transaction's own connection, which is already where the reader put it.
func (s *session) runIn(ctx context.Context, database, statement string) error {
	if database == "" {
		return s.run(ctx, "", statement)
	}

	conn, release, err := s.Pinned(ctx, driver.Namespace{Database: database})
	if err != nil {
		return err
	}
	defer release()

	if _, err := conn.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
