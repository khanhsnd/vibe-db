package oracle

import (
	"context"
	"database/sql"
	"errors"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Everything about *who* an Oracle service knows. The statements are `sqlkit.Auth`'s, which this
// session embeds; what is here is catalogue reads.
//
// **A user is a schema.** Making one here adds a row to the namespace list the tree draws, and
// dropping one takes every table in it — which is why the drop carries `CASCADE` and why there is
// no rename at all: the name is on every object inside it, and Oracle has no statement that moves
// them. `sqlkit.Auth` answers `UNSUPPORTED` naming it rather than leaving one for the server.
//
// **Every read here has two spellings.** The `dba_` views are the whole truth and need a privilege
// an ordinary schema owner has no reason to hold; the `user_` and `all_` views are what that owner
// may see, which is themselves. Both are tried, in that order, so a reader who is not a DBA is shown
// their own account rather than an error — the same choice this adapter already makes for the server
// version.

// objectPrivileges is what may be granted on a table or a view. It is a fixed list because Oracle
// enumerates its *system* privileges in a catalogue and its object privileges nowhere: there is a
// `system_privilege_map` and no `object_privilege_map`.
var objectPrivileges = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "ALTER", "INDEX", "REFERENCES",
	"READ", "EXECUTE", "DEBUG", "FLASHBACK", "ON COMMIT REFRESH", "QUERY REWRITE",
}

func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	const asDBA = `
		SELECT username AS "Name",
		       'user' AS "Kind",
		       account_status AS "Status",
		       default_tablespace AS "Tablespace",
		       profile AS "Profile",
		       TO_CHAR(created, 'YYYY-MM-DD') AS "Created",
		       TO_CHAR(expiry_date, 'YYYY-MM-DD') AS "Password Expires"
		FROM dba_users
		UNION ALL
		SELECT role, 'role', NULL, NULL, NULL, NULL, NULL FROM dba_roles
		ORDER BY 1`

	if result, err := sqlkit.Read(ctx, s.db, asDBA, 0, nil); err == nil {
		return accountList(result), nil
	}

	// What an ordinary account may see: the user names the catalogue will admit to, and the roles
	// this session actually holds. Fewer columns because it knows fewer things.
	const asUser = `
		SELECT username AS "Name", 'user' AS "Kind" FROM all_users
		UNION ALL
		SELECT role, 'role' FROM session_roles
		ORDER BY 1`

	result, err := sqlkit.Read(ctx, s.db, asUser, 0, nil)
	if err != nil {
		return driver.AccountList{}, translateError(err)
	}
	return accountList(result), nil
}

func accountList(result driver.QueryResult) driver.AccountList {
	return driver.AccountList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.AccountRefs(result.Columns, result.Rows, "Name", ""),
	}
}

func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	detail := driver.AccountDetail{AccountRef: ref, Attributes: map[string]string{}}

	const query = `
		SELECT account_status, default_tablespace, temporary_tablespace, profile
		FROM dba_users WHERE username = :1`

	var status, tablespace, temporary, profile string
	err := s.db.QueryRowContext(ctx, query, ref.Name).
		Scan(&status, &tablespace, &temporary, &profile)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// It may still be a role, which lives in a catalogue of its own and carries none of these.
		if known, roleErr := s.isRole(ctx, ref.Name); roleErr == nil && known {
			detail.Role = true
			break
		}
		return driver.AccountDetail{}, domain.ErrUnknownAccount
	case err != nil:
		// Not a DBA. The attributes are unreadable and are left empty rather than guessed at; the
		// grants below are still readable through the `user_` views.
	default:
		detail.Attributes["tablespace"] = tablespace
		detail.Attributes["temporaryTablespace"] = temporary
		detail.Attributes["profile"] = profile
		detail.Attributes["locked"] = yesNo(status == "LOCKED" || status == "EXPIRED & LOCKED")
		detail.Attributes["expired"] = yesNo(status == "EXPIRED" || status == "EXPIRED & LOCKED")
	}

	if detail.Grants, err = s.grantsOf(ctx, ref.Name); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Roles, err = s.rolesOf(ctx, ref.Name); err != nil {
		return driver.AccountDetail{}, err
	}
	detail.Source = s.grantedDDL(ctx, ref.Name)
	return detail, nil
}

func (s *session) isRole(ctx context.Context, name string) (bool, error) {
	roles, err := s.names(ctx, "SELECT role FROM dba_roles WHERE role = :1", name)
	if err != nil {
		return false, err
	}
	return len(roles) > 0, nil
}

// grantsOf reads the three levels Oracle grants at. There is no database level and no schema level:
// a schema *is* a user here, so what another engine grants on a schema is granted here either as a
// system privilege — `SELECT ANY TABLE` — or table by table.
func (s *session) grantsOf(ctx context.Context, name string) ([]driver.Grant, error) {
	grants, err := s.systemGrants(ctx, name)
	if err != nil {
		return nil, err
	}

	objects, err := s.objectGrants(ctx, name)
	if err != nil {
		return grants, nil
	}
	return append(grants, objects...), nil
}

func (s *session) systemGrants(ctx context.Context, name string) ([]driver.Grant, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT privilege, admin_option FROM dba_sys_privs WHERE grantee = :1 ORDER BY privilege",
		name)
	if err != nil {
		rows, err = s.db.QueryContext(ctx,
			"SELECT privilege, admin_option FROM user_sys_privs ORDER BY privilege")
		if err != nil {
			return []driver.Grant{}, nil
		}
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var privilege, admin string
		if err := rows.Scan(&privilege, &admin); err != nil {
			return nil, translateError(err)
		}
		grants = append(grants, driver.Grant{
			Level:     driver.GrantServer,
			Privilege: privilege,
			Grantable: admin == "YES",
		})
	}
	return grants, translateError(rows.Err())
}

// objectGrants reads what this account holds on somebody else's tables, and on their columns. The
// owner is what a Namespace's Database holds on this engine — Oracle has no level below its service,
// so the schema is the one name a table is qualified by.
func (s *session) objectGrants(ctx context.Context, name string) ([]driver.Grant, error) {
	const query = `
		SELECT owner, table_name, privilege, grantable, ' ' AS column_name
		FROM dba_tab_privs WHERE grantee = :1
		UNION ALL
		SELECT owner, table_name, privilege, grantable, column_name
		FROM dba_col_privs WHERE grantee = :1
		ORDER BY 1, 2, 3, 5`

	rows, err := s.db.QueryContext(ctx, query, name, name)
	if err != nil {
		return []driver.Grant{}, nil
	}
	defer rows.Close()

	grants := []driver.Grant{}
	at := map[string]int{}
	for rows.Next() {
		var owner, table, privilege, grantable, column string
		if err := rows.Scan(&owner, &table, &privilege, &grantable, &column); err != nil {
			return nil, translateError(err)
		}

		grant := driver.Grant{
			Level:     driver.GrantTable,
			Namespace: driver.Namespace{Database: owner},
			Object:    table,
			Privilege: privilege,
			Grantable: grantable == "YES",
		}
		if column == " " {
			grants = append(grants, grant)
			continue
		}

		key := owner + "\x00" + table + "\x00" + privilege
		if seen, ok := at[key]; ok {
			grants[seen].Columns = append(grants[seen].Columns, column)
			continue
		}
		grant.Level, grant.Columns = driver.GrantColumn, []string{column}
		at[key] = len(grants)
		grants = append(grants, grant)
	}
	return grants, translateError(rows.Err())
}

func (s *session) rolesOf(ctx context.Context, name string) ([]string, error) {
	roles, err := s.names(ctx,
		"SELECT granted_role FROM dba_role_privs WHERE grantee = :1 ORDER BY granted_role", name)
	if err == nil {
		return roles, nil
	}

	roles, err = s.names(ctx, "SELECT granted_role FROM user_role_privs ORDER BY granted_role")
	if err != nil {
		return []string{}, nil
	}
	return roles, nil
}

// grantedDDL is the server's own words, and Oracle is the one engine that will write them out as
// statements: `DBMS_METADATA` re-spells every grant an account holds as the `GRANT` that would make
// it. Reading somebody else's needs a privilege, so a reader without one is answered with nothing
// rather than with a failure — the Privileges tab beside it is read from the catalogue either way.
func (s *session) grantedDDL(ctx context.Context, name string) []string {
	const query = `
		SELECT DBMS_METADATA.GET_GRANTED_DDL(kind, :1)
		FROM (SELECT 'SYSTEM_GRANT' AS kind FROM dual
		      UNION ALL SELECT 'OBJECT_GRANT' FROM dual
		      UNION ALL SELECT 'ROLE_GRANT' FROM dual)`

	lines, err := s.names(ctx, query, name)
	if err != nil {
		return []string{}
	}
	return lines
}

// AccountOptions reads what this service understands. The system privileges are a **read** —
// `system_privilege_map` is Oracle's own enumeration of them — and the object privileges are the
// fixed list above, that catalogue having no counterpart for them.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: sqlkit.Attributes(dialect),
		Roles:      []string{},
		Hosts:      []string{},
		Databases:  []string{},
	}

	system, err := s.names(ctx, "SELECT name FROM system_privilege_map ORDER BY name")
	if err != nil {
		return driver.AccountOptions{}, translateError(err)
	}

	options.Privileges = []driver.PrivilegeSet{
		{Level: driver.GrantServer, Names: system},
		{Level: driver.GrantTable, Names: objectPrivileges},
		{Level: driver.GrantColumn, Names: []string{"INSERT", "UPDATE", "REFERENCES"}},
	}

	if roles, err := s.names(ctx, "SELECT role FROM dba_roles ORDER BY role"); err == nil {
		options.Roles = roles
	} else if roles, err := s.names(ctx, "SELECT role FROM session_roles ORDER BY role"); err == nil {
		options.Roles = roles
	}
	if schemas, err := s.Databases(ctx); err == nil {
		options.Databases = schemas
	}

	// The two tablespace boxes and the profile box are choices, and their lists are a read: this is
	// the one place an adapter fills in what `sqlkit.Attributes` left empty.
	options.Attributes = s.fillChoices(ctx, options.Attributes)
	return options, nil
}

func (s *session) fillChoices(
	ctx context.Context,
	attributes []driver.AccountAttribute,
) []driver.AccountAttribute {
	tablespaces, err := s.names(ctx, "SELECT tablespace_name FROM user_tablespaces ORDER BY 1")
	if err != nil {
		tablespaces = []string{}
	}
	profiles, err := s.names(ctx, "SELECT DISTINCT profile FROM dba_profiles ORDER BY 1")
	if err != nil {
		profiles = []string{}
	}

	filled := make([]driver.AccountAttribute, 0, len(attributes))
	for _, attribute := range attributes {
		switch attribute.Name {
		case "tablespace", "temporaryTablespace":
			attribute.Choices = tablespaces
		case "profile":
			attribute.Choices = profiles
		}
		filled = append(filled, attribute)
	}
	return filled
}

func yesNo(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
