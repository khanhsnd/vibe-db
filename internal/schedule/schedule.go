// Package schedule keeps the jobs this app runs on a clock: which one, how often it fires, and what
// happened the last few times it did.
//
// **Nothing here runs anything.** A task names a kind of work — a backup, an export, one of the
// engine's own dump programs, a comparison — and `internal/app` turns that into the very same call
// the reader's own button makes, which is the whole reason a scheduled backup and a hand-pressed one
// cannot drift apart. What lives here is when a task is next due and what it has done, because both
// are rules about a task rather than about a database.
package schedule

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Kind is what a task does when it fires. Each one is work one of the app's own dialogs already
// does, and every one of the four only ever **reads** a server: nothing is put back on a schedule,
// because a restore or an import that ran while nobody was looking is the one job whose outcome
// cannot be undone by reading the log afterwards.
type Kind string

const (
	KindBackup  Kind = "backup"
	KindExport  Kind = "export"
	KindDump    Kind = "dump"
	KindCompare Kind = "compare"
)

func (k Kind) Valid() bool {
	switch k {
	case KindBackup, KindExport, KindDump, KindCompare:
		return true
	default:
		return false
	}
}

// Every is the shape of a cadence, and there are four because those are the four questions a reader
// answers about a job: how many minutes apart, at what minute past each hour, at what time each day,
// or on which day of the week. Anything finer is a query the reader is watching, and anything coarser
// than a week is a job whose next run nobody would be able to predict from the list.
type Every string

const (
	EveryMinutes Every = "minutes"
	EveryHourly  Every = "hourly"
	EveryDaily   Every = "daily"
	EveryWeekly  Every = "weekly"
)

// Cadence is how often a task fires. The fields belonging to the other three shapes go unread — one
// struct rather than four, because JSON carries no union and the dialog draws one row of boxes.
//
// It is read in **this machine's own time zone**, because a reader who asks for a backup at 02:00
// means two in the morning where they are, and a task that fired at 02:00 UTC on their laptop would
// be the one thing about a scheduler nobody could explain.
type Cadence struct {
	Every Every `json:"every"`
	// Minutes is the gap for EveryMinutes, in minutes.
	Minutes int `json:"minutes"`
	// Hour and Minute are the moment for EveryDaily and EveryWeekly; Minute alone is the moment
	// past the hour for EveryHourly.
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
	// Weekday is time.Weekday — Sunday is 0 — and is read only by EveryWeekly.
	Weekday int `json:"weekday"`
}

func (c Cadence) Validate() error {
	switch c.Every {
	case EveryMinutes:
		if c.Minutes < 1 {
			return domain.ValidationError{Field: "minutes", Message: "must be at least 1"}
		}
	case EveryWeekly:
		if c.Weekday < 0 || c.Weekday > 6 {
			return domain.ValidationError{Field: "weekday", Message: "is not a day of the week"}
		}
	case EveryHourly, EveryDaily:
	default:
		return domain.ValidationError{Field: "every", Message: "is not a cadence this app knows"}
	}

	if c.Hour < 0 || c.Hour > 23 {
		return domain.ValidationError{Field: "hour", Message: "must be between 0 and 23"}
	}
	if c.Minute < 0 || c.Minute > 59 {
		return domain.ValidationError{Field: "minute", Message: "must be between 0 and 59"}
	}
	return nil
}

// Next is the first moment this cadence fires **strictly after** the one given. Strictly, so that a
// task recording the run it has just done cannot be handed the same moment back and fire again on
// the next tick.
func (c Cadence) Next(after time.Time) time.Time {
	switch c.Every {
	case EveryMinutes:
		return after.Add(time.Duration(max(c.Minutes, 1)) * time.Minute)

	case EveryHourly:
		at := time.Date(after.Year(), after.Month(), after.Day(), after.Hour(), c.Minute, 0, 0,
			after.Location())
		for !at.After(after) {
			at = at.Add(time.Hour)
		}
		return at

	case EveryWeekly:
		at := c.dayAt(after)
		// Eight and not seven: the moment may already have passed *today*, in which case the day a
		// week from now is the answer rather than today.
		for at.Before(after) || at.Equal(after) || at.Weekday() != time.Weekday(c.Weekday) {
			at = at.AddDate(0, 0, 1)
		}
		return at

	default:
		at := c.dayAt(after)
		for !at.After(after) {
			at = at.AddDate(0, 0, 1)
		}
		return at
	}
}

// dayAt is this cadence's time of day on the day the given moment falls in. AddDate rather than a
// duration is what walks it forward, so a day the clocks changed on is still one day.
func (c Cadence) dayAt(on time.Time) time.Time {
	return time.Date(on.Year(), on.Month(), on.Day(), c.Hour, c.Minute, 0, 0, on.Location())
}

// Target names the far side of a comparison: another connection entirely, because comparing a
// staging table against the production one is the whole point and those are two servers.
type Target struct {
	ProfileID string `json:"profileId"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`
}

// Options is everything the four kinds take, in one struct — the arrangement `Cadence` has, and for
// the same reason. What a kind does not read is left alone rather than validated, so switching a task
// from an export to a backup keeps the folder it would go back to.
type Options struct {
	// Tables is what the run covers, and **empty is every table the namespace holds** — the same
	// thing an empty list means to the dialogs these tasks borrow.
	Tables []string `json:"tables"`

	// The backup's, and the dump's where its program splits the two.
	WithStructure bool   `json:"withStructure"`
	WithData      bool   `json:"withData"`
	Comment       string `json:"comment"`
	// Keep is how many of this task's backups to leave on disk, oldest deleted first, and **0 is
	// all of them**. It is the one thing a schedule does that no dialog does: a nightly backup that
	// kept every file would fill the volume, and the reader is not there to prune it.
	Keep int `json:"keep"`

	// The export's and the dump's.
	Format   string `json:"format"`
	Folder   string `json:"folder"`
	MaxRows  int    `json:"maxRows"`
	Compress bool   `json:"compress"`

	// The comparison's: the near side's table, and the whole of the far side.
	Table string `json:"table"`
	Right Target `json:"right"`
}

// Task is one job on the clock.
type Task struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    Kind   `json:"kind"`
	Enabled bool   `json:"enabled"`
	// CatchUp says what to do about a firing time that went by while the app was shut. It is the
	// reader's answer and not a default this package picks, because both are honest: a nightly
	// backup usually wants running late, and an export into a folder somebody watches usually does
	// not. See Store.Arm, which is where the answer is acted on.
	CatchUp bool `json:"catchUp"`

	ProfileID string `json:"profileId"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`

	Cadence Cadence `json:"cadence"`
	Options Options `json:"options"`

	// ArmedAt is when this task was last saved, enabled, or moved past a firing it missed. It is
	// what `Next` counts from where the task has never run, so a daily task saved at ten in the
	// morning waits for tomorrow's two rather than firing the moment it is written down.
	ArmedAt time.Time `json:"armedAt"`
	LastRun time.Time `json:"lastRun"`
	// Runs is what happened the last few times, newest first — see RunsKept.
	Runs []Run `json:"runs,omitempty"`
}

// Outcome is what one firing came to. `Differs` is not a failure: a scheduled comparison is a
// **watch**, and two sides that have drifted apart is exactly what it was asked to notice — reporting
// that as an error would put it beside a server that was unreachable, which is a different sentence
// and a different thing to do about it.
type Outcome string

const (
	OutcomeOK      Outcome = "ok"
	OutcomeFailed  Outcome = "failed"
	OutcomeDiffers Outcome = "differs"
)

// Run is one firing, as the list under a task draws it. `Detail` is the failure's own words — the
// server's, or the program's — and is empty for a run that went through.
type Run struct {
	At        time.Time `json:"at"`
	ElapsedMs int64     `json:"elapsedMs"`
	Outcome   Outcome   `json:"outcome"`
	Summary   string    `json:"summary"`
	Detail    string    `json:"detail,omitempty"`
}

// RunsKept is how many firings a task remembers. It is a handful rather than a history because the
// question a reader brings here is "did last night's run work" — and every statement any of these
// jobs sends is in the History Log already, on the same clock as everything else.
const RunsKept = 20

// Next is when this task fires next: one cadence step after whichever is later — the moment it was
// armed, or the moment it last ran. A task that has never run counts from its arming, which is what
// keeps a job saved at noon from firing at noon.
func (t Task) Next() time.Time {
	after := t.ArmedAt
	if t.LastRun.After(after) {
		after = t.LastRun
	}
	return t.Cadence.Next(after)
}

// Due is whether this task's moment has come. A disabled task never has one: it keeps its cadence and
// its history, and turning it back on arms it afresh rather than firing everything it slept through.
func (t Task) Due(now time.Time) bool {
	return t.Enabled && !now.Before(t.Next())
}

func (t Task) Validate() error {
	if err := domain.RequireName("task", t.Name); err != nil {
		return err
	}
	if !t.Kind.Valid() {
		return domain.ValidationError{Field: "kind", Message: "is not a job this app knows"}
	}
	if t.ProfileID == "" {
		return domain.ValidationError{Field: "connection", Message: "is required"}
	}
	if t.Database == "" {
		return domain.ValidationError{Field: "database", Message: "is required"}
	}
	if err := t.Cadence.Validate(); err != nil {
		return err
	}
	return t.Options.validate(t.Kind)
}

// validate refuses only what the named kind will actually read, so a task carries the answers it was
// given for the other three without being asked about them.
func (o Options) validate(kind Kind) error {
	switch kind {
	case KindBackup:
		if !o.WithStructure && !o.WithData {
			return domain.ValidationError{
				Field:   "backup",
				Message: "must hold the structure, the records, or both",
			}
		}
		if o.Keep < 0 {
			return domain.ValidationError{Field: "keep", Message: "cannot be negative"}
		}

	case KindExport:
		if o.Folder == "" {
			return domain.ValidationError{Field: "folder", Message: "is required"}
		}
		if o.Format == "" {
			return domain.ValidationError{Field: "format", Message: "is required"}
		}

	case KindDump:
		if o.Folder == "" {
			return domain.ValidationError{Field: "folder", Message: "is required"}
		}

	case KindCompare:
		if !o.WithStructure && !o.WithData {
			return domain.ValidationError{
				Field:   "compare",
				Message: "must read the structures, the records, or both",
			}
		}
		if err := domain.RequireNames("table", o.Table, "table", o.Right.Table); err != nil {
			return err
		}
		if o.Right.ProfileID == "" {
			return domain.ValidationError{Field: "connection", Message: "is required"}
		}
		if o.Right.Database == "" {
			return domain.ValidationError{Field: "database", Message: "is required"}
		}
	}
	return nil
}
