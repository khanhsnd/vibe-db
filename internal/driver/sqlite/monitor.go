package sqlite

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A SQLite database is a **file**, and a file is not doing anything. There is no server holding
// connections, so there is no process to list and none to end; and there is nothing that was
// *started* with settings, so there is nothing to report as a status or a variable — what a pragma
// answers is a fact about this program's own handle on the file, which is not a server's state under
// another name.
//
// So every one of these is `domain.Unsupported` naming it, for the reason the whole of `account.go`
// in this adapter is: a reader answers "this engine has not got one" by not asking again, while a
// server's rejection reads as something that might work next time. There is no server here to reject
// anything in the first place.
//
// The frontend never asks. `EngineInfo.hasMonitor` is false for this engine, so no `Server Monitor`
// node is drawn under a SQLite connection and no menu offers one.

const noServer = "a server monitor: a SQLite database is a file, with nobody connected to it"

func (*session) Processes(context.Context) (driver.ProcessList, error) {
	return driver.ProcessList{}, domain.Unsupported(noServer)
}

// ProcessStatements spells nothing, which is what an empty preview means everywhere in this port.
func (*session) ProcessStatements([]driver.ProcessKill) []string {
	return nil
}

func (*session) KillProcesses(context.Context, []driver.ProcessKill) error {
	return domain.Unsupported(noServer)
}

func (*session) ServerStatus(context.Context) (driver.ServerFacts, error) {
	return driver.ServerFacts{}, domain.Unsupported(noServer)
}

func (*session) ServerVariables(context.Context) (driver.ServerFacts, error) {
	return driver.ServerFacts{}, domain.Unsupported(noServer)
}

func (*session) ServerMetrics(context.Context) (driver.ServerMetrics, error) {
	return driver.ServerMetrics{}, domain.Unsupported(noServer)
}
