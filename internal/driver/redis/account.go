package redis

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Who a Redis server knows, which is its **ACL** and nothing else. Nothing here touches
// `sqlkit.Auth`: there is no statement to spell, an account being made and changed by one command,
// and that command's arguments are the rules themselves.
//
// **An account is a line.** `ACL LIST` answers with one line per user holding the whole of what that
// user is — whether it is on, what its passwords hash to, which keys it may touch, which channels,
// and which commands — so it is the list, the detail and the server's own words all at once, and
// nothing here asks a second time.
//
// **A privilege is a command category.** There is no table to grant on and no database either: an
// ACL rule reaches the whole server, which is why the one privilege set this engine offers is at
// server level and why its names come from `ACL CAT` — the server's own enumeration. What a rule
// *narrows* — a key pattern, a channel pattern — is an **attribute** rather than a grant, because
// that is what it is: `~cached:*` is not something granted on `cached:*`, it is a bound on the whole
// account.
//
// **There are no roles.** Redis has nothing that is granted to an account in turn, so the Roles tab
// is not drawn and asking answers `domain.Unsupported` naming it.

func (s *session) Accounts(ctx context.Context) (driver.AccountList, error) {
	lines, err := s.aclList(ctx)
	if err != nil {
		return driver.AccountList{}, err
	}

	list := driver.AccountList{
		Columns: []driver.Column{
			{Name: "Name", Type: "string"},
			{Name: "Enabled", Type: "string"},
			{Name: "Keys", Type: "string"},
			{Name: "Channels", Type: "string"},
			{Name: "Commands", Type: "string"},
		},
		Rows: [][]*string{},
		Refs: []driver.AccountRef{},
	}

	for _, line := range lines {
		rule := parseACL(line)
		if rule.Name == "" {
			continue
		}
		list.Rows = append(list.Rows, []*string{
			aclCell(rule.Name),
			aclCell(yesNo(rule.Enabled)),
			aclCell(strings.Join(rule.Keys, " ")),
			aclCell(strings.Join(rule.Channels, " ")),
			aclCell(strings.Join(rule.Commands, " ")),
		})
		list.Refs = append(list.Refs, driver.AccountRef{Name: rule.Name})
	}
	return list, nil
}

func (s *session) Account(
	ctx context.Context,
	ref driver.AccountRef,
) (driver.AccountDetail, error) {
	lines, err := s.aclList(ctx)
	if err != nil {
		return driver.AccountDetail{}, err
	}

	for _, line := range lines {
		rule := parseACL(line)
		if rule.Name != ref.Name {
			continue
		}

		grants := make([]driver.Grant, 0, len(rule.Commands))
		for _, command := range rule.Commands {
			// Only what the account *has*. A `-` rule is the absence of a grant rather than a
			// denial: Redis has two states and not SQL Server's three.
			if strings.HasPrefix(command, "+") {
				grants = append(grants, driver.Grant{
					Level:     driver.GrantServer,
					Privilege: strings.TrimPrefix(command, "+"),
				})
			}
		}

		return driver.AccountDetail{
			AccountRef: driver.AccountRef{Name: rule.Name},
			Attributes: map[string]string{
				"enabled":  yesNo(rule.Enabled),
				"nopass":   yesNo(rule.NoPassword),
				"keys":     strings.Join(rule.Keys, " "),
				"channels": strings.Join(rule.Channels, " "),
			},
			Grants: grants,
			Roles:  []string{},
			// The line itself, which is the server's own answer and the whole of it.
			Source: []string{line},
		}, nil
	}
	return driver.AccountDetail{}, domain.ErrUnknownAccount
}

// AccountOptions offers the command categories `ACL CAT` names, and the four bounds an ACL puts on
// an account. No roles, no hosts, no databases: an ACL rule is the server's and reaches all of them.
func (s *session) AccountOptions(ctx context.Context) (driver.AccountOptions, error) {
	options := driver.AccountOptions{
		Attributes: aclAttributes,
		Roles:      []string{},
		Hosts:      []string{},
		Databases:  []string{},
	}

	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.AccountOptions{}, err
	}

	categories, err := client.Do(ctx, "ACL", "CAT").StringSlice()
	if err != nil {
		return driver.AccountOptions{}, translateError(err)
	}

	names := make([]string, 0, len(categories))
	for _, category := range categories {
		names = append(names, "@"+category)
	}
	options.Privileges = []driver.PrivilegeSet{{Level: driver.GrantServer, Names: names}}
	return options, nil
}

// aclAttributes is what an ACL bounds an account by, as the editor draws it. The two pattern boxes
// take the rules verbatim — `~cached:*`, `&news.*` — because that notation *is* the value, and a
// box that took a plain glob would be hiding which of the two kinds it was setting.
var aclAttributes = []driver.AccountAttribute{
	{
		Name: "enabled", Label: "Enabled", Kind: driver.AttributeCheck,
		Help: "A disabled user exists and cannot log in.",
	},
	{
		Name: "nopass", Label: "No password", Kind: driver.AttributeCheck,
		Help: "Any password is accepted. Setting a password clears this.",
	},
	{
		Name: "keys", Label: "Key patterns", Kind: driver.AttributeText,
		Help: "Space-separated, in the server's own notation: ~cached:* ~session:*, or allkeys.",
	},
	{
		Name: "channels", Label: "Channel patterns", Kind: driver.AttributeText,
		Help: "Space-separated: &news.* , or allchannels.",
	},
}

// AccountStatements is the commands the changes amount to, spelled as a reader would type them into
// the tab beside this one. The password is a placeholder, exactly as it is on every other engine —
// and here it matters twice over, an ACL password being written into the command line itself.
func (s *session) AccountStatements(changes []driver.AccountChange) []string {
	planned := commands(changes, false)
	statements := make([]string, 0, len(planned))
	for _, command := range planned {
		statements = append(statements, command.Text)
	}
	return statements
}

func (s *session) ApplyAccountChanges(ctx context.Context, changes []driver.AccountChange) error {
	for _, change := range changes {
		if err := supports(change); err != nil {
			return err
		}
	}

	client, err := s.client(driver.Namespace{})
	if err != nil {
		return err
	}

	for _, command := range commands(changes, true) {
		if err := client.Do(ctx, command.Args...).Err(); err != nil {
			return translateError(err)
		}
	}
	return nil
}

// supports refuses what an ACL has no notion of. A user cannot be renamed, there is nothing to
// grant a role, and `DENY` has no counterpart: a Redis rule is on or absent, and `-@admin` is the
// absence rather than a third state that outranks anything.
func supports(change driver.AccountChange) error {
	switch change.Kind {
	case driver.ChangeRenameAccount:
		return domain.Unsupported("renaming a user: an ACL user is deleted and set again")
	case driver.ChangeDeny:
		return domain.Unsupported("denying a command: a Redis rule is granted or it is absent")
	case driver.ChangeGrantRole, driver.ChangeRevokeRole:
		return domain.Unsupported("a role: a Redis ACL grants command categories and nothing else")
	}
	return nil
}

// aclCommand is one command and the line it reads as. Both come off the same arguments, which is
// how the preview and the run are held together here.
type aclCommand struct {
	Args []any
	Text string
}

// commands folds every change to one account into **one** `ACL SETUSER`, which is what a reader
// managing an ACL would write: the rules are read left to right and the last one about a thing
// wins, so a create, a password and three grants are one line rather than five.
//
// A drop is its own command, `ACL DELUSER` being a different verb.
func commands(changes []driver.AccountChange, secret bool) []aclCommand {
	planned := []aclCommand{}

	for i := 0; i < len(changes); {
		change := changes[i]
		if change.Kind == driver.ChangeDropAccount {
			planned = append(planned, aclSetUser("ACL", "DELUSER", change.Account.Name))
			i++
			continue
		}

		rules, next := rulesFor(changes, i, secret)
		i = next
		if len(rules) == 0 {
			continue
		}
		planned = append(planned, aclSetUser(append(
			[]string{"ACL", "SETUSER", change.Account.Name}, rules...)...))
	}
	return planned
}

// rulesFor collects the run of changes from `at` that are about the same account and are not drops,
// and answers with the rules they amount to and where the run ends.
func rulesFor(changes []driver.AccountChange, at int, secret bool) ([]string, int) {
	name := changes[at].Account.Name
	rules := []string{}

	next := at
	for ; next < len(changes); next++ {
		change := changes[next]
		if change.Account.Name != name || change.Kind == driver.ChangeDropAccount {
			break
		}

		switch change.Kind {
		case driver.ChangeCreateAccount, driver.ChangeAlterAccount:
			rules = append(rules, accountRules(change, secret)...)
		case driver.ChangeGrant:
			rules = append(rules, "+"+change.Grant.Privilege)
		case driver.ChangeRevoke:
			rules = append(rules, "-"+change.Grant.Privilege)
		}
	}
	return rules, next
}

// accountRules is what a create or an alter says about the account itself, in the order the server
// reads them: whether it is on, then its password, then its bounds.
//
// The two pattern bounds are **reset before they are set**, because an ACL rule adds rather than
// replaces: without the reset, editing `~a` to `~b` would leave an account that could still reach
// `a`, which is not what changing a bound means.
func accountRules(change driver.AccountChange, secret bool) []string {
	rules := []string{}

	if enabled, said := change.Attributes["enabled"]; said {
		rules = append(rules, onOff(enabled))
	}
	if change.Password != "" {
		password := sqlkit.PasswordPlaceholder
		if secret {
			password = change.Password
		}
		rules = append(rules, ">"+password)
	}
	if nopass, said := change.Attributes["nopass"]; said && affirmative(nopass) {
		rules = append(rules, "nopass")
	}
	if keys, said := change.Attributes["keys"]; said {
		rules = append(rules, append([]string{"resetkeys"}, strings.Fields(keys)...)...)
	}
	if channels, said := change.Attributes["channels"]; said {
		rules = append(rules, append([]string{"resetchannels"}, strings.Fields(channels)...)...)
	}
	return rules
}

func aclSetUser(args ...string) aclCommand {
	sent := make([]any, 0, len(args))
	words := make([]string, 0, len(args))
	for _, arg := range args {
		sent = append(sent, arg)
		words = append(words, shellArg(arg))
	}
	return aclCommand{Args: sent, Text: strings.Join(words, " ")}
}

// shellArg writes an argument the way the tab beside this one would read it back: bare unless it
// holds something that would otherwise split it into two.
func shellArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\n\"'") {
		return arg
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(arg) + `"`
}

func (s *session) aclList(ctx context.Context) ([]string, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return nil, err
	}

	lines, err := client.Do(ctx, "ACL", "LIST").StringSlice()
	if err != nil {
		return nil, translateError(err)
	}
	return lines, nil
}

// aclRule is one `ACL LIST` line read apart. The four groups are the four things a rule can say,
// and anything the server writes that is none of them — a selector, `sanitize-payload` — stays in
// the line itself, which the detail shows whole.
type aclRule struct {
	Name       string
	Enabled    bool
	NoPassword bool
	Keys       []string
	Channels   []string
	Commands   []string
}

// parseACL reads one line of `ACL LIST`. The first word is always `user` and the second is the name;
// everything after it is a rule, told apart by the character it starts with.
func parseACL(line string) aclRule {
	words := strings.Fields(line)
	if len(words) < 2 || !strings.EqualFold(words[0], "user") {
		return aclRule{}
	}

	rule := aclRule{Name: words[1]}
	for _, word := range words[2:] {
		switch {
		case strings.EqualFold(word, "on"):
			rule.Enabled = true
		case strings.EqualFold(word, "off"):
			rule.Enabled = false
		case strings.EqualFold(word, "nopass"):
			rule.NoPassword = true
		case word == "allkeys", strings.HasPrefix(word, "~"), strings.HasPrefix(word, "%"):
			rule.Keys = append(rule.Keys, word)
		case word == "allchannels", strings.HasPrefix(word, "&"):
			rule.Channels = append(rule.Channels, word)
		case strings.HasPrefix(word, "+"), strings.HasPrefix(word, "-"):
			rule.Commands = append(rule.Commands, word)
		}
	}
	return rule
}

func onOff(enabled string) string {
	if affirmative(enabled) {
		return "on"
	}
	return "off"
}

func affirmative(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func yesNo(flag bool) string {
	if flag {
		return "true"
	}
	return "false"
}

func aclCell(text string) *string {
	return &text
}
