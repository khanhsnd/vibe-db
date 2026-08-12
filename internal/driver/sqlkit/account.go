package sqlkit

import (
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// PasswordPlaceholder is what stands where a password literal will go, wherever the statement is
// going to be **read** rather than run: the Changes pane, the History Log, a saved history.
//
// It is deliberately not a row of asterisks. Asterisks look like a value, and a reader who copied
// the statement out of the preview would send them to the server as one; angle brackets are the one
// thing nobody mistakes for a literal. See Auth for why the preview and the run come from the same
// speller with this as the only difference between them.
const PasswordPlaceholder = "<password>"

// AccountAttr is one fact an account carries besides its password, together with the clause that
// sets it. It is a **flag** in the sense every other difference in a Dialect is — a handful of
// strings per engine — and it is the dialect's rather than the adapter's for the reason nothing
// outside a driver ever spells a statement.
//
// **The order they are declared in is the order they are written in**, which is what MySQL's grammar
// wants: a resource option comes before a lock, and a clause out of place is refused by the server
// rather than reordered here.
//
// What reaches the *frontend* is `driver.AccountAttribute`, which carries the name, the label and
// the kind of box and no clause at all.
type AccountAttr struct {
	Name  string
	Label string
	Kind  driver.AttributeKind
	// Choices is the fixed list for a choice this engine already knows the whole of. One that has to
	// be read — Oracle's tablespaces — is left empty here and filled by the adapter's AccountOptions,
	// which is the read.
	Choices []string
	Help    string
	// On is the clause that sets this attribute and Off the clause that clears it. A `check` uses
	// both — `ACCOUNT LOCK` against `ACCOUNT UNLOCK`, `SUPERUSER` against `NOSUPERUSER` — and
	// everything else uses On alone, with `%s` where the value goes.
	On  string
	Off string
	// Ident says the value is a **name** rather than a string, so it is quoted as one: Oracle's
	// default tablespace, SQL Server's default schema. A number is never quoted either way.
	Ident bool
	// Login says the clause belongs to the statement that declares the **login** rather than the one
	// that declares the user. It is SQL Server's alone, that engine being the only one where the two
	// are separate objects — see LoginIsSeparate.
	Login bool
	// Bare says the clause is a statement of its own rather than one more item after a `WITH`:
	// SQL Server's `ALTER LOGIN x DISABLE`, which has no `WITH` to join.
	Bare bool
}

// clause is this attribute set to the value given, or the empty string for a value that says
// nothing. A number that is not one contributes **nothing** rather than reaching the statement:
// this is the one place in an account change where text a reader typed would otherwise be pasted
// into DDL, DDL binding no arguments.
func (a AccountAttr) clause(d Dialect, value string) string {
	if a.Kind == driver.AttributeCheck {
		if isAffirmative(value) {
			return a.On
		}
		return a.Off
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	literal := value
	switch {
	case a.Kind == driver.AttributeNumber:
		if !isNumber(value) {
			return ""
		}
	case a.Ident:
		literal = d.QuoteIdent(value)
	default:
		literal = d.QuoteString(value)
	}
	return strings.ReplaceAll(a.On, "%s", literal)
}

// isAffirmative reads the tick a reader left. The wire carries a string because every other
// attribute does, and one grammar for all of them is worth more than a second field.
func isAffirmative(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func isNumber(value string) bool {
	for i, char := range value {
		if char == '-' && i == 0 && len(value) > 1 {
			continue
		}
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

// Attributes is what this engine's account carries, as the **editor** sees it: a name, a label, a
// kind of box and a list to pick from. Not one clause of it travels — the statement that sets an
// attribute is spelled here and nowhere else, which is why `AccountAttr` and `driver.AccountAttribute`
// are two types rather than one shared across the boundary.
//
// An adapter with a list to read — Oracle's tablespaces — fills the choices in afterwards, that
// being the read.
func Attributes(d Dialect) []driver.AccountAttribute {
	attributes := make([]driver.AccountAttribute, 0, len(d.AccountAttrs))
	for _, attr := range d.AccountAttrs {
		attributes = append(attributes, driver.AccountAttribute{
			Name:    attr.Name,
			Label:   attr.Label,
			Kind:    attr.Kind,
			Choices: attr.Choices,
			Help:    attr.Help,
		})
	}
	return attributes
}

// AccountRefs reads one reference per row out of an account list, by the **names** of the columns
// that hold them rather than by their places. A list is the engine's own columns in the engine's own
// order, so a reference read off the first column would break the day one was added in front of it —
// the same reason a comparison matches a field by its name.
//
// `host` is empty on every engine but MySQL, and a column this list has not got contributes an empty
// string rather than shifting what the next one means.
func AccountRefs(
	columns []driver.Column,
	rows [][]*string,
	name, host string,
) []driver.AccountRef {
	at := func(wanted string) int {
		if wanted == "" {
			return -1
		}
		for i, column := range columns {
			if strings.EqualFold(column.Name, wanted) {
				return i
			}
		}
		return -1
	}

	nameAt, hostAt := at(name), at(host)
	refs := make([]driver.AccountRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, driver.AccountRef{
			Name: cellText(row, nameAt),
			Host: cellText(row, hostAt),
		})
	}
	return refs
}

func cellText(row []*string, at int) string {
	if at < 0 || at >= len(row) || row[at] == nil {
		return ""
	}
	return *row[at]
}

// AccountName writes the account the way this engine's statements address it. It is the one name in
// the whole of this package that is not always an identifier: MySQL's account **is** the pair
// `'name'@'host'`, and both halves of it are string literals.
//
// A missing host is `%`, which is what MySQL itself defaults `CREATE USER 'a'` to — spelling it out
// keeps the statement the reader sees the same as the account the server ends up with.
func (d Dialect) AccountName(ref driver.AccountRef) string {
	if !d.AccountHasHost {
		return d.QuoteIdent(ref.Name)
	}

	host := ref.Host
	if host == "" {
		host = "%"
	}
	return d.QuoteString(ref.Name) + "@" + d.QuoteString(host)
}

// AccountPassword is the clause that sets one, and `secret` is the whole difference between what
// the reader is shown and what the server is sent. An engine with no such clause, or a change that
// set no password, spells nothing.
func (d Dialect) AccountPassword(password string, secret bool) string {
	if d.PasswordClause == "" || password == "" {
		return ""
	}

	literal := PasswordPlaceholder
	if secret {
		// Oracle takes a password as a quoted *name*, everybody else as a string. It is the same
		// difference `Ident` draws for an attribute, and the same reason: the engine's grammar.
		if d.PasswordIsIdentifier {
			literal = d.QuoteIdent(password)
		} else {
			literal = d.QuoteString(password)
		}
	}
	return strings.ReplaceAll(d.PasswordClause, "%s", literal)
}

// accountClauses is every attribute the change spoke about, in the order this dialect declares
// them. `login` picks which of the two statements they belong to, which is a question only SQL
// Server asks; `bare` picks the ones that stand alone rather than joining a WITH.
//
// An attribute the change never mentioned contributes nothing — it is a fact the change is silent
// about, not one it clears.
func (d Dialect) accountClauses(attributes map[string]string, login, bare bool) []string {
	clauses := []string{}
	for _, attr := range d.AccountAttrs {
		if attr.Login != login || attr.Bare != bare {
			continue
		}
		value, mentioned := attributes[attr.Name]
		if !mentioned {
			continue
		}
		if clause := attr.clause(d, value); clause != "" {
			clauses = append(clauses, clause)
		}
	}
	return clauses
}

// CreateAccount spells declaring one. It answers with a **list** because on SQL Server it is two
// statements: a login belongs to the server and a user to a database, and one without the other is
// either an account that cannot sign in or one that can sign in and reach nothing.
func (d Dialect) CreateAccount(change driver.AccountChange, secret bool) []string {
	name := d.AccountName(change.Account)
	password := d.AccountPassword(change.Password, secret)

	if d.LoginIsSeparate {
		statements := []string{}
		login := "CREATE LOGIN " + name +
			d.withClauses(append(clausesOf(password), d.accountClauses(change.Attributes, true, false)...))
		statements = append(statements, login)
		statements = append(statements, d.bareStatements("ALTER LOGIN "+name, change.Attributes, true)...)

		user := "CREATE USER " + name + " FOR LOGIN " + name +
			d.withClauses(d.accountClauses(change.Attributes, false, false))
		statements = append(statements, user)
		return append(statements, d.bareStatements("ALTER USER "+name, change.Attributes, false)...)
	}

	clauses := append(clausesOf(password), d.accountClauses(change.Attributes, false, false)...)
	statement := "CREATE " + d.accountKeyword() + " " + name
	if d.AttributesInWith {
		return []string{statement + d.withClauses(clauses)}
	}
	return []string{statement + clauseList(clauses)}
}

// AlterAccount spells changing one: its password, its attributes, or both. A change that says
// nothing spells nothing rather than an `ALTER` with no clauses, which every engine refuses.
func (d Dialect) AlterAccount(change driver.AccountChange, secret bool) []string {
	name := d.AccountName(change.Account)
	password := d.AccountPassword(change.Password, secret)

	if d.LoginIsSeparate {
		statements := []string{}
		if login := append(clausesOf(password), d.accountClauses(change.Attributes, true, false)...); len(login) > 0 {
			statements = append(statements, "ALTER LOGIN "+name+d.withClauses(login))
		}
		statements = append(statements, d.bareStatements("ALTER LOGIN "+name, change.Attributes, true)...)
		if user := d.accountClauses(change.Attributes, false, false); len(user) > 0 {
			statements = append(statements, "ALTER USER "+name+d.withClauses(user))
		}
		return append(statements, d.bareStatements("ALTER USER "+name, change.Attributes, false)...)
	}

	clauses := append(clausesOf(password), d.accountClauses(change.Attributes, false, false)...)
	if len(clauses) == 0 {
		return nil
	}

	statement := "ALTER " + d.accountKeyword() + " " + name
	if d.AttributesInWith {
		return []string{statement + d.withClauses(clauses)}
	}
	return []string{statement + clauseList(clauses)}
}

// RenameAccount spells renaming one, and answers with nothing on an engine that cannot: Oracle has
// no such statement at all, an account there being a schema and a schema's name being on every
// object in it. The run of that same change is where Unsupported is raised.
func (d Dialect) RenameAccount(ref driver.AccountRef, name string) []string {
	if d.RenameAccountSQL == "" || name == "" {
		return nil
	}

	renamed := ref
	renamed.Name = name
	return []string{strings.NewReplacer(
		"%s", d.AccountName(ref),
		"%t", d.AccountName(renamed),
	).Replace(d.RenameAccountSQL)}
}

// DropAccount spells removing one — two statements on the engine whose login and user are two
// objects, and the user goes first: a login still owning a user in a database cannot be dropped.
func (d Dialect) DropAccount(ref driver.AccountRef) []string {
	name := d.AccountName(ref)
	if d.LoginIsSeparate {
		return []string{"DROP USER " + name, "DROP LOGIN " + name}
	}
	return []string{"DROP " + d.accountKeyword() + " " + name + d.DropAccountSuffix}
}

// GrantTarget is the `ON …` of a grant, and the four shapes it comes in are the four engines'
// grammars for one idea:
//
//   - **MySQL** names everything with a wildcard — `*.*`, `db.*`, `db.tbl` — so a grant on a
//     database is the database and a star.
//   - **PostgreSQL** names the *class* of the thing in words: `ON DATABASE`, `ON SCHEMA`, `ON TABLE`.
//   - **SQL Server** names it with a class prefix — `ON OBJECT::`, `ON SCHEMA::` — and has no clause
//     at all at database or server level, a grant there running inside the database it is about.
//   - **Oracle** names the object and nothing else, and a system privilege names nothing.
//
// A **two-part** name is used where the class prefix is, for the reason a trigger takes one there:
// a grant is run inside its database, so the third part would name the database twice.
func (d Dialect) GrantTarget(on driver.Grant) string {
	switch {
	case d.GrantsByWildcard:
		switch on.Level {
		case driver.GrantServer:
			return " ON *.*"
		case driver.GrantDatabase, driver.GrantSchema:
			return " ON " + d.QuoteIdent(namespaceName(on.Namespace)) + ".*"
		default:
			return " ON " + d.Table(on.Namespace, on.Object)
		}

	case d.GrantsOnTable:
		switch on.Level {
		case driver.GrantServer:
			return ""
		case driver.GrantDatabase:
			return " ON DATABASE " + d.QuoteIdent(on.Database)
		case driver.GrantSchema:
			return " ON SCHEMA " + d.QuoteIdent(on.Schema)
		default:
			return " ON TABLE " + d.Table(on.Namespace, on.Object)
		}

	case d.GrantsByClass:
		switch on.Level {
		case driver.GrantServer, driver.GrantDatabase:
			return ""
		case driver.GrantSchema:
			return " ON SCHEMA::" + d.QuoteIdent(on.Schema)
		default:
			return " ON OBJECT::" + d.Qualify(on.Schema, on.Object)
		}

	default:
		if on.Level == driver.GrantServer || on.Object == "" {
			return ""
		}
		return " ON " + d.Table(on.Namespace, on.Object)
	}
}

// Grant spells one grant, revoke or denial: the verb, the privileges it carries, what they are on,
// and who they are for. `privileges` is a list because several granted on one thing are **one**
// statement — that is what a reader typing it would write, and it is the only place the one-privilege
// -to-a-Grant rule is undone.
//
// A revoke does not carry the grant option: revoking a privilege revokes the right to pass it on
// with it, so `WITH GRANT OPTION` on a revoke would either say nothing or say something narrower
// than the reader asked for.
func (d Dialect) Grant(
	action driver.AccountChangeKind,
	privileges []string,
	on driver.Grant,
	ref driver.AccountRef,
) string {
	if len(privileges) == 0 {
		return ""
	}

	verb, preposition := "GRANT", " TO "
	switch action {
	case driver.ChangeRevoke:
		verb, preposition = "REVOKE", " FROM "
	case driver.ChangeDeny:
		verb = "DENY"
	}

	// Columns sit after the privilege on three engines and after the object on SQL Server, which is
	// the one difference in this statement that is about *placement* rather than about words.
	columns, afterObject := "", ""
	if len(on.Columns) > 0 {
		list := " (" + strings.Join(d.QuoteAll(on.Columns), ", ") + ")"
		if d.ColumnsAfterObject {
			afterObject = list
		} else {
			columns = list
		}
	}

	statement := verb + " " + strings.Join(privileges, ", ") + columns +
		d.GrantTarget(on) + afterObject + preposition + d.AccountName(ref)

	if action == driver.ChangeGrant && on.Grantable {
		statement += d.grantOption(on.Level)
	}
	return statement
}

// GrantRole spells an account being made a member of one. Two shapes: every engine but SQL Server
// grants a role the way it grants a privilege, and SQL Server alters the **role** instead — the
// membership being a fact of the role there rather than of the account.
func (d Dialect) GrantRole(role string, ref driver.AccountRef, revoke, admin bool) string {
	if role == "" {
		return ""
	}

	// A role is an account on the engine whose accounts carry a host — MySQL, where a role *is* a
	// user that cannot log in — so it is addressed the way one is. The host is the default, which is
	// what an unqualified role name means to that server anyway.
	named := d.QuoteIdent(role)
	if d.AccountHasHost {
		named = d.AccountName(driver.AccountRef{Name: role})
	}

	if d.RoleByMembership {
		action := "ADD MEMBER "
		if revoke {
			action = "DROP MEMBER "
		}
		return "ALTER ROLE " + named + " " + action + d.AccountName(ref)
	}

	if revoke {
		return "REVOKE " + named + " FROM " + d.AccountName(ref)
	}

	statement := "GRANT " + named + " TO " + d.AccountName(ref)
	if admin && d.RoleAdminOption {
		statement += " WITH ADMIN OPTION"
	}
	return statement
}

// grantOption is the words that let the account pass the privilege on. They are not the same words
// everywhere: a privilege over the *server* is administered rather than granted on Oracle, which is
// the one engine that draws the distinction in its grammar.
func (d Dialect) grantOption(level driver.GrantLevel) string {
	if level == driver.GrantServer && d.SystemAdminOption {
		return " WITH ADMIN OPTION"
	}
	return " WITH GRANT OPTION"
}

func (d Dialect) accountKeyword() string {
	if d.AccountKeyword == "" {
		return "USER"
	}
	return d.AccountKeyword
}

// bareStatements is one statement per attribute that has no WITH to join — SQL Server's
// `ALTER LOGIN x DISABLE`.
func (d Dialect) bareStatements(prefix string, attributes map[string]string, login bool) []string {
	clauses := d.accountClauses(attributes, login, true)
	statements := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		statements = append(statements, prefix+" "+clause)
	}
	return statements
}

// withClauses is the clauses after a `WITH`, and nothing at all for none of them.
func (d Dialect) withClauses(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return " WITH " + strings.Join(clauses, " ")
}

func clauseList(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return " " + strings.Join(clauses, " ")
}

func clausesOf(clause string) []string {
	if clause == "" {
		return []string{}
	}
	return []string{clause}
}

// namespaceName is whichever half of a namespace this engine's one level holds, which is what a
// MySQL grant on a database is addressed by — the database on MySQL and Oracle, the schema on the
// two that have both.
func namespaceName(ns driver.Namespace) string {
	if ns.Database != "" {
		return ns.Database
	}
	return ns.Schema
}
