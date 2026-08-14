package sqlkit

import (
	"context"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Profile is the half of a profiler that does not vary by engine — which, once the four statements
// are flags on a Dialect, is the whole of the changing. An adapter embeds it and hands it one way to
// run a statement; what is left to write per engine is the catalogue read that says which statements
// ran slowly and how the recording is currently set.
//
// It is `Watch`'s arrangement exactly, including the seam that matters most: `exec` asks the open
// transaction before it reaches for a pool, so a change made while one is open goes down the same
// connection every other statement of the session is going down. Autocommit is off for the whole
// session or for none of it.
type Profile struct {
	Dialect Dialect
	// Run executes one statement against the named database, which every change here carries because
	// one of them needs it: `CREATE EXTENSION` installs into the database it is run in, so a
	// PostgreSQL reader turning the recording on in one database has not turned it on in another.
	// It is also where the adapter's own error translation happens.
	Run func(ctx context.Context, database, statement string) error
	// Translate is the adapter's error map, for what goes through the open transaction rather than
	// through Run.
	Translate func(error) error
	// Tx is the session's manual transaction, and nil is a session that cannot hold one.
	Tx *Txn
}

// ProfileStatements is what ApplyProfileChanges will run, and is what a confirmation shows the
// reader: there is no Changes pane here, so this **is** the preview — one line per change, in the
// order they were asked for.
func (p Profile) ProfileStatements(changes []driver.ProfileChange) []string {
	statements := make([]string, 0, len(changes))
	for _, change := range changes {
		statements = append(statements, p.Dialect.ProfileChange(change)...)
	}
	return statements
}

// ApplyProfileChanges runs exactly what ProfileStatements showed, in that order, **stopping at the
// first failure**.
//
// Every change is checked against the engine *before* anything runs, so a list holding one this
// engine cannot make is refused whole rather than half-applied — a threshold Oracle has no statement
// for would otherwise land after the reset in front of it had already emptied the log.
func (p Profile) ApplyProfileChanges(
	ctx context.Context,
	ns driver.Namespace,
	changes []driver.ProfileChange,
) error {
	for _, change := range changes {
		if err := p.supports(change); err != nil {
			return err
		}
	}

	for _, change := range changes {
		for _, statement := range p.Dialect.ProfileChange(change) {
			if err := p.exec(ctx, ns.Database, statement); err != nil {
				return err
			}
		}
	}
	return nil
}

// supports refuses a change this engine has no statement for, naming which, and refuses a threshold
// that is not a number.
//
// The second of those is not a nicety. A `SET GLOBAL` binds no arguments — the same reason a kill's
// process id is checked — so the number reaches the statement as text, and nothing else stands
// between it and the server.
func (p Profile) supports(change driver.ProfileChange) error {
	switch change.Action {
	case domain.ProfileEnable:
		if len(p.Dialect.ProfileOnSQL) == 0 && p.Dialect.ProfileThresholdSQL == "" {
			return domain.Unsupported("turning slow query recording on: this engine always records")
		}
		if change.ThresholdMs < 0 {
			return domain.ValidationError{Field: "threshold", Message: "is not a number of milliseconds"}
		}
		return nil
	case domain.ProfileDisable:
		if p.Dialect.ProfileOffSQL == "" {
			return domain.Unsupported("turning slow query recording off: this engine always records")
		}
		return nil
	case domain.ProfileReset:
		if p.Dialect.ProfileResetSQL == "" {
			return domain.Unsupported("clearing what was recorded")
		}
		return nil
	default:
		return domain.ValidationError{Field: "action", Message: "is not a change to profiling"}
	}
}

// exec runs one statement wherever this session's statements currently go: inside the open
// transaction, or through the adapter's own pool. It is Watch.exec, and it is here for the same
// reason — no method above has to know a transaction exists.
func (p Profile) exec(ctx context.Context, database, statement string) error {
	open := p.Tx.Queryer()
	if open == nil {
		return p.Run(ctx, database, statement)
	}
	if _, err := open.ExecContext(ctx, statement); err != nil {
		return p.Translate(err)
	}
	return nil
}

// ProfileChange spells one change, and spells **nothing** for one this engine has no statement for —
// which is what an empty preview means everywhere in this port, and what `Profile.supports` turns
// into a refusal when the same change is run.
//
// Turning the recording on is more than one statement on the engine that keeps its switch and its
// threshold apart, and each of them is its own line: what a preview shows is what the run sends, and
// the run sends them one at a time.
func (d Dialect) ProfileChange(change driver.ProfileChange) []string {
	switch change.Action {
	case domain.ProfileEnable:
		statements := append([]string{}, d.ProfileOnSQL...)
		if threshold := d.threshold(change.ThresholdMs); threshold != "" {
			statements = append(statements, threshold)
		}
		return statements
	case domain.ProfileDisable:
		return oneStatement(d.ProfileOffSQL)
	case domain.ProfileReset:
		return oneStatement(d.ProfileResetSQL)
	default:
		return nil
	}
}

// oneStatement is the list a single spelling makes, and the empty list an engine with no such
// statement makes — the two being what a preview draws and what a run sends.
func oneStatement(statement string) []string {
	if statement == "" {
		return nil
	}
	return []string{statement}
}

// threshold spells the one statement whose number comes from the reader. A threshold of zero is
// **no change to it** rather than "record everything": zero is what a config carries on an engine
// with no such setting, so writing it would let a dialog that never showed the box move it anyway.
func (d Dialect) threshold(milliseconds int) string {
	if d.ProfileThresholdSQL == "" || milliseconds <= 0 {
		return ""
	}
	return strings.ReplaceAll(d.ProfileThresholdSQL, "%s", d.thresholdValue(milliseconds))
}

// thresholdValue writes the number in the unit this engine's own setting is kept in. The reader is
// asked in milliseconds on every engine, so the one that wants seconds is converted here — and
// written as a decimal where the milliseconds do not divide, `long_query_time` taking one.
func (d Dialect) thresholdValue(milliseconds int) string {
	if !d.ProfileThresholdInSeconds {
		return strconv.Itoa(milliseconds)
	}
	if milliseconds%1000 == 0 {
		return strconv.Itoa(milliseconds / 1000)
	}
	return strconv.FormatFloat(float64(milliseconds)/1000, 'f', -1, 64)
}

// SlowQueryRefs reads one reference per row out of a slow list, by the **names** of the columns that
// hold them rather than by their places — `ProcessRefs`, for the same reason: a list is the engine's
// own columns in the engine's own order, so a statement read off a fixed column would break the day
// one was added in front of it.
//
// `complete` is the adapter's own answer about the catalogue it read: false where the engine keeps
// only what it normalized, and never a guess at the text.
func SlowQueryRefs(
	columns []driver.Column,
	rows [][]*string,
	dialect Dialect,
	statement, database string,
	complete bool,
) []driver.SlowQueryRef {
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

	statementAt, databaseAt := at(statement), at(database)
	refs := make([]driver.SlowQueryRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, driver.SlowQueryRef{
			Runnable: dialect.Runnable(
				cellText(row, databaseAt), cellText(row, statementAt), complete),
			Complete: complete,
		})
	}
	return refs
}

// Runnable writes a recorded statement out as something that would **run somewhere else** — in
// another client, in a script, in a colleague's terminal.
//
// It is three parts, and the middle one is why this is the dialect's rather than the frontend's:
//
//   - a **note** where the statement will not run as it stands, because a reader who pastes `$1`
//     into a terminal and is refused has been told nothing by the refusal;
//   - the statement that **enters the database**, which four engines spell four ways and one has no
//     statement for at all — see `EnterDatabaseSQL`;
//   - the statement itself, ended with a semicolon where it has not already got one.
//
// A row whose statement the server did not record answers with nothing rather than with a `USE`
// and a blank line after it.
func (d Dialect) Runnable(database, statement string, complete bool) string {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return ""
	}

	lines := make([]string, 0, 3)
	if !complete {
		lines = append(lines,
			"-- This server records the statement with its values already replaced.",
			"-- Fill the placeholders in before running it.")
	}

	switch {
	case database == "":
	case d.EnterDatabaseSQL != "":
		lines = append(lines, strings.ReplaceAll(d.EnterDatabaseSQL, "%s", d.QuoteIdent(database)))
	default:
		// The engine has no statement that enters a database, so the script says which one it wants
		// and the reader connects to it. Saying nothing would be handing them a statement whose
		// unqualified names resolve wherever they happen to be.
		lines = append(lines, "-- Connect to the database "+d.QuoteIdent(database)+" to run this.")
	}

	if !strings.HasSuffix(statement, ";") {
		statement += ";"
	}
	return strings.Join(append(lines, statement), "\n")
}
