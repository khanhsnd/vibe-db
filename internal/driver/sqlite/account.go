package sqlite

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A SQLite database is a **file**, and a file has nobody to be. There is no server to hold an
// account list, no statement that would declare one, and no privilege to grant: whoever can open
// the file can do everything to it, and whoever cannot can do nothing — which is the operating
// system's answer rather than this engine's.
//
// So every one of these is `domain.Unsupported` naming it, for the reason the whole of `ddl.go` in
// the Redis adapter is: a reader answers "this engine has not got one" by not asking again, while a
// server's rejection reads as something that might work next time. There is no server here to
// reject anything in the first place.
//
// The frontend never asks. `EngineInfo.hasAccounts` is false for this engine, so no `Users` folder
// is drawn under a SQLite connection and no menu offers one.

const noAccounts = "users: a SQLite database is a file, and its permissions are the file's own"

func (*session) Accounts(context.Context) (driver.AccountList, error) {
	return driver.AccountList{}, domain.Unsupported(noAccounts)
}

func (*session) Account(context.Context, driver.AccountRef) (driver.AccountDetail, error) {
	return driver.AccountDetail{}, domain.Unsupported(noAccounts)
}

// AccountOptions has no lists to fill, there being no editor for it to fill. It answers with
// nothing rather than refusing, exactly as DatabaseOptions does on an engine with no dialog: a read
// that asks what is on offer is fairly answered by "nothing".
func (*session) AccountOptions(context.Context) (driver.AccountOptions, error) {
	return driver.AccountOptions{}, nil
}

// AccountStatements spells nothing, which is what an empty preview means everywhere in this port.
func (*session) AccountStatements([]driver.AccountChange) []string {
	return nil
}

func (*session) ApplyAccountChanges(context.Context, []driver.AccountChange) error {
	return domain.Unsupported(noAccounts)
}
