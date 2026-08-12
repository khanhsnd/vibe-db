package mongo

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Who a MongoDB deployment knows, and what each of them may do. Nothing here touches `sqlkit.Auth`:
// there is no statement to spell, an account being made and changed by **commands** — and a command
// is a document, so nothing in this file quotes anything.
//
// **A user belongs to a database.** `alice` in `shop` and `alice` in `admin` are two accounts, which
// is what `AccountRef.Database` holds here — the same field SQL Server fills for the same kind of
// reason, and the reason a list draws the database as its own column.
//
// **A role is the unit, not a privilege.** Every other engine here grants `SELECT` on a table; this
// one grants `readWrite` on a database, and the privileges inside that role belong to the *role*
// rather than to the account. So the Privileges tab is not drawn at all — `AccountOptions.Privileges`
// is empty — and everything a reader does to an account here is done on the Roles tab. Asking to
// grant a privilege directly answers `domain.Unsupported` naming it, rather than a command left for
// the server to refuse.

func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	// Every user on the deployment, which needs a privilege in `admin`. A reader whose credentials
	// are scoped to one database is answered with that database's users instead of with a failure —
	// the same choice ServerInfo already makes about where to ask.
	users, err := s.usersInfo(ctx, adminDatabase, bson.D{
		{Key: "usersInfo", Value: bson.D{{Key: "forAllDBs", Value: true}}},
	})
	if err != nil {
		users, err = s.usersInfo(ctx, s.databaseName(driver.Namespace{}), bson.D{
			{Key: "usersInfo", Value: 1},
		})
		if err != nil {
			return driver.AccountList{}, err
		}
	}

	list := driver.AccountList{
		Columns: []driver.Column{
			{Name: "Name", Type: "string"},
			{Name: "Database", Type: "string"},
			{Name: "Roles", Type: "string"},
			{Name: "Mechanisms", Type: "string"},
		},
		Rows: [][]*string{},
		Refs: []driver.AccountRef{},
	}

	for _, user := range users {
		roles := strings.Join(roleNames(user.Roles), ", ")
		mechanisms := strings.Join(user.Mechanisms, ", ")
		list.Rows = append(list.Rows, []*string{
			accountCell(user.User), accountCell(user.DB), accountCell(roles), accountCell(mechanisms),
		})
		list.Refs = append(list.Refs, driver.AccountRef{Name: user.User, Database: user.DB})
	}
	return list, nil
}

func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	database := ref.Database
	if database == "" {
		database = s.databaseName(driver.Namespace{})
	}

	users, err := s.usersInfo(ctx, database, bson.D{
		{Key: "usersInfo", Value: bson.D{
			{Key: "user", Value: ref.Name},
			{Key: "db", Value: database},
		}},
		{Key: "showPrivileges", Value: false},
	})
	if err != nil {
		return driver.AccountDetail{}, err
	}
	if len(users) == 0 {
		return driver.AccountDetail{}, domain.ErrUnknownAccount
	}

	user := users[0]
	return driver.AccountDetail{
		AccountRef: driver.AccountRef{Name: user.User, Database: user.DB},
		Attributes: map[string]string{},
		// No Grants: see the note at the top of this file. A role is the unit here.
		Grants: []driver.Grant{},
		Roles:  roleNames(user.Roles),
		// The user document itself, which is the server's own answer and the only complete one:
		// anything else would be this app's summary of it.
		Source: []string{document(user.Raw)},
	}, nil
}

// AccountOptions offers roles and nothing else, there being nothing else to offer: no privilege
// levels, no attributes, and no hosts. A role is named `role@database` because that is what one
// **is** here — `read` on `logs` and `read` on `shop` are two different grants of one word.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: []driver.AccountAttribute{},
		Privileges: []driver.PrivilegeSet{},
		Roles:      []string{},
		Hosts:      []string{},
		Databases:  []string{},
	}

	databases, err := s.Databases(ctx)
	if err != nil {
		return driver.AccountOptions{}, err
	}
	options.Databases = databases

	// The built-in roles are the same in every database and the custom ones are not, so each
	// database is asked for its own. A database that refuses the read contributes nothing rather
	// than stopping the list: a reader scoped to one database still gets that one's roles.
	seen := map[string]bool{}
	for _, database := range databases {
		for _, role := range s.roleNamesIn(ctx, database) {
			if !seen[role] {
				seen[role] = true
				options.Roles = append(options.Roles, role)
			}
		}
	}
	return options, nil
}

// AccountStatements is the commands the changes amount to, spelled as a reader would type them into
// the shell — which is what `AlterStatements` already does in this adapter, and for the same reason:
// there is no SQL here, and a preview that showed one would be showing something that never runs.
//
// The password is a placeholder, exactly as it is on every other engine.
func (s *session) AccountStatements(changes []driver.AccountChange) []string {
	planned := s.commands(changes, false)
	statements := make([]string, 0, len(planned))
	for _, command := range planned {
		statements = append(statements, command.Shell)
	}
	return statements
}

// ApplyAccountChanges runs exactly those commands, in order, stopping at the first failure. What is
// sent is the same `bson.D` the preview was rendered from, so the two cannot drift.
func (s *session) ApplyAccountChanges(ctx context.Context, changes []driver.AccountChange) error {
	for _, change := range changes {
		if err := supports(change); err != nil {
			return err
		}
	}

	for _, command := range s.commands(changes, true) {
		result := s.client.Database(command.Database).RunCommand(ctx, command.Command)
		if err := result.Err(); err != nil {
			return translateError(err)
		}
	}
	return nil
}

// supports refuses what a document store has no notion of. Three of the nine kinds: a user cannot be
// renamed, and a privilege cannot be granted to one — it belongs to a role, and a role is granted.
func supports(change driver.AccountChange) error {
	switch change.Kind {
	case driver.ChangeRenameAccount:
		return domain.Unsupported("renaming a user: a MongoDB user is dropped and made again")
	case driver.ChangeGrant, driver.ChangeRevoke:
		return domain.Unsupported(
			"granting a privilege to a user: a privilege here belongs to a role, and a role is granted")
	case driver.ChangeDeny:
		return domain.Unsupported("denying a privilege")
	}
	return nil
}

// accountCommand is one command and where it goes. The shell text is rendered from the **same**
// document that is sent, which is how the preview and the run are held together here — the
// arrangement `sqlkit.Auth` has, in a grammar with no statements in it.
type accountCommand struct {
	Database string
	Command  bson.D
	Shell    string
}

func (s *session) commands(changes []driver.AccountChange, secret bool) []accountCommand {
	commands := []accountCommand{}

	for _, change := range changes {
		database := change.Account.Database
		if database == "" {
			database = s.databaseName(driver.Namespace{})
		}

		var command bson.D
		switch change.Kind {
		case driver.ChangeCreateAccount:
			command = bson.D{
				{Key: "createUser", Value: change.Account.Name},
				{Key: "pwd", Value: password(change.Password, secret)},
				{Key: "roles", Value: bson.A{}},
			}

		case driver.ChangeAlterAccount:
			if change.Password == "" {
				continue
			}
			command = bson.D{
				{Key: "updateUser", Value: change.Account.Name},
				{Key: "pwd", Value: password(change.Password, secret)},
			}

		case driver.ChangeDropAccount:
			command = bson.D{{Key: "dropUser", Value: change.Account.Name}}

		case driver.ChangeGrantRole:
			command = bson.D{
				{Key: "grantRolesToUser", Value: change.Account.Name},
				{Key: "roles", Value: bson.A{roleDocument(change.Role, database)}},
			}

		case driver.ChangeRevokeRole:
			command = bson.D{
				{Key: "revokeRolesFromUser", Value: change.Account.Name},
				{Key: "roles", Value: bson.A{roleDocument(change.Role, database)}},
			}

		default:
			continue
		}

		commands = append(commands, accountCommand{
			Database: database,
			Command:  command,
			Shell:    shellCommand(database, command),
		})
	}
	return commands
}

// shellCommand renders the command as `db.getSiblingDB("x").runCommand({ … })`, which is what a
// reader would type to send exactly this. It goes through the same renderer every value in this
// adapter is drawn with, so what the preview shows is the document itself.
func shellCommand(database string, command bson.D) string {
	raw, err := bson.Marshal(command)
	if err != nil {
		return ""
	}
	return "db.getSiblingDB(" + quote(database) + ").runCommand(" + document(bson.Raw(raw)) + ")"
}

// password is the value the command carries: the real one on the way to the server, and the
// placeholder on the way to a screen.
func password(value string, secret bool) string {
	if secret {
		return value
	}
	return sqlkit.PasswordPlaceholder
}

// roleDocument reads `role@database` back into the pair MongoDB keeps. A name with no database is
// the one the account itself lives in, which is what the shell does with a bare role name.
func roleDocument(role, database string) bson.D {
	name, in := role, database
	if at := strings.LastIndex(role, "@"); at > 0 {
		name, in = role[:at], role[at+1:]
	}
	return bson.D{{Key: "role", Value: name}, {Key: "db", Value: in}}
}

// accountUser is one row of what `usersInfo` answers with. Raw is kept beside the fields because the
// document itself is what the detail shows — see Account.
type accountUser struct {
	User       string     `bson:"user"`
	DB         string     `bson:"db"`
	Roles      []userRole `bson:"roles"`
	Mechanisms []string   `bson:"mechanisms"`
	Raw        bson.Raw   `bson:"-"`
}

type userRole struct {
	Role string `bson:"role"`
	DB   string `bson:"db"`
}

func (s *session) usersInfo(
	ctx context.Context,
	database string,
	command bson.D,
) ([]accountUser, error) {
	var answer struct {
		Users []bson.Raw `bson:"users"`
	}

	err := s.client.Database(database).RunCommand(ctx, command).Decode(&answer)
	if err != nil {
		return nil, translateError(err)
	}

	users := make([]accountUser, 0, len(answer.Users))
	for _, raw := range answer.Users {
		var user accountUser
		if err := bson.Unmarshal(raw, &user); err != nil {
			continue
		}
		user.Raw = raw
		users = append(users, user)
	}
	return users, nil
}

// roleNamesIn reads what one database offers, built-in roles included — the built-in ones being the
// answer to "what may I grant" as much as the custom ones are.
func (s *session) roleNamesIn(ctx context.Context, database string) []string {
	var answer struct {
		Roles []struct {
			Role string `bson:"role"`
			DB   string `bson:"db"`
		} `bson:"roles"`
	}

	command := bson.D{
		{Key: "rolesInfo", Value: 1},
		{Key: "showBuiltinRoles", Value: true},
	}
	if err := s.client.Database(database).RunCommand(ctx, command).Decode(&answer); err != nil {
		return nil
	}

	names := make([]string, 0, len(answer.Roles))
	for _, role := range answer.Roles {
		names = append(names, role.Role+"@"+role.DB)
	}
	return names
}

func roleNames(roles []userRole) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Role+"@"+role.DB)
	}
	return names
}

func accountCell(value string) *string {
	return &value
}
