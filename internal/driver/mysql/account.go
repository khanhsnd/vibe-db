package mysql

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

// Everything about *who* a MySQL server knows, and nothing about how a statement that changes one
// is spelled: that is `sqlkit.Auth`'s, which this session embeds. What is here is catalogue reads
// and one read of the server's own words.
//
// **An account here is the pair.** `'app'@'10.%'` and `'app'@'localhost'` are two accounts with two
// passwords and two sets of grants, which is why every reference carries a host and why the list
// draws one as its own column rather than folding the two into one name.

// columnPrivileges is the whole of what MySQL grants below a table. It is a fixed list because the
// server does not enumerate it: `SHOW PRIVILEGES` reports the context a privilege applies in, and
// no context it reports says "columns" — these four are the ones the grammar accepts there.
var columnPrivileges = []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}

// roleTest is how MySQL itself tells a role from a user, and it is worth saying plainly that it
// cannot really: `CREATE ROLE r` makes a row in `mysql.user` that is locked and expired, and that
// is the whole of the difference. A user somebody locked and whose password has expired reads as a
// role here — which is the **server's** ambiguity and not this app's, and is reported rather than
// guessed around, the alternative being a second rule that disagrees with the engine's.
const roleTest = "account_locked = 'Y' AND password_expired = 'Y'"

// Accounts lists what the server knows. `mysql.user` is the whole truth and needs a privilege to
// read; an account that may not read it falls back to the grant tables, which every account may
// read — of itself. The reader is then shown themselves rather than an error, which is the honest
// answer to "who is on this server" asked by somebody who may only see one.
func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	const query = "SELECT user AS `Name`," +
		" host AS `Host`," +
		" IF(" + roleTest + ", 'role', 'user') AS `Kind`," +
		" plugin AS `Plugin`," +
		" account_locked AS `Locked`," +
		" password_expired AS `Password Expired`," +
		" max_user_connections AS `Max Connections`," +
		" password_last_changed AS `Password Changed`" +
		" FROM mysql.user" +
		" ORDER BY user, host"

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return s.grantedAccounts(ctx)
	}
	return driver.AccountList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.AccountRefs(result.Columns, result.Rows, "Name", "Host"),
	}, nil
}

// grantedAccounts is the list an account with no reach into `mysql.user` can still be given: every
// grantee the grant tables mention. It carries fewer columns because it *knows* fewer things — a
// list padded out with empty columns would claim to have read facts it never saw.
func (s *session) grantedAccounts(ctx context.Context) (driver.AccountList, error) {
	const query = "SELECT DISTINCT" +
		" SUBSTRING_INDEX(SUBSTRING_INDEX(grantee, '@', 1), '''', 2) AS `Name`," +
		" REPLACE(SUBSTRING_INDEX(grantee, '@', -1), '''', '') AS `Host`" +
		" FROM information_schema.user_privileges" +
		" ORDER BY `Name`, `Host`"

	result, err := sqlkit.Read(ctx, s.db, query, 0, nil)
	if err != nil {
		return driver.AccountList{}, translateError(err)
	}
	return driver.AccountList{
		Columns: result.Columns,
		Rows:    result.Rows,
		Refs:    sqlkit.AccountRefs(result.Columns, result.Rows, "Name", "Host"),
	}, nil
}

// Account reads one whole: what `mysql.user` keeps about it, every grant the four grant tables
// hold for it, the roles it is a member of, and `SHOW GRANTS` — the server's own words, which is
// the one part of this a reader auditing an account actually wants.
func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	detail := driver.AccountDetail{AccountRef: ref, Attributes: map[string]string{}}

	query := `
		SELECT account_locked, password_expired, max_user_connections, ` + roleTest + `
		FROM mysql.user
		WHERE user = ? AND host = ?`

	var locked, expired string
	var maxConnections int
	var isRole bool
	err := s.db.QueryRowContext(ctx, query, ref.Name, host(ref)).
		Scan(&locked, &expired, &maxConnections, &isRole)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return driver.AccountDetail{}, domain.ErrUnknownAccount
	case err != nil:
		// An account that cannot read `mysql.user` can still read its own grants, so the detail is
		// answered from those alone rather than refused: what is missing is the attributes, and an
		// empty map is honestly empty.
	default:
		detail.Role = isRole
		detail.Attributes["locked"] = affirmative(locked)
		detail.Attributes["expired"] = affirmative(expired)
		detail.Attributes["maxConnections"] = strconv.Itoa(maxConnections)
	}

	if detail.Grants, err = s.grantsOf(ctx, ref); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Roles, err = s.rolesOf(ctx, ref); err != nil {
		return driver.AccountDetail{}, err
	}
	if detail.Source, err = s.showGrants(ctx, ref); err != nil {
		return driver.AccountDetail{}, err
	}
	return detail, nil
}

// grantsOf reads the four levels MySQL grants at, one query each, in the order a reader reads them:
// the server, then a database, then a table, then a column. One privilege to a Grant, which is what
// makes the difference between this and what the editor draws plain set arithmetic.
func (s *session) grantsOf(ctx context.Context, ref driver.AccountRef) ([]driver.Grant, error) {
	grantee := grantee(ref)
	grants := []driver.Grant{}

	reads := []struct {
		level driver.GrantLevel
		query string
	}{
		{driver.GrantServer, `
			SELECT privilege_type, is_grantable, '', ''
			FROM information_schema.user_privileges WHERE grantee = ?`},
		{driver.GrantDatabase, `
			SELECT privilege_type, is_grantable, table_schema, ''
			FROM information_schema.schema_privileges WHERE grantee = ?`},
		{driver.GrantTable, `
			SELECT privilege_type, is_grantable, table_schema, table_name
			FROM information_schema.table_privileges WHERE grantee = ?`},
	}

	for _, read := range reads {
		found, err := s.readGrants(ctx, read.query, read.level, grantee)
		if err != nil {
			return nil, err
		}
		grants = append(grants, found...)
	}

	// A column grant is read apart because it carries a fifth thing, and because MySQL reports one
	// row per column: two columns of one privilege are two rows here and one Grant to the editor.
	const columns = `
		SELECT privilege_type, is_grantable, table_schema, table_name, column_name
		FROM information_schema.column_privileges
		WHERE grantee = ?
		ORDER BY table_schema, table_name, privilege_type, column_name`

	rows, err := s.db.QueryContext(ctx, columns, grantee)
	if err != nil {
		return grants, nil
	}
	defer rows.Close()

	byPrivilege := map[string]int{}
	for rows.Next() {
		var privilege, grantable, schema, table, column string
		if err := rows.Scan(&privilege, &grantable, &schema, &table, &column); err != nil {
			return nil, translateError(err)
		}
		key := schema + "\x00" + table + "\x00" + privilege
		if at, seen := byPrivilege[key]; seen {
			grants[at].Columns = append(grants[at].Columns, column)
			continue
		}
		byPrivilege[key] = len(grants)
		grants = append(grants, driver.Grant{
			Level:     driver.GrantColumn,
			Namespace: driver.Namespace{Database: schema},
			Object:    table,
			Columns:   []string{column},
			Privilege: privilege,
			Grantable: grantable == "YES",
		})
	}
	return grants, translateError(rows.Err())
}

func (s *session) readGrants(
	ctx context.Context,
	query string,
	level driver.GrantLevel,
	grantee string,
) ([]driver.Grant, error) {
	rows, err := s.db.QueryContext(ctx, query, grantee)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	grants := []driver.Grant{}
	for rows.Next() {
		var privilege, grantable, schema, table string
		if err := rows.Scan(&privilege, &grantable, &schema, &table); err != nil {
			return nil, translateError(err)
		}
		grants = append(grants, driver.Grant{
			Level:     level,
			Namespace: driver.Namespace{Database: schema},
			Object:    table,
			Privilege: privilege,
			Grantable: grantable == "YES",
		})
	}
	return grants, translateError(rows.Err())
}

// rolesOf reads what this account is a member of. `mysql.role_edges` is the only place that fact
// lives — a role on MySQL is an account that does not log in, so nothing about the account row
// itself says which roles it holds.
func (s *session) rolesOf(ctx context.Context, ref driver.AccountRef) ([]string, error) {
	const query = `
		SELECT from_user FROM mysql.role_edges
		WHERE to_user = ? AND to_host = ?
		ORDER BY from_user`

	roles, err := s.names(ctx, query, ref.Name, host(ref))
	if err != nil {
		// Reading the edges needs a privilege the account may not have. Nothing is claimed in that
		// case rather than an empty list being reported as "no roles".
		return []string{}, nil
	}
	return roles, nil
}

// showGrants is the server's own words, and is a read for the reason a definition is one: what
// MySQL says about an account is more complete than what four catalogue tables put back together
// can be, and a reader auditing one is reading exactly this.
func (s *session) showGrants(ctx context.Context, ref driver.AccountRef) ([]string, error) {
	statement := "SHOW GRANTS FOR " + dialect.AccountName(ref)
	lines, err := s.names(ctx, statement)
	if err != nil {
		return []string{}, nil
	}
	return lines, nil
}

// AccountOptions is what the editor draws its boxes and its privilege columns from. The privileges
// are a **read** — `SHOW PRIVILEGES`, which is where this server says what it understands, dynamic
// privileges included — and the levels are read off the context each one applies in.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: sqlkit.Attributes(dialect),
		Roles:      []string{},
		Hosts:      []string{},
		Databases:  []string{},
	}

	privileges, err := s.privileges(ctx)
	if err != nil {
		return driver.AccountOptions{}, err
	}
	options.Privileges = privileges

	if hosts, err := s.names(ctx, "SELECT DISTINCT host FROM mysql.user ORDER BY host"); err == nil {
		options.Hosts = hosts
	}
	// Any account may be granted to any other here, a role being a user that does not log in, so
	// what may be granted as a role is what exists. The host is the default one — a role kept at
	// some other host is granted from a query tab, MySQL's own unqualified role name meaning `@%`.
	if roles, err := s.names(ctx, "SELECT DISTINCT user FROM mysql.user ORDER BY user"); err == nil {
		options.Roles = roles
	}
	if databases, err := s.Databases(ctx); err == nil {
		options.Databases = databases
	}
	return options, nil
}

// privileges reads what this server understands and sorts each one into the levels it may be
// granted at. `SHOW PRIVILEGES` reports a context — `Tables`, `Databases`, `Server Admin` — and a
// privilege that applies to tables applies to a database and to the server above it, a wider grant
// being a superset of a narrower one.
func (s *session) privileges(ctx context.Context) ([]driver.PrivilegeSet, error) {
	rows, err := s.db.QueryContext(ctx, "SHOW PRIVILEGES")
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	server, database, table := []string{}, []string{}, []string{}
	for rows.Next() {
		var name, context, comment string
		if err := rows.Scan(&name, &context, &comment); err != nil {
			return nil, translateError(err)
		}

		privilege := strings.ToUpper(name)
		server = append(server, privilege)
		if strings.Contains(context, "Databases") || strings.Contains(context, "Tables") {
			database = append(database, privilege)
		}
		if strings.Contains(context, "Tables") {
			table = append(table, privilege)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}

	return []driver.PrivilegeSet{
		{Level: driver.GrantServer, Names: server},
		{Level: driver.GrantDatabase, Names: database},
		{Level: driver.GrantTable, Names: table},
		{Level: driver.GrantColumn, Names: columnPrivileges},
	}, nil
}

// grantee is how the grant tables spell an account: `'name'@'host'`, quoted, in one string. It is
// the same pair `AccountName` writes and is not that method — one is a value being compared in a
// WHERE and the other is a name in a statement, and folding them together would put a bound
// argument's job on the speller.
func grantee(ref driver.AccountRef) string {
	return "'" + strings.ReplaceAll(ref.Name, "'", "''") + "'@'" +
		strings.ReplaceAll(host(ref), "'", "''") + "'"
}

// host is the host a reference names, and `%` for one that named none — which is what MySQL itself
// reads a bare `CREATE USER 'a'` as.
func host(ref driver.AccountRef) string {
	if ref.Host == "" {
		return "%"
	}
	return ref.Host
}

// affirmative turns MySQL's `Y`/`N` into the wire's own yes and no, which every attribute speaks.
func affirmative(value string) string {
	if strings.EqualFold(value, "Y") || strings.EqualFold(value, "YES") {
		return "true"
	}
	return "false"
}
