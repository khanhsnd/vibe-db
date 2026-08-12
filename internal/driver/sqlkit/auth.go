package sqlkit

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Auth is the half of an account change that does not vary by engine — which, once the four
// grammars are flags on a Dialect, is the whole of it. An adapter embeds it and hands it one way to
// run a statement; what is left to write per engine is the catalogue read that says who exists and
// what they hold.
//
// It is `DDL`'s arrangement exactly, including the seam that matters most: `exec` asks the open
// transaction before it reaches for a pool, so a `GRANT` made while one is open is inside it and a
// `Rollback` takes it back. Autocommit is off for the whole session or for none of it.
type Auth struct {
	Dialect Dialect
	// Run executes one statement against the named database, which is what a grant needs on the two
	// engines where a privilege belongs to a database rather than to the server. It is also where
	// the adapter's own error translation happens.
	Run func(ctx context.Context, database, statement string) error
	// Translate is the adapter's error map, for what goes through the open transaction rather than
	// through Run.
	Translate func(error) error
	// Tx is the session's manual transaction, and nil is a session that cannot hold one.
	Tx *Txn
}

// AccountStatements is what ApplyAccountChanges will run, spelled for a **reader**: the password is
// a placeholder, since this is what goes on the screen, into the History Log and into a saved
// history. Everything else about it is the statement itself, character for character.
func (x Auth) AccountStatements(changes []driver.AccountChange) []string {
	planned := x.spell(changes, false)
	statements := make([]string, 0, len(planned))
	for _, statement := range planned {
		statements = append(statements, statement.SQL)
	}
	return statements
}

// ApplyAccountChanges runs exactly what AccountStatements showed, with the passwords in place of
// their placeholders, in that order, **stopping at the first failure**. There is no transaction
// around it unless the reader opened one, which is why the preview has to be exact.
//
// Every change is checked against the engine *before* anything runs, so a list holding one this
// engine has no concept of is refused whole rather than half-applied — a rename Oracle cannot make
// would otherwise land after the grants in front of it had already gone through.
func (x Auth) ApplyAccountChanges(ctx context.Context, changes []driver.AccountChange) error {
	for _, change := range changes {
		if err := x.supports(change); err != nil {
			return err
		}
	}

	for _, statement := range x.spell(changes, true) {
		if err := x.exec(ctx, statement.In, statement.SQL); err != nil {
			return err
		}
	}
	return nil
}

// supports refuses a change this engine has no concept of, naming which — never a statement left
// for the server to reject. Two of the nine kinds are not universal: a denial is SQL Server's alone,
// and Oracle has no way to rename an account, a user there being a schema whose name is on every
// object inside it.
func (x Auth) supports(change driver.AccountChange) error {
	switch change.Kind {
	case driver.ChangeDeny:
		if !x.Dialect.HasDeny {
			return domain.Unsupported("denying a privilege: this engine grants and revokes only")
		}
	case driver.ChangeRenameAccount:
		if x.Dialect.RenameAccountSQL == "" {
			return domain.Unsupported("renaming a user")
		}
	}
	return nil
}

// spell turns the changes into the statements that make them. `secret` is the one difference
// between the preview and the run, and it reaches only the password clause.
//
// **Privileges granted on the same thing become one statement.** That is what a reader typing it
// would write — `GRANT SELECT, INSERT ON shop.orders TO 'app'@'%'` rather than two lines — and it is
// the one place the one-privilege-to-a-Grant rule is undone, on the way out where it costs nothing.
func (x Auth) spell(changes []driver.AccountChange, secret bool) []driver.DatabaseStatement {
	statements := []driver.DatabaseStatement{}

	for i := 0; i < len(changes); {
		change := changes[i]

		switch change.Kind {
		case driver.ChangeGrant, driver.ChangeRevoke, driver.ChangeDeny:
			privileges, next := gatherPrivileges(changes, i)
			sql := x.Dialect.Grant(change.Kind, privileges, change.Grant, change.Account)
			statements = appendStatement(statements, change.Grant.Database, sql)
			i = next

		default:
			for _, sql := range x.one(change, secret) {
				statements = appendStatement(statements, change.Account.Database, sql)
			}
			i++
		}
	}
	return statements
}

// one spells a change that is about the account itself rather than about a privilege. Each of them
// may be more than one statement: SQL Server's login and user are two objects and are declared and
// dropped separately, and an attribute with no `WITH` to join is a statement of its own.
func (x Auth) one(change driver.AccountChange, secret bool) []string {
	switch change.Kind {
	case driver.ChangeCreateAccount:
		return x.Dialect.CreateAccount(change, secret)
	case driver.ChangeAlterAccount:
		return x.Dialect.AlterAccount(change, secret)
	case driver.ChangeRenameAccount:
		return x.Dialect.RenameAccount(change.Account, change.Name)
	case driver.ChangeDropAccount:
		return x.Dialect.DropAccount(change.Account)
	case driver.ChangeGrantRole:
		return oneOf(x.Dialect.GrantRole(change.Role, change.Account, false, change.Grant.Grantable))
	case driver.ChangeRevokeRole:
		return oneOf(x.Dialect.GrantRole(change.Role, change.Account, true, false))
	default:
		return nil
	}
}

// exec runs one statement wherever this session's statements currently go: inside the open
// transaction, or through the adapter's own pool. It is DDL.exec, and it is here for the same
// reason — no method above has to know a transaction exists.
func (x Auth) exec(ctx context.Context, database, statement string) error {
	open := x.Tx.Queryer()
	if open == nil {
		return x.Run(ctx, database, statement)
	}
	if _, err := open.ExecContext(ctx, statement); err != nil {
		return x.Translate(err)
	}
	return nil
}

// gatherPrivileges collects the run of changes from `at` that are the same verb, for the same
// account, on the same thing, and answers with their privileges and where the run ends. Anything
// that differs in any of those stops it, so nothing is ever folded into a statement that would say
// something the reader did not ask for.
func gatherPrivileges(changes []driver.AccountChange, at int) ([]string, int) {
	first := changes[at]
	privileges := []string{first.Grant.Privilege}

	next := at + 1
	for ; next < len(changes); next++ {
		if !sameGrantTarget(first, changes[next]) {
			break
		}
		privileges = append(privileges, changes[next].Grant.Privilege)
	}
	return privileges, next
}

func sameGrantTarget(a, b driver.AccountChange) bool {
	return a.Kind == b.Kind &&
		a.Account == b.Account &&
		a.Grant.Level == b.Grant.Level &&
		a.Grant.Namespace == b.Grant.Namespace &&
		a.Grant.Object == b.Grant.Object &&
		a.Grant.Grantable == b.Grant.Grantable &&
		strings.Join(a.Grant.Columns, "\x00") == strings.Join(b.Grant.Columns, "\x00")
}

// appendStatement drops a statement this engine spelled as nothing. A change that spells nothing is
// a change this engine has no clause for, and `supports` has already refused the two that are
// refusals rather than silences.
func appendStatement(
	statements []driver.DatabaseStatement,
	database, sql string,
) []driver.DatabaseStatement {
	if strings.TrimSpace(sql) == "" {
		return statements
	}
	return append(statements, driver.DatabaseStatement{In: database, SQL: sql})
}

func oneOf(statement string) []string {
	if statement == "" {
		return nil
	}
	return []string{statement}
}
