package app

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/khanhsnd/vibe-db/internal/backup"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// backupTimeout is what a whole backup or a whole restore is given, and it is a dump's two hours for
// the same reason: a few hundred thousand rows is a long job that is going fine, and a clock that
// cut it off would be the only thing wrong with it.
const backupTimeout = 2 * time.Hour

// BackupInput is what to write out, and what the reader wants said about it afterwards.
type BackupInput struct {
	Namespace
	// Tables is what the backup covers, and **empty is every table the namespace holds** — which is
	// what Object Selection starts on and what the reader narrows from.
	Tables []string `json:"tables"`
	// WithStructure writes what would declare each table again; WithData writes the rows it holds.
	// Both together are a backup; either alone is still a file that restores, which is why the
	// Advanced tab offers them separately rather than as one three-way choice.
	WithStructure bool `json:"withStructure"`
	WithData      bool `json:"withData"`
	// Comment is the reader's own note, kept in the file's header — see internal/backup.
	Comment string `json:"comment"`
}

// RestoreInput is which backup to run back at the server, and how much of it.
type RestoreInput struct {
	Namespace
	Name string `json:"name"`
	// Tables is which of the backup's tables to run back, and **empty is all of them** — the same
	// thing an empty `BackupInput.Tables` means at the other end. What the file holds is on its
	// first line, so offering the choice costs no read.
	Tables []string `json:"tables"`
	// RunID is minted by the caller so a restore already going can be stopped — Cancel takes it.
	RunID string `json:"runId"`
}

// Backup is one file, as the list draws a row for it. The size is bytes and the time is RFC 3339,
// because how either reads is the reader's locale and belongs on the other side of the bridge.
type Backup struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	ModifiedAt string `json:"modifiedAt"`
	Size       int64  `json:"size"`
	Comment    string `json:"comment"`
	// Tables is what the file holds, read off its own header. It rides on the listing because the
	// listing already read that line: the restore dialog offers a selection without a read of its
	// own, and a file with no header of ours simply names nothing and restores whole.
	Tables []string `json:"tables"`
}

// BackupResult is what a backup or a restore did, in the lines the Message Log shows. It carries
// `TransferMessage` rather than a log of its own because a reader watching a job go past should not
// have to learn a second shape of line for the same thing happening.
type BackupResult struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Failed    int               `json:"failed"`
	ElapsedMs int64             `json:"elapsedMs"`
	Messages  []TransferMessage `json:"messages"`
}

// ListBackups answers with every backup kept for this namespace, newest first.
func (a *ConnectionAPI) ListBackups(sessionID string, ns Namespace) ([]Backup, error) {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return nil, wrapError(err)
	}

	files, err := a.backups.List(target)
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]Backup, 0, len(files))
	for _, file := range files {
		out = append(out, Backup{
			Name:       file.Name,
			Path:       file.Path,
			ModifiedAt: file.Modified.Format(time.RFC3339),
			Size:       file.Size,
			Comment:    file.Comment,
			// Copied into a slice that is never nil, so a file naming no tables reaches the dialog
			// as an empty list rather than as `null` for it to guard against.
			Tables: append([]string{}, file.Tables...),
		})
	}
	return out, nil
}

// RunBackup writes one backup of this namespace and answers with what it did, table by table.
//
// It is a **dump into a folder the app owns** rather than one the reader picks: a backup is found
// again by being where backups are, which is the whole difference between the two. Everything else
// about the file is the dump's — the same statements, written by the same method — so a backup is a
// script and restoring one is running it.
//
// A table the server refuses **stops it**, and the half-written file is removed. A backup that is
// missing a table is the one kind of file whose existence is a lie worth avoiding: it would sit in
// the list looking exactly like a whole one, and be found out only by a restore.
func (a *ConnectionAPI) RunBackup(sessionID string, input BackupInput) (BackupResult, error) {
	target, err := a.backupTarget(sessionID, input.Namespace)
	if err != nil {
		return BackupResult{}, wrapError(err)
	}
	if !input.WithStructure && !input.WithData {
		return BackupResult{}, wrapError(domain.ValidationError{
			Field:   "backup",
			Message: "must hold the structure, the records, or both",
		})
	}

	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return BackupResult{}, wrapError(err)
	}
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return BackupResult{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(backupTimeout)
	defer cancel()

	tables := input.Tables
	if len(tables) == 0 {
		tables, err = a.namespaceTables(ctx, sessionID, input.Namespace)
		if err != nil {
			return BackupResult{}, wrapError(err)
		}
	}

	started := time.Now()

	file, err := a.backups.Create(target, started.Format(backup.NameLayout), backup.Header{
		Server:   held.Profile.Name,
		Database: namespaceText(input.Namespace),
		Created:  started,
		Comment:  input.Comment,
		Tables:   tables,
	})
	if err != nil {
		return BackupResult{}, wrapError(err)
	}

	// The name is read back off the path rather than assumed: a second backup inside one second is
	// given a suffix, and the list would otherwise be told about a file that is not there.
	result := BackupResult{
		Name:     backup.Name(file.Name()),
		Path:     file.Name(),
		Messages: []TransferMessage{transferMessage("Writing " + file.Name())},
	}

	if err := a.writeBackup(ctx, sessionID, schema, file, input, tables, &result); err != nil {
		// The file is removed rather than left behind: see the doc comment above.
		file.Close()
		os.Remove(file.Name())

		result.Messages = append(result.Messages, transferFailure("Backup", err))
		result.Failed++
		result.ElapsedMs = time.Since(started).Milliseconds()
		return result, nil
	}

	result.Messages = append(result.Messages,
		transferMessage("Finished · "+strconv.Itoa(len(tables))+" of "+strconv.Itoa(len(tables))+
			" tables"))
	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

// writeBackup puts every table into the open file and logs each one as it lands. The file is closed
// here rather than by the caller, because a buffered write reports nothing until it is flushed and a
// flush reports nothing about the disk — so the close is where a full volume is finally heard from.
func (a *ConnectionAPI) writeBackup(
	ctx context.Context,
	sessionID string,
	schema driver.Schema,
	file *os.File,
	input BackupInput,
	tables []string,
	result *BackupResult,
) error {
	ns := input.Namespace.toDriver()
	what := dumpWhat{structure: input.WithStructure, data: input.WithData}

	writer := bufio.NewWriter(file)
	for _, table := range tables {
		// The line naming the table goes in ahead of its statements, which is the whole of what a
		// narrowed restore reads — see internal/backup. A dump gets none: it is carried somewhere
		// and run whole, and there is nothing at the far end to narrow it with.
		if err := backup.MarkTable(writer, table); err != nil {
			return err
		}

		rows, err := a.dumpTable(ctx, sessionID, schema, writer, ns, table, what)
		if err != nil {
			return err
		}
		result.Messages = append(result.Messages, transferMessage(table+" · "+rowsText(rows)))
	}

	if err := writer.Flush(); err != nil {
		return err
	}
	return file.Close()
}

// RestoreBackup runs a backup's statements back at the server, or the statements of the tables the
// reader picked out of it.
//
// It is **a file run and reads as one**: the file's own text, split where the driver says a
// statement ends, run in order. Nothing here spells a statement and nothing re-reads what the backup
// holds, which is the whole reason a backup is a script rather than a format of its own.
//
// A narrowing is applied to the **text**, ahead of the split: the file already says where each table
// begins, so keeping some of it is keeping lines. What is left is still a script, and it is still
// the driver that reads where a statement in it ends.
//
// A rejected statement **stops it**, and there is no transaction around it — a restore's statements
// depend on each other, and a table that failed to be created has nothing for the thousand inserts
// behind it to land in. What has already run stays run, which is what the log is for.
func (a *ConnectionAPI) RestoreBackup(sessionID string, input RestoreInput) (BackupResult, error) {
	target, err := a.backupTarget(sessionID, input.Namespace)
	if err != nil {
		return BackupResult{}, wrapError(err)
	}

	path, err := a.backups.Path(target, input.Name)
	if err != nil {
		return BackupResult{}, wrapError(err)
	}

	// The file is read as UTF-8 because that is what a backup is written as. It goes through the
	// same reader a picked script does, so a file that has been edited by hand into something else
	// fails as that dialog's would rather than silently restoring mojibake.
	text, err := readScriptFile(path, "")
	if err != nil {
		return BackupResult{}, wrapError(err)
	}

	statements, err := a.sessions.Split(sessionID, backup.Only(text, input.Tables))
	if err != nil {
		return BackupResult{}, wrapError(err)
	}
	if len(statements) == 0 {
		return BackupResult{}, wrapError(domain.ErrEmptyStatement)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, backupTimeout)
	defer release()

	ran := runStatements(ctx, a.sessions, sessionID, input.Namespace.toDriver(), statements, false)

	result := BackupResult{
		Name:      input.Name,
		Path:      path,
		Failed:    ran.Failed,
		ElapsedMs: ran.ElapsedMs,
		Messages: []TransferMessage{
			transferMessage("Restoring " + restoringText(path, input.Tables, len(statements))),
		},
	}

	// One line per statement would be a hundred thousand lines for a backup with rows in it, so the
	// log says how far it got and quotes only what the server refused — which is the one thing in a
	// restore the reader has to read.
	for _, outcome := range ran.Statements {
		if outcome.Error != "" {
			result.Messages = append(result.Messages,
				TransferMessage{Text: outcome.SQL + ": " + outcome.Error, Error: true})
		}
	}

	result.Messages = append(result.Messages,
		transferMessage(statementsText(len(ran.Statements))+" run"))
	return result, nil
}

// DeleteBackup removes one backup from disk. Nothing on any server is touched, and nothing brings
// the file back — which is why what offers this asks first.
func (a *ConnectionAPI) DeleteBackup(sessionID string, ns Namespace, name string) error {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return wrapError(err)
	}
	return wrapError(a.backups.Remove(target, name))
}

// CopyBackup duplicates one backup beside itself and answers with what the copy is called — which
// is what the tree selects once the list has been re-read.
func (a *ConnectionAPI) CopyBackup(sessionID string, ns Namespace, name string) (string, error) {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return "", wrapError(err)
	}

	copied, err := a.backups.Copy(target, name)
	if err != nil {
		return "", wrapError(err)
	}
	return copied, nil
}

// RenameBackup gives one backup another name. Only the file moves — a backup's header says which
// server and which moment it came off, and neither of those changes because it was retitled.
func (a *ConnectionAPI) RenameBackup(sessionID string, ns Namespace, name, to string) error {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return wrapError(err)
	}
	return wrapError(a.backups.Rename(target, name, to))
}

// ExtractBackupSQL writes a copy of one backup wherever the reader points, and answers with the path
// it wrote. An empty path is a dismissed dialog rather than a failure, exactly as DumpObjects reads.
//
// It is a **copy and not a conversion**: a backup here is already a file of statements, so what is
// extracted is the file. The only thing this adds is a name and a place the reader chose.
func (a *ConnectionAPI) ExtractBackupSQL(sessionID string, ns Namespace, name string) (string, error) {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return "", wrapError(err)
	}

	source, err := a.backups.Path(target, name)
	if err != nil {
		return "", wrapError(err)
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Extract SQL",
		DefaultFilename: name + ".sql",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "SQL script", Pattern: "*.sql"}},
	})
	if err != nil {
		return "", wrapError(err)
	}
	if path == "" {
		return "", nil
	}

	if err := copyFile(source, path); err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// OpenBackupFolder shows this namespace's backup folder in the host's own file manager, which is
// where a reader goes to copy one off the machine or drop one in.
func (a *ConnectionAPI) OpenBackupFolder(sessionID string, ns Namespace) error {
	target, err := a.backupTarget(sessionID, ns)
	if err != nil {
		return wrapError(err)
	}

	dir, err := a.backups.Folder(target)
	if err != nil {
		return wrapError(err)
	}
	return wrapError(openFolder(dir))
}

// backupTarget is which folder this session's namespace keeps its backups in, and is where an engine
// that has no such thing is refused.
//
// A backup is a file of statements that would put a database back, and that is a `domain` rule about
// the engine — `Engine.Backups` — not a list of names kept here. What the reader is told is
// `domain.Unsupported` naming the thing, which is what every other absent capability answers with.
func (a *ConnectionAPI) backupTarget(sessionID string, ns Namespace) (backup.Target, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return backup.Target{}, err
	}
	if !held.Profile.Engine.Backups() {
		return backup.Target{}, domain.UnsupportedError{What: "backups"}
	}

	return backup.Target{
		ProfileID: held.Profile.ID,
		Database:  ns.Database,
		Schema:    ns.Schema,
	}, nil
}

// namespaceText is how a namespace reads in a backup's header: `shop` where the engine has no
// schemas, and `shop/public` where it has.
func namespaceText(ns Namespace) string {
	if ns.Schema == "" {
		return ns.Database
	}
	return ns.Database + "/" + ns.Schema
}

// restoringText is the log's first line: which file, how much of it, and how many statements that
// came to. A restore of the whole file says nothing about tables — there is nothing to say — so the
// line only ever names a narrowing where there was one.
func restoringText(path string, tables []string, statements int) string {
	line := path + " · " + statementsText(statements)
	if len(tables) == 0 {
		return line
	}
	return line + " · " + tablesText(len(tables))
}

func statementsText(count int) string {
	if count == 1 {
		return "1 statement"
	}
	return strconv.Itoa(count) + " statements"
}

func tablesText(count int) string {
	if count == 1 {
		return "1 table"
	}
	return strconv.Itoa(count) + " tables"
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return domain.FileError{Err: err}
	}
	defer in.Close()

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return domain.FileError{Err: err}
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return domain.FileError{Err: err}
	}
	// The close is where a full volume is finally heard from, so it is answered with as well as
	// deferred — the order every other file this app writes is closed in.
	if err := out.Close(); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

// openFolder hands a directory to whatever this desktop opens one with. Wails' own BrowserOpenURL
// refuses a `file:` URL outright, so the host's command is asked for directly.
//
// Windows' explorer answers a success with exit status 1, so what is waited for is the command
// having *started* rather than what it went on to say — which is all this can honestly report
// either way, the folder being opened by another process.
func openFolder(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return domain.FileError{Err: err}
	}

	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("explorer", filepath.Clean(dir))
	case "darwin":
		command = exec.Command("open", dir)
	default:
		command = exec.Command("xdg-open", dir)
	}

	if err := command.Start(); err != nil {
		return domain.FileError{Err: err}
	}
	go func() { _ = command.Wait() }()
	return nil
}
