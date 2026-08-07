// Package dbtool runs the programs an engine's own vendor ships for carrying a whole database out
// to a file and putting it back — mongodump and mongorestore, mysqldump and mysql, pg_dump and
// pg_restore. Which pair an engine has is a `domain` rule (`Engine.Toolkit`); this is where one is
// found on the machine and run.
//
// It is the one place in this app that runs something that is not this app, and three rules follow
// from that:
//
//   - **No secret is ever an argument.** A command line is readable by every process on the
//     machine, so a password travels in a file this process wrote at 0600 and removed afterwards.
//     Every one of the three programs takes one — `--defaults-file`, `PGPASSFILE`, `--config` —
//     and command.go is where each is spelled.
//   - **A program is found, never guessed.** It is looked for on the PATH and in the one folder
//     the reader pointed at, and a program that is not there is `domain.ToolNotFoundError` naming
//     it — never a run that fails with something the shell said.
//   - **What the program prints is what the reader is shown.** mongorestore knows why it refused a
//     document and this app does not, so its own lines are carried through unchanged.
package dbtool

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	secretMode = 0o600

	// maxLines is how many of a program's own lines are carried back. A restore of a large file
	// can print one line per rejected document, and a log nobody can scroll is a log nobody reads.
	// It is never a silent cut: Run says it stopped listening, on a line of its own.
	maxLines = 500

	// maxLineBytes bounds one line. Nothing these three print is longer than a path and a
	// sentence, so a line past this is a program writing its output where its progress goes.
	maxLineBytes = 64 << 10
)

// Program is one of the two, and where it turned out to be. An empty Path is a program this
// machine has not got, which is a thing to say on the dialog rather than a failure to run into.
type Program struct {
	Name string `json:"name"`
	// Path is the runnable file, resolved. Empty is a program that is not here.
	Path string `json:"path"`
	// Chosen is the file the reader pointed at, and is empty where the PATH is what found it. It
	// survives the file going away, which is the one case worth saying out loud: `Chosen` set and
	// `Path` empty is "the copy you picked is gone", not "you never picked one".
	Chosen string `json:"chosen"`
}

func (p Program) Found() bool {
	return p.Path != ""
}

// Kit is both programs of one engine, as this machine holds them. **Both**, always, whichever
// direction the caller is going: a reader looking at where mongodump is wants to see mongorestore
// beside it, and a path that was right last month is exactly the thing that needs changing today.
type Kit struct {
	domain.Toolkit
	Dump    Program
	Restore Program
}

// Programs is which of the pair a caller means. It is a word rather than a boolean because it
// crosses to the frontend and reads there: `dump` and `restore` say which button was pressed.
const (
	DumpProgram    = "dump"
	RestoreProgram = "restore"
)

// settings is what the app remembers about the tools: **one path per program**, and not a folder
// per engine.
//
// A folder was the wrong unit. mysqldump and the mysql client are one download and pg_dump and
// pg_restore are another, but a reader who has built one from source, or has two versions of the
// MongoDB tools, has them in two places — and a folder cannot say which of the two files in it is
// meant. The program's own name is the key, so a path outlives the profile that led to it.
type settings struct {
	Programs map[string]string `json:"programs"`
}

// Locator finds an engine's two programs, and remembers the ones the reader pointed at.
type Locator struct {
	file *jsonstore.File[settings]
}

func New(path string) *Locator {
	return &Locator{file: jsonstore.New[settings](path)}
}

// DefaultPath is one document beside the connection profiles and the query history, so everything
// this app keeps for the reader sits in one place.
func DefaultPath() (string, error) {
	return jsonstore.Path("tools.json")
}

// Kit is both programs of this engine as they stand right now — looked for every time rather than
// remembered, because a reader who installs the tools while the dialog is open should be able to
// press the button rather than restart the app.
func (l *Locator) Kit(engine domain.Engine) (Kit, error) {
	toolkit, ok := engine.Toolkit()
	if !ok {
		return Kit{}, domain.Unsupported("a dump program of its own")
	}

	stored, err := l.read()
	if err != nil {
		return Kit{}, err
	}

	return Kit{
		Toolkit: toolkit,
		Dump:    locate(toolkit.Dump, stored[toolkit.Dump]),
		Restore: locate(toolkit.Restore, stored[toolkit.Restore]),
	}, nil
}

// Choose remembers the file the reader picked for one of the pair, and answers with the kit as it
// stands afterwards so a caller redraws from one call rather than two.
//
// **It sets that one program and nothing else.** Filling the other in from the same directory would
// be this app deciding which mongorestore the reader meant, on the strength of it being next door —
// and a path nobody chose is a path nobody thinks to check when the wrong version runs. The other
// program is pointed at where the other program is asked for.
//
// An empty path forgets this program, which is how a reader goes back to whatever is on the PATH.
func (l *Locator) Choose(engine domain.Engine, which, path string) (Kit, error) {
	toolkit, ok := engine.Toolkit()
	if !ok {
		return Kit{}, domain.Unsupported("a dump program of its own")
	}

	picked := toolkit.Dump
	if which == RestoreProgram {
		picked = toolkit.Restore
	}

	err := l.file.Update(func(stored *settings) error {
		if stored.Programs == nil {
			stored.Programs = map[string]string{}
		}
		if strings.TrimSpace(path) == "" {
			delete(stored.Programs, picked)
			return nil
		}
		stored.Programs[picked] = filepath.Clean(path)
		return nil
	})
	if err != nil {
		return Kit{}, domain.FileError{Err: err}
	}

	return l.Kit(engine)
}

func (l *Locator) read() (map[string]string, error) {
	stored, err := l.file.Read()
	if err != nil {
		return nil, domain.FileError{Err: err}
	}
	return stored.Programs, nil
}

// locate answers where a program really is: the file the reader pointed at, and the PATH where they
// pointed at none. Both go through `exec.LookPath`, which is what checks a path is *runnable* as
// well as present — and what puts `.exe` on the end of a bare name on Windows.
//
// A chosen path that is no longer there comes back **not found but still named**, so the dialog can
// say which file has gone rather than quietly falling back to a different copy on the PATH.
func locate(name, chosen string) Program {
	program := Program{Name: name, Chosen: chosen}
	if name == "" {
		return program
	}

	target := name
	if chosen != "" {
		target = chosen
	}

	if path, err := exec.LookPath(target); err == nil {
		program.Path = path
	}
	return program
}

// Secret is a credential that must never appear in a command line. It is written to a file of its
// own just before the program starts and removed the moment it ends, and the program is told where
// it is — by an argument for two of the three, and by the environment for pg_dump.
type Secret struct {
	// Body is the whole of the file, in whatever the program reads: an ini section, a YAML
	// document, a pgpass line.
	Body string
	// Extension is what the temporary file is called, which matters only in that mysqldump will
	// not read a defaults file it thinks is something else.
	Extension string
	// Arg names the file to the program, with one `%s` for the path. Empty means EnvVar does.
	Arg string
	// EnvVar names the file through the child's environment instead.
	EnvVar string
	// First says the argument has to come before all the others, which is what mysqldump insists
	// on for `--defaults-file` and the one thing about it that is not obvious.
	First bool
}

// Command is one run of one program: everything the child process is, built by command.go.
type Command struct {
	Program Program
	Args    []string
	// Env is added to this process's own environment rather than replacing it, so a tool that
	// needs a library path or a locale keeps finding it.
	Env []string
	// StdinPath is a file the child reads its input from, which is how the mysql client runs a
	// script: `mysql < dump.sql` with no shell in the middle.
	StdinPath string
	Secret    Secret
}

// Run starts the program and hands every line it prints to `line`, in order, as it prints it.
//
// It answers with what happened rather than with what was printed: a program that ends badly is
// `domain.ToolFailedError` carrying its own last words, and a run the reader stopped is
// `domain.ErrQueryCancelled` — the same answer a cancelled query gives, because it is the same
// gesture. The context is what kills the process, so a Cancel reaches a child that is halfway
// through a hundred-gigabyte archive.
func Run(ctx context.Context, command Command, line func(string)) error {
	if !command.Program.Found() {
		return domain.ToolNotFoundError{Tool: command.Program.Name}
	}

	cleanup, err := withSecret(&command)
	if err != nil {
		return err
	}
	defer cleanup()

	child := exec.CommandContext(ctx, command.Program.Path, command.Args...)
	child.Env = append(os.Environ(), command.Env...)
	hideWindow(child)

	if command.StdinPath != "" {
		input, err := os.Open(command.StdinPath)
		if err != nil {
			return domain.FileError{Err: err}
		}
		defer input.Close()
		child.Stdin = input
	}

	// One pipe for both streams: these programs print their progress on stderr and their
	// complaints on stderr too, and interleaving them in the order they were written is what makes
	// the log read like the run. os/exec serialises the writes when the two are the same writer.
	reader, writer := io.Pipe()
	child.Stdout = writer
	child.Stderr = writer

	if err := child.Start(); err != nil {
		writer.Close()
		// A run stopped before the child was even off the ground is still a stopped run, and
		// "context canceled" is not a sentence about a dump.
		if ctx.Err() != nil {
			return domain.ErrQueryCancelled
		}
		return domain.ToolFailedError{Tool: command.Program.Name, Message: err.Error()}
	}

	read := make(chan string, 1)
	go func() { read <- follow(reader, line) }()

	waitErr := child.Wait()
	writer.Close()
	last := <-read

	// A cancelled run is not a program that refused: the process was killed on purpose, and the
	// exit status it left behind says nothing the reader wants to read.
	if ctx.Err() != nil {
		return domain.ErrQueryCancelled
	}
	if waitErr != nil {
		return domain.ToolFailedError{
			Tool:    command.Program.Name,
			Status:  child.ProcessState.ExitCode(),
			Message: last,
		}
	}
	return nil
}

// follow carries the program's lines out as they arrive and answers with the last one, which is
// what a failure is reported as: these three programs all end by saying why.
func follow(reader io.Reader, line func(string)) string {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 4<<10), maxLineBytes)

	last := ""
	count := 0
	for scanner.Scan() {
		text := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(text) == "" {
			continue
		}
		last = text

		count++
		switch {
		case count < maxLines:
			line(text)
		case count == maxLines:
			line("… the rest of this program's output is not shown. The file itself is unaffected.")
		}
	}

	// The rest is still drained even where a line was too long to scan, or the child would block
	// writing into a pipe nobody is reading.
	if scanner.Err() != nil {
		_, _ = io.Copy(io.Discard, reader)
	}
	return last
}

// withSecret writes the credential file, puts what names it into the command, and hands back what
// removes it. The file is this process's alone — 0600, in the machine's temporary directory — and
// it is gone before Run returns however the run ended.
func withSecret(command *Command) (func(), error) {
	nothing := func() {}

	secret := command.Secret
	if secret.Body == "" {
		return nothing, nil
	}

	file, err := os.CreateTemp("", "vibe-db-*"+secret.Extension)
	if err != nil {
		return nothing, domain.FileError{Err: err}
	}
	path := file.Name()
	remove := func() { os.Remove(path) }

	if err := writeSecret(file, secret.Body); err != nil {
		remove()
		return nothing, err
	}

	if secret.EnvVar != "" {
		command.Env = append(command.Env, secret.EnvVar+"="+path)
		return remove, nil
	}

	named := strings.Replace(secret.Arg, "%s", path, 1)
	if secret.First {
		command.Args = append([]string{named}, command.Args...)
	} else {
		command.Args = append(command.Args, named)
	}
	return remove, nil
}

func writeSecret(file *os.File, body string) error {
	defer file.Close()

	if err := file.Chmod(secretMode); err != nil {
		return domain.FileError{Err: err}
	}
	if _, err := file.WriteString(body); err != nil {
		return domain.FileError{Err: err}
	}
	// The close is where a full volume is finally heard from, so it is answered for as well as
	// deferred — the order every other file this app writes is closed in.
	if err := file.Close(); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}
