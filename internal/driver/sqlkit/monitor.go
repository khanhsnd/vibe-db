package sqlkit

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Watch is the half of a monitor that does not vary by engine — which, once the two statements are
// flags on a Dialect, is the whole of the killing. An adapter embeds it and hands it one way to run
// a statement; what is left to write per engine is the catalogue read that says what the server is
// doing and what it was started with.
//
// It is `Auth`'s arrangement exactly, including the seam that matters most: `exec` asks the open
// transaction before it reaches for a pool, so a kill made while one is open goes down the same
// connection every other statement of the session is going down. Autocommit is off for the whole
// session or for none of it.
type Watch struct {
	Dialect Dialect
	// Run executes one statement against the named database, which is what every kill here carries
	// empty: a process belongs to the server. It is also where the adapter's own error translation
	// happens.
	Run func(ctx context.Context, database, statement string) error
	// Translate is the adapter's error map, for what goes through the open transaction rather than
	// through Run.
	Translate func(error) error
	// Tx is the session's manual transaction, and nil is a session that cannot hold one.
	Tx *Txn
}

// ProcessStatements is what KillProcesses will run, and is what a confirmation shows the reader:
// there is no Changes pane for a kill, so this **is** the preview — one line per process, in the
// order they were picked.
func (w Watch) ProcessStatements(kills []driver.ProcessKill) []string {
	statements := make([]string, 0, len(kills))
	for _, kill := range kills {
		if statement := w.Dialect.KillProcess(kill); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}

// KillProcesses runs exactly what ProcessStatements showed, in that order, **stopping at the first
// failure**.
//
// Every kill is checked against the engine *before* anything runs, so a list holding one this engine
// cannot make is refused whole rather than half-applied — a cancel SQL Server has no statement for
// would otherwise land after the two in front of it had already gone through.
func (w Watch) KillProcesses(ctx context.Context, kills []driver.ProcessKill) error {
	for _, kill := range kills {
		if err := w.supports(kill); err != nil {
			return err
		}
	}

	for _, kill := range kills {
		if err := w.exec(ctx, w.Dialect.KillProcess(kill)); err != nil {
			return err
		}
	}
	return nil
}

// supports refuses a kill this engine has no statement for, naming which, and refuses a reference
// that is not a process this engine could name at all.
//
// The second of those is not a nicety. A kill is DDL in the sense that matters — it binds no
// arguments — so the id reaches the statement as text, and this is the one place in the whole port
// where a value off the wire would otherwise be pasted into SQL. It is checked here rather than
// trusted because the frontend is not what makes it safe.
func (w Watch) supports(kill driver.ProcessKill) error {
	if !processNumbers(kill.Process, w.Dialect.ProcessIsPair) {
		return domain.ValidationError{Field: "process", Message: "is not a process this server names"}
	}

	if kill.Action == domain.ProcessCancel {
		if w.Dialect.CancelStatementSQL == "" {
			return domain.Unsupported(
				"cancelling one statement: this engine ends the whole connection instead")
		}
		return nil
	}
	if w.Dialect.CloseConnectionSQL == "" {
		return domain.Unsupported("ending a connection")
	}
	return nil
}

// exec runs one statement wherever this session's statements currently go: inside the open
// transaction, or through the adapter's own pool. It is Auth.exec, and it is here for the same
// reason — no method above has to know a transaction exists.
func (w Watch) exec(ctx context.Context, statement string) error {
	open := w.Tx.Queryer()
	if open == nil {
		return w.Run(ctx, "", statement)
	}
	if _, err := open.ExecContext(ctx, statement); err != nil {
		return w.Translate(err)
	}
	return nil
}

// KillProcess spells one kill, and spells **nothing** for an action this engine has no statement for
// or a reference that is not numbers — which is what an empty preview means everywhere in this port,
// and what `Watch.supports` turns into a refusal when the same kill is run.
func (d Dialect) KillProcess(kill driver.ProcessKill) string {
	template := d.CloseConnectionSQL
	if kill.Action == domain.ProcessCancel {
		template = d.CancelStatementSQL
	}
	if template == "" || !processNumbers(kill.Process, d.ProcessIsPair) {
		return ""
	}
	return strings.ReplaceAll(template, "%s", d.ProcessName(kill.Process))
}

// ProcessName writes the process the way this engine's statements address it: a bare number, or —
// on the one engine whose session is the pair — both numbers inside one string literal. It is
// AccountName's counterpart, and it is a method for the same reason: what names a thing is the
// dialect's, never the adapter's.
func (d Dialect) ProcessName(ref driver.ProcessRef) string {
	if d.ProcessIsPair {
		return d.QuoteString(ref.ID + "," + ref.Serial)
	}
	return ref.ID
}

// processNumbers says the reference holds the numbers this engine's statement needs — the id, and
// the serial as well wherever a session is the pair. It is `isNumber` applied where it matters most:
// nothing else stands between a value off the wire and a statement.
func processNumbers(ref driver.ProcessRef, pair bool) bool {
	if !isNumber(ref.ID) {
		return false
	}
	return !pair || isNumber(ref.Serial)
}

// ProcessRefs reads one reference per row out of a process list, by the **names** of the columns
// that hold them rather than by their places — `AccountRefs`, for the same reason: a list is the
// engine's own columns in the engine's own order, so a reference read off the first column would
// break the day one was added in front of it.
//
// `own` is the id this session's own read came back on, and an empty one marks nothing: an engine
// that will not say which connection is asking is answered by not claiming to know.
func ProcessRefs(
	columns []driver.Column,
	rows [][]*string,
	id, serial, own string,
) []driver.ProcessRef {
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

	idAt, serialAt := at(id), at(serial)
	refs := make([]driver.ProcessRef, 0, len(rows))
	for _, row := range rows {
		ref := driver.ProcessRef{ID: cellText(row, idAt), Serial: cellText(row, serialAt)}
		ref.Own = own != "" && ref.ID == own
		refs = append(refs, ref)
	}
	return refs
}
