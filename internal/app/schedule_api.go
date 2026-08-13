package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/schedule"
)

// TaskInput is one scheduled job as the dialog fills it in — see internal/schedule for what each
// kind reads. The namespace is embedded for the reason every other input's is: one shape for
// "where", wherever the app says it.
type TaskInput struct {
	Namespace
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
	// CatchUp says whether a firing time that went by while the app was shut should still run — see
	// schedule.Task.CatchUp, which is the whole of that rule.
	CatchUp   bool   `json:"catchUp"`
	ProfileID string `json:"profileId"`

	Every   string `json:"every"`
	Minutes int    `json:"minutes"`
	Hour    int    `json:"hour"`
	Minute  int    `json:"minute"`
	Weekday int    `json:"weekday"`

	// Tables is what the job covers, and **empty is every table the namespace holds** — the same
	// thing an empty list means to the dialogs these jobs borrow.
	Tables        []string `json:"tables"`
	WithStructure bool     `json:"withStructure"`
	WithData      bool     `json:"withData"`
	Comment       string   `json:"comment"`
	// Keep is how many of this task's backups to leave on disk; 0 is all of them.
	Keep     int    `json:"keep"`
	Format   string `json:"format"`
	Folder   string `json:"folder"`
	MaxRows  int    `json:"maxRows"`
	Compress bool   `json:"compress"`

	// The comparison's two sides: the near one's table, and the whole of the far one.
	Table string     `json:"table"`
	Right TaskTarget `json:"right"`
}

// TaskTarget is the far side of a scheduled comparison, which carries a connection of its own.
type TaskTarget struct {
	ProfileID string `json:"profileId"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`
}

// Task is one job as the list draws it: everything it was saved with, plus what has happened to it.
// The three answers on the end are read-only — they are the scheduler's, and a save ignores them.
type Task struct {
	TaskInput
	// NextRunAt and LastRunAt are RFC 3339, and LastRunAt is empty for a job that has never fired.
	// How either *reads* is the reader's locale and belongs on the other side of the bridge.
	NextRunAt string `json:"nextRunAt"`
	LastRunAt string `json:"lastRunAt"`
	// Running is whether this task is firing right now, here, in this window.
	Running bool `json:"running"`
	// Runs is what happened the last few times, newest first — see schedule.RunsKept.
	Runs []TaskRun `json:"runs"`
}

// TaskRun is one firing. `Outcome` is `ok`, `failed` or `differs`, and the third is not a failure —
// see schedule.Outcome.
type TaskRun struct {
	TaskID    string `json:"taskId"`
	At        string `json:"at"`
	ElapsedMs int64  `json:"elapsedMs"`
	Outcome   string `json:"outcome"`
	Summary   string `json:"summary"`
	Detail    string `json:"detail"`
}

// ListTasks answers with every scheduled job, in the order the file holds them, each carrying when
// it fires next and what its last few firings came to.
func (a *ConnectionAPI) ListTasks() ([]Task, error) {
	tasks, err := a.tasks.List()
	if err != nil {
		return nil, wrapError(err)
	}
	return a.taskDTOs(tasks), nil
}

// SaveTask writes one job down, minting an id for a new one, and answers with it as it now stands.
//
// It **arms** what it saved, so a job edited at noon does not fire at noon — see
// `schedule.Store.Save`. It runs nothing: the reader who wants to see what a job does presses Run,
// which is the same call the clock makes.
func (a *ConnectionAPI) SaveTask(input TaskInput) (Task, error) {
	saved, err := a.tasks.Save(input.toSchedule(), time.Now())
	if err != nil {
		return Task{}, wrapError(err)
	}
	return a.taskDTO(saved), nil
}

// DeleteTask forgets one job. Nothing it ever wrote is touched — the backups it took are still in
// the backup folder, and the files it exported are still where they landed.
func (a *ConnectionAPI) DeleteTask(id string) error {
	return wrapError(a.tasks.Delete(id))
}

// EnableTask turns one job on or off. Turning one on **arms** it: it waits a whole cadence rather
// than firing once for every moment it slept through.
func (a *ConnectionAPI) EnableTask(id string, enabled bool) (Task, error) {
	saved, err := a.tasks.Enable(id, enabled, time.Now())
	if err != nil {
		return Task{}, wrapError(err)
	}
	return a.taskDTO(saved), nil
}

// RunTask fires one job now, and answers with what it came to.
//
// It is **the same call the clock makes**, which is the whole point of the button: a reader setting
// a nightly backup up finds out this evening whether it works, rather than tomorrow morning. What it
// does not do is move the clock on — a run is a run, and the job's next firing is counted from it
// exactly as a scheduled one would be.
//
// A task already firing is refused rather than started twice, since the two would write the same
// file. Everything else comes back as a **result**: a failure here is the job's, and the reader
// answers it in the log rather than in a dialog that threw it away.
func (a *ConnectionAPI) RunTask(id string) (TaskRun, error) {
	task, err := a.tasks.Get(id)
	if err != nil {
		return TaskRun{}, wrapError(err)
	}

	run, err := a.scheduler.fire(task)
	if err != nil {
		return TaskRun{}, wrapError(err)
	}
	return taskRunDTO(id, run), nil
}

func (in TaskInput) toSchedule() schedule.Task {
	return schedule.Task{
		ID:        in.ID,
		Name:      in.Name,
		Kind:      schedule.Kind(in.Kind),
		Enabled:   in.Enabled,
		CatchUp:   in.CatchUp,
		ProfileID: in.ProfileID,
		Database:  in.Database,
		Schema:    in.Schema,
		Cadence: schedule.Cadence{
			Every:   schedule.Every(in.Every),
			Minutes: in.Minutes,
			Hour:    in.Hour,
			Minute:  in.Minute,
			Weekday: in.Weekday,
		},
		Options: schedule.Options{
			Tables:        in.Tables,
			WithStructure: in.WithStructure,
			WithData:      in.WithData,
			Comment:       in.Comment,
			Keep:          in.Keep,
			Format:        in.Format,
			Folder:        in.Folder,
			MaxRows:       in.MaxRows,
			Compress:      in.Compress,
			Table:         in.Table,
			Right: schedule.Target{
				ProfileID: in.Right.ProfileID,
				Database:  in.Right.Database,
				Schema:    in.Right.Schema,
				Table:     in.Right.Table,
			},
		},
	}
}

func (a *ConnectionAPI) taskDTOs(tasks []schedule.Task) []Task {
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, a.taskDTO(task))
	}
	return out
}

func (a *ConnectionAPI) taskDTO(task schedule.Task) Task {
	runs := make([]TaskRun, 0, len(task.Runs))
	for _, run := range task.Runs {
		runs = append(runs, taskRunDTO(task.ID, run))
	}

	return Task{
		TaskInput: TaskInput{
			Namespace: Namespace{Database: task.Database, Schema: task.Schema},
			ID:        task.ID,
			Name:      task.Name,
			Kind:      string(task.Kind),
			Enabled:   task.Enabled,
			CatchUp:   task.CatchUp,
			ProfileID: task.ProfileID,
			Every:     string(task.Cadence.Every),
			Minutes:   task.Cadence.Minutes,
			Hour:      task.Cadence.Hour,
			Minute:    task.Cadence.Minute,
			Weekday:   task.Cadence.Weekday,

			Tables:        append([]string{}, task.Options.Tables...),
			WithStructure: task.Options.WithStructure,
			WithData:      task.Options.WithData,
			Comment:       task.Options.Comment,
			Keep:          task.Options.Keep,
			Format:        task.Options.Format,
			Folder:        task.Options.Folder,
			MaxRows:       task.Options.MaxRows,
			Compress:      task.Options.Compress,
			Table:         task.Options.Table,
			Right: TaskTarget{
				ProfileID: task.Options.Right.ProfileID,
				Database:  task.Options.Right.Database,
				Schema:    task.Options.Right.Schema,
				Table:     task.Options.Right.Table,
			},
		},
		// A disabled job has no next firing, and saying so is honester than a date it will not keep.
		NextRunAt: momentText(nextRunOf(task)),
		LastRunAt: momentText(task.LastRun),
		Running:   a.scheduler.running(task.ID),
		Runs:      runs,
	}
}

func nextRunOf(task schedule.Task) time.Time {
	if !task.Enabled {
		return time.Time{}
	}
	return task.Next()
}

func taskRunDTO(taskID string, run schedule.Run) TaskRun {
	return TaskRun{
		TaskID:    taskID,
		At:        momentText(run.At),
		ElapsedMs: run.ElapsedMs,
		Outcome:   string(run.Outcome),
		Summary:   run.Summary,
		Detail:    run.Detail,
	}
}

// momentText is how a moment crosses the bridge: RFC 3339, or empty for one that never happened.
// The zero time is not a date the other side should ever have to recognise.
func momentText(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.Format(time.RFC3339)
}
