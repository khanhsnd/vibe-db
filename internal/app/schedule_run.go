package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/backup"
	"github.com/khanhsnd/vibe-db/internal/schedule"
	"github.com/khanhsnd/vibe-db/internal/transfer"
)

// doTask is one firing, whichever kind it is: dial the connection the task names, do the work, and
// answer with the three things a run is written down as.
//
// **Every kind here is a bound method the reader's own button calls.** A scheduled backup is
// `RunBackup`, a scheduled export is `RunExport`, a scheduled dump is `RunToolDump`: there is no
// second way to take a backup, which is the whole reason one that ran at two in the morning writes
// exactly the file the reader would have got by pressing the button themselves.
//
// The connection is opened for the run and shut after it. A session the reader has open is *not*
// borrowed: a job that fired while they were half way through a transaction on that connection would
// run inside it — see docs/features/transaction.md — and a backup is not something to make part of
// somebody else's transaction.
func (a *ConnectionAPI) doTask(task schedule.Task) (schedule.Outcome, string, string) {
	sessionID, release, err := a.openScheduled(task.ProfileID)
	if err != nil {
		return taskFailure("Could not connect", err)
	}
	defer release()

	switch task.Kind {
	case schedule.KindBackup:
		return a.taskBackup(sessionID, task)
	case schedule.KindExport:
		return a.taskExport(sessionID, task)
	case schedule.KindDump:
		return a.taskDump(sessionID, task)
	case schedule.KindCompare:
		return a.taskCompare(sessionID, task)
	default:
		return schedule.OutcomeFailed, "Unknown job", "This app has no job of that kind."
	}
}

// openScheduled dials a task's profile and answers with the session and how to shut it.
//
// **Nothing is typed here, because nobody is here.** The password is whatever the credential store
// holds — or a token minted for this connection, where the profile signs in by cloud identity. A
// profile that keeps neither is dialled with none rather than refused: a server that takes no
// password is ordinary, and refusing here would be this app guessing at what the server will say.
// What it says is what the run records.
func (a *ConnectionAPI) openScheduled(profileID string) (string, func(), error) {
	stored, err := a.profiles.Get(profileID)
	if err != nil {
		return "", nil, err
	}

	secrets, err := a.resolveSecrets(stored, "", "", "")
	if err != nil {
		return "", nil, err
	}

	ctx, cancel := a.withTimeout(openTimeout(stored))
	defer cancel()

	opened, err := a.sessions.Open(ctx, stored, secrets)
	if err != nil {
		return "", nil, err
	}
	return opened.ID, func() { _ = a.sessions.Close(opened.ID) }, nil
}

// taskBackup writes one backup into the folder this app keeps, and then prunes — see keepBackups,
// which is the one thing a schedule does that no dialog does.
func (a *ConnectionAPI) taskBackup(
	sessionID string,
	task schedule.Task,
) (schedule.Outcome, string, string) {
	ns := taskNamespace(task)

	result, err := a.RunBackup(sessionID, BackupInput{
		Namespace:     ns,
		Tables:        task.Options.Tables,
		WithStructure: task.Options.WithStructure,
		WithData:      task.Options.WithData,
		Comment:       backupComment(task),
	})
	if err != nil {
		return taskFailure("Backup", err)
	}
	if result.Failed > 0 {
		return schedule.OutcomeFailed, "Backup", lastFailure(result.Messages)
	}

	summary := result.Name
	removed, err := a.keepBackups(sessionID, ns, task.Options.Keep)
	if err != nil {
		// The backup itself is written and is what the reader came for, so a folder that could not
		// be pruned is said in the detail rather than reported as a failed backup.
		_, message := wireFields(err)
		return schedule.OutcomeOK, summary, "Older backups were not deleted: " + message
	}
	if removed > 0 {
		summary += " · " + strconv.Itoa(removed) + " older deleted"
	}
	return schedule.OutcomeOK, summary, ""
}

// backupComment is what the file's own header says about where it came from. The reader's own note
// wins; where they left it empty the task's name stands in, so a backup found months later says
// which schedule wrote it rather than nothing at all.
func backupComment(task schedule.Task) string {
	if strings.TrimSpace(task.Options.Comment) != "" {
		return task.Options.Comment
	}
	return "Scheduled · " + task.Name
}

// keepBackups deletes the oldest backups of this namespace until only `keep` remain, and answers with
// how many went. **0 is all of them**, which is what a task that was never given a number means.
//
// It counts *the folder* and not this task's own files: a backup carries the moment it was taken and
// not the schedule that took it, so the honest promise is the one the dialog makes — how many
// backups this namespace keeps.
func (a *ConnectionAPI) keepBackups(sessionID string, ns Namespace, keep int) (int, error) {
	if keep <= 0 {
		return 0, nil
	}

	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return 0, err
	}

	files, err := a.backups.List(target)
	if err != nil {
		return 0, err
	}
	if len(files) <= keep {
		return 0, nil
	}

	// The list is newest first, so everything past the count is what there is too much of.
	removed := 0
	for _, file := range files[keep:] {
		if err := a.backups.Remove(target, file.Name); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// taskExport writes one file per table into the folder the task names.
//
// It **overwrites**: a table's file is named after the table, so a nightly export is a mirror of the
// namespace rather than a pile of dated copies. That is the export wizard's own behaviour and is not
// changed for having been asked for by a clock — a reader who wants yesterday's kept points a second
// task at a second folder, or schedules a backup, which is the kind that snapshots.
func (a *ConnectionAPI) taskExport(
	sessionID string,
	task schedule.Task,
) (schedule.Outcome, string, string) {
	ns := taskNamespace(task)

	tables, err := a.taskTables(sessionID, ns, task.Options.Tables)
	if err != nil {
		return taskFailure("Export", err)
	}

	result, err := a.RunExport(sessionID, ExportJobInput{
		Namespace: ns,
		Format:    task.Options.Format,
		Tables:    tables,
		Folder:    task.Options.Folder,
		// The wizard's own defaults. A schedule asks no questions about a delimiter: what it writes
		// is read by something that was told what to expect, and a task that offered ten more boxes
		// would be a wizard with a clock on it.
		Options: defaultTransferOptions(),
		MaxRows: task.Options.MaxRows,
	})
	if err != nil {
		return taskFailure("Export", err)
	}

	summary := filesText(len(result.Files)) + " · " + rowsText(result.Rows)
	if result.Failed > 0 {
		return schedule.OutcomeFailed, summary, lastFailure(result.Messages)
	}
	return schedule.OutcomeOK, summary, ""
}

// taskDump runs the engine's *own* dump program into a file of its own, named for the moment it was
// taken — the naming a backup has, and for the same reason: each run is a snapshot, so each run is a
// file, and two of them never write over each other.
func (a *ConnectionAPI) taskDump(
	sessionID string,
	task schedule.Task,
) (schedule.Outcome, string, string) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return taskFailure("Dump", err)
	}

	kit, err := a.tools.Kit(held.Profile.Engine)
	if err != nil {
		return taskFailure("Dump", err)
	}

	path := filepath.Join(task.Options.Folder, dumpFileName(task, kit.Extension))
	result, err := a.RunToolDump(sessionID, ToolDumpInput{
		Namespace:     taskNamespace(task),
		Path:          path,
		Tables:        task.Options.Tables,
		WithStructure: task.Options.WithStructure,
		WithData:      task.Options.WithData,
		Compress:      task.Options.Compress,
	})
	if err != nil {
		return taskFailure("Dump", err)
	}
	if result.Failed {
		return schedule.OutcomeFailed, filepath.Base(path), lastFailure(result.Messages)
	}
	return schedule.OutcomeOK, filepath.Base(path), ""
}

// dumpFileName is `shop-20260813020000.sql`: the namespace, then the moment, in the layout a backup's
// own file name uses — one spelling of "when this was taken", wherever the app writes one.
func dumpFileName(task schedule.Task, extension string) string {
	name := task.Database
	if task.Schema != "" {
		name += "-" + task.Schema
	}
	return fileSafe(name) + "-" + time.Now().Format(backup.NameLayout) + "." + extension
}

// taskCompare reads two tables and answers with what has drifted apart.
//
// A comparison that finds differences is **not a failure**: it is the answer the task was asked for.
// It is `differs`, which the list draws in its own colour — see schedule.Outcome.
func (a *ConnectionAPI) taskCompare(
	sessionID string,
	task schedule.Task,
) (schedule.Outcome, string, string) {
	other := task.Options.Right

	rightID := sessionID
	if other.ProfileID != task.ProfileID {
		opened, release, err := a.openScheduled(other.ProfileID)
		if err != nil {
			return taskFailure("Could not connect to the other side", err)
		}
		defer release()
		rightID = opened
	}

	left := CompareSide{SessionID: sessionID, Namespace: taskNamespace(task), Table: task.Options.Table}
	right := CompareSide{
		SessionID: rightID,
		Namespace: Namespace{Database: other.Database, Schema: other.Schema},
		Table:     other.Table,
	}

	differences := make([]string, 0, 2)

	if task.Options.WithStructure {
		diff, err := a.CompareStructure(left, right)
		if err != nil {
			return taskFailure("Compare", err)
		}
		if count := structureDifferences(diff); count > 0 {
			differences = append(differences, countText(count, "field or index"))
		}
	}

	if task.Options.WithData {
		diff, err := a.CompareData(left, right, task.Options.MaxRows)
		if err != nil {
			return taskFailure("Compare", err)
		}
		if count := rowDifferences(diff); count > 0 {
			differences = append(differences, countText(count, "row"))
		}
	}

	where := task.Options.Table + " ⇄ " + other.Table
	if len(differences) == 0 {
		return schedule.OutcomeOK, where + " · the same", ""
	}
	return schedule.OutcomeDiffers, where + " · " + strings.Join(differences, ", ") + " differ", ""
}

// structureDifferences is everything either side holds that the other does not hold in the same
// shape. `same` is the one status that is not a difference — see compare.Status.
func structureDifferences(diff StructureDiff) int {
	count := 0
	for _, field := range diff.Fields {
		if field.Status != compareSame {
			count++
		}
	}
	for _, index := range diff.Indexes {
		if index.Status != compareSame {
			count++
		}
	}
	for _, key := range diff.ForeignKeys {
		if key.Status != compareSame {
			count++
		}
	}
	return count
}

func rowDifferences(diff DataDiff) int {
	count := 0
	for _, row := range diff.Rows {
		if row.Status != compareSame {
			count++
		}
	}
	return count
}

// compareSame is the one word a comparison uses for a thing that has not changed, spelled as
// compare.Status spells it.
const compareSame = "same"

// taskTables is what a job covers: what the reader ticked, or everything the namespace holds where
// they ticked nothing. It is read at the moment the job fires rather than when it was saved, so a
// table added last week is in tonight's export.
func (a *ConnectionAPI) taskTables(
	sessionID string,
	ns Namespace,
	picked []string,
) ([]string, error) {
	if len(picked) > 0 {
		return picked, nil
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return a.namespaceTables(ctx, sessionID, ns)
}

func taskNamespace(task schedule.Task) Namespace {
	return Namespace{Database: task.Database, Schema: task.Schema}
}

// taskFailure is a job that could not be done, in the words the reader would have been shown had
// they pressed the button themselves — the wire's own message, so a failure does not read differently
// for having happened at two in the morning.
func taskFailure(what string, err error) (schedule.Outcome, string, string) {
	_, message := wireFields(err)
	return schedule.OutcomeFailed, what, message
}

// lastFailure is the line a log ends badly on. The *last* one and not the first: a job that failed
// half way through says most in what it said last.
func lastFailure(messages []TransferMessage) string {
	for at := len(messages) - 1; at >= 0; at-- {
		if messages[at].Error {
			return messages[at].Text
		}
	}
	return "The job did not finish."
}

// defaultTransferOptions is what a scheduled export writes with, and is the same set the wizard
// opens on — see `bridge/transfer.ts`, which spells the identical defaults for the reader.
func defaultTransferOptions() TransferOptions {
	return TransferOptions{
		Encoding:  transfer.DefaultEncoding,
		Delimiter: ",",
		Qualifier: `"`,
		HeaderRow: true,
	}
}

// fileSafe replaces the characters a file name may not hold, rather than dropping them — the rule
// ExportJobInput.pathFor follows, and for the same reason.
func fileSafe(name string) string {
	return strings.Map(func(char rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, char) || char < 0x20 {
			return '_'
		}
		return char
	}, name)
}

func filesText(count int) string {
	if count == 1 {
		return "1 file"
	}
	return strconv.Itoa(count) + " files"
}

func countText(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + " " + noun + "s"
}
