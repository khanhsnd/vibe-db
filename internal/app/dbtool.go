package app

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/dbtool"
	"github.com/khanhsnd/vibe-db/internal/domain"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// toolTimeout is what a whole dump or a whole restore is given, and it is a backup's two hours for
// the same reason: a hundred gigabytes is a long job that is going fine, and a clock that cut it
// off would be the only thing wrong with it. Stop is the honest bound, and it kills the child.
const toolTimeout = 2 * time.Hour

// ToolProgram is one of an engine's two programs, as this machine holds it. `Chosen` set with
// `Found` false is the one state worth drawing differently: the file the reader picked has gone,
// which is a different sentence from never having picked one.
type ToolProgram struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Chosen string `json:"chosen"`
	Found  bool   `json:"found"`
}

// Toolkit is what the dialog draws itself from: what the two programs are called, whether they are
// here, and which of the four options this pair actually has. Every one of those is a fact about
// the engine or about the machine — never a comparison against an engine's name in a component.
//
// **Both programs, always**, whichever direction the dialog is open in: a reader looking at where
// mongodump is wants to see mongorestore beside it, and either can be pointed somewhere else at any
// time — see ChooseToolProgram.
type Toolkit struct {
	Engine    string      `json:"engine"`
	Dump      ToolProgram `json:"dump"`
	Restore   ToolProgram `json:"restore"`
	Extension string      `json:"extension"`
	Format    string      `json:"format"`

	SplitsStructure bool `json:"splitsStructure"`
	Drops           bool `json:"drops"`
	DryRuns         bool `json:"dryRuns"`
	Compresses      bool `json:"compresses"`
}

// ToolDumpInput is one run of the engine's own dump program.
type ToolDumpInput struct {
	Namespace
	// RunID is minted by the caller so a dump already going can be stopped — StopTool takes it,
	// and stopping it kills the child process.
	RunID string `json:"runId"`
	// Path is the file to write, picked by the reader through ChooseToolFile.
	Path string `json:"path"`
	// Tables is what the dump covers, and **empty is every table the namespace holds**.
	Tables        []string `json:"tables"`
	WithStructure bool     `json:"withStructure"`
	WithData      bool     `json:"withData"`
	Compress      bool     `json:"compress"`
}

// ToolRestoreInput is one run of the engine's own restore program.
type ToolRestoreInput struct {
	Namespace
	RunID string `json:"runId"`
	Path  string `json:"path"`
	// Tables narrows what comes back out of the file, and empty is everything in it.
	Tables      []string `json:"tables"`
	Drop        bool     `json:"drop"`
	DryRun      bool     `json:"dryRun"`
	StopOnError bool     `json:"stopOnError"`
}

// ToolResult is what the run did, in the lines the program itself printed. It carries
// `TransferMessage` for the same reason a backup does: a reader watching a job go past should not
// have to learn a second shape of line for the same thing happening.
type ToolResult struct {
	Program   string            `json:"program"`
	Path      string            `json:"path"`
	Failed    bool              `json:"failed"`
	ElapsedMs int64             `json:"elapsedMs"`
	Messages  []TransferMessage `json:"messages"`
}

// LoadToolkit answers what this connection's engine ships, and whether this machine has it.
//
// It is looked for **every time** rather than remembered: a reader who installs the tools while the
// dialog is open should be able to press the button rather than restart the app.
func (a *ConnectionAPI) LoadToolkit(sessionID string) (Toolkit, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}

	kit, err := a.tools.Kit(held.Profile.Engine)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}
	return toolkitDTO(held.Profile.Engine, kit), nil
}

// ChooseToolProgram asks for **the program file itself** — `which` is `dump` or `restore` — and
// answers with the kit as it stands afterwards, so the dialog redraws from one call rather than two.
//
// The file and not the folder it is in: a reader who has built one from source, or who keeps two
// versions of the MongoDB tools, is pointing at *this* mongodump. Picking one still finds the other
// where it is sitting beside it, which is `Locator.Choose`'s doing and never overrides a file the
// reader picked themselves.
//
// It is offered whether or not the program is already here, because a path that was right last
// month is exactly the thing that needs changing today. A dismissed picker changes nothing and is
// not a failure — the same reading every other picker in this app has.
func (a *ConnectionAPI) ChooseToolProgram(sessionID, which string) (Toolkit, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}

	kit, err := a.tools.Kit(held.Profile.Engine)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}

	program := kit.Dump
	if which == dbtool.RestoreProgram {
		program = kit.Restore
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Where " + program.Name + " is",
		DefaultDirectory: filepath.Dir(program.Path),
		Filters:          programFilters(program.Name),
	})
	if err != nil {
		return Toolkit{}, wrapError(err)
	}
	if path == "" {
		return toolkitDTO(held.Profile.Engine, kit), nil
	}

	chosen, err := a.tools.Choose(held.Profile.Engine, which, path)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}
	return toolkitDTO(held.Profile.Engine, chosen), nil
}

// ForgetToolProgram drops the file the reader pointed at, so this program goes back to whatever is
// on the PATH — which is the way back from having pointed at the wrong copy.
func (a *ConnectionAPI) ForgetToolProgram(sessionID, which string) (Toolkit, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return Toolkit{}, wrapError(err)
	}

	kit, err := a.tools.Choose(held.Profile.Engine, which, "")
	if err != nil {
		return Toolkit{}, wrapError(err)
	}
	return toolkitDTO(held.Profile.Engine, kit), nil
}

// programFilters offers the program by name first, so the picker opens on the one file in a `bin`
// directory that is the right one. `All files` sits behind it because a program built from source,
// or renamed to carry its version, is still the program the reader means.
func programFilters(name string) []wailsruntime.FileFilter {
	pattern := name
	if runtime.GOOS == "windows" {
		pattern = name + ".exe"
	}

	return []wailsruntime.FileFilter{
		{DisplayName: name, Pattern: pattern},
		{DisplayName: "All files", Pattern: "*.*"},
	}
}

// ChooseToolFile raises the host's own picker for the file a dump writes or a restore reads, and
// answers with the path. An empty path is a dismissed dialog rather than a failure.
//
// The filter is the **engine's** — a MongoDB archive is not a script and a PostgreSQL custom-format
// file is not SQL, so neither is offered under a name that would let it be run as one.
func (a *ConnectionAPI) ChooseToolFile(sessionID string, saving bool, suggested string) (string, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	kit, err := a.tools.Kit(held.Profile.Engine)
	if err != nil {
		return "", wrapError(err)
	}

	filters := []wailsruntime.FileFilter{
		{DisplayName: kit.Format, Pattern: "*." + kit.Extension},
		{DisplayName: "All files", Pattern: "*.*"},
	}

	if saving {
		path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
			Title:           kit.Dump.Name,
			DefaultFilename: suggested + "." + kit.Extension,
			Filters:         filters,
		})
		return path, wrapError(err)
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   kit.Restore.Name,
		Filters: filters,
	})
	return path, wrapError(err)
}

// RunToolDump writes this namespace out with the engine's own dump program.
//
// **Nothing about the file is this app's.** What mongodump writes is BSON and what pg_dump writes
// is a custom-format archive, so nothing here reads either one — which is the whole point of
// running them: a script this app spelled could carry only what a catalogue read told it about,
// and a document store's catalogue is a sample.
//
// The bytes never pass through this process either: each program is told the path and writes it
// itself, so a dump of a hundred gigabytes costs this app one log's worth of memory.
func (a *ConnectionAPI) RunToolDump(sessionID string, input ToolDumpInput) (ToolResult, error) {
	request, kit, err := a.toolRequest(sessionID, input.Namespace, input.Path)
	if err != nil {
		return ToolResult{}, wrapError(err)
	}

	if !input.WithStructure && !input.WithData && kit.SplitsStructure {
		return ToolResult{}, wrapError(domain.ValidationError{
			Field:   "dump",
			Message: "must hold the structure, the records, or both",
		})
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, toolTimeout)
	defer release()

	// What the program is told to leave out is worked out here rather than in the driver, because
	// it takes the whole table list to know it — and mongodump narrows only by exclusion.
	excluded, err := a.excludedTables(ctx, sessionID, input.Namespace, input.Tables)
	if err != nil {
		return ToolResult{}, wrapError(err)
	}

	request.Include = input.Tables
	request.Exclude = excluded
	request.Structure = input.WithStructure || !kit.SplitsStructure
	request.Data = input.WithData || !kit.SplitsStructure
	request.Compress = input.Compress && kit.Compresses

	command, err := dbtool.Dump(kit, request)
	if err != nil {
		return ToolResult{}, wrapError(err)
	}
	return a.runTool(ctx, command, input.RunID, input.Path)
}

// RunToolRestore puts one of those files back with the engine's own restore program.
//
// A restore **is not undone by anything**, which is why the dialog says so before it starts: what
// the program has already written stays written, and `Drop` means the reader asked for what was
// there to be removed first.
func (a *ConnectionAPI) RunToolRestore(sessionID string, input ToolRestoreInput) (ToolResult, error) {
	request, kit, err := a.toolRequest(sessionID, input.Namespace, input.Path)
	if err != nil {
		return ToolResult{}, wrapError(err)
	}

	// The one write in this app that no session carries: the vendor's program opens its own
	// connection, so the wrapper guarding every other write never sees it and the refusal is made
	// here instead. The matching dump only reads and is left alone. See domain.Access.
	if err := a.refuseReadOnly(sessionID, "a restore"); err != nil {
		return ToolResult{}, wrapError(err)
	}

	request.Include = input.Tables
	request.Drop = input.Drop && kit.Drops
	request.DryRun = input.DryRun && kit.DryRuns
	request.StopOnError = input.StopOnError

	command, err := dbtool.Restore(kit, request)
	if err != nil {
		return ToolResult{}, wrapError(err)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, toolTimeout)
	defer release()

	return a.runTool(ctx, command, input.RunID, input.Path)
}

// StopTool stops the run with this id, which kills the program. An unknown id is ignored: the run
// may have finished between the click and this call, the same contract StopTableRows has.
func (a *ConnectionAPI) StopTool(runID string) {
	a.runs.cancel(runID)
}

// runTool is the whole of what both directions have in common: run it, carry its lines to the
// reader as they arrive, and say afterwards what happened.
//
// A failure comes back as a **result** rather than as a thrown error, because the lines are the
// answer: a rejected restore is a program that said why, and a promise that rejects would throw the
// log away. What could not even be started — a program that is not here, a path that is empty — is
// thrown instead, and is refused before this is reached.
func (a *ConnectionAPI) runTool(
	ctx context.Context,
	command dbtool.Command,
	runID, path string,
) (ToolResult, error) {
	started := time.Now()

	result := ToolResult{
		Program: command.Program.Name,
		Path:    path,
		// The command line is the first thing in the log because it is the whole truth about what
		// ran, and it can be shown at all only because no secret is ever an argument — see
		// `dbtool.Secret`.
		Messages: []TransferMessage{transferMessage(commandText(command))},
	}

	line := func(text string) {
		result.Messages = append(result.Messages, transferMessage(text))
		wailsruntime.EventsEmit(a.ctx, EventToolLine, ToolLine{RunID: runID, Text: text})
	}

	err := dbtool.Run(ctx, command, line)
	result.ElapsedMs = time.Since(started).Milliseconds()

	// A program that was never here printed nothing, so there is no log to keep and nothing for a
	// reader to read in one: it is thrown, and the dialog says which program and offers Locate.
	if errors.Is(err, domain.ErrToolNotFound) {
		return ToolResult{}, wrapError(err)
	}
	if err != nil {
		result.Failed = true
		result.Messages = append(result.Messages, transferFailure(command.Program.Name, err))
		return result, nil
	}

	result.Messages = append(result.Messages, transferMessage("Finished · "+path))
	return result, nil
}

// toolRequest is everything a run needs and every reason it cannot happen, in one place — so the
// two directions refuse the same things in the same words.
func (a *ConnectionAPI) toolRequest(
	sessionID string,
	ns Namespace,
	path string,
) (dbtool.Request, dbtool.Kit, error) {
	if strings.TrimSpace(path) == "" {
		return dbtool.Request{}, dbtool.Kit{}, domain.NameError{What: "file"}
	}

	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return dbtool.Request{}, dbtool.Kit{}, err
	}
	profile := held.Profile

	// A tunnel is an in-process dialer and listens on nothing, so there is no address a child
	// process could be given — see `internal/sshtunnel`. Saying so is the honest answer; running
	// the program against the far side's host name would reach whatever is at that name *here*.
	if profile.SSH.Enabled {
		return dbtool.Request{}, dbtool.Kit{},
			domain.Unsupported("running " + string(profile.Engine) + "'s own tools through an SSH tunnel")
	}
	// A minted token is signed for one connection at the moment it is made, and nothing here
	// makes a connection: the program would be handed a password that is already somebody else's.
	if profile.Auth.Minted() {
		return dbtool.Request{}, dbtool.Kit{},
			domain.Unsupported("running these tools with a cloud identity")
	}

	kit, err := a.tools.Kit(profile.Engine)
	if err != nil {
		return dbtool.Request{}, dbtool.Kit{}, err
	}

	password, err := a.storedSecret(profile, profile.ID)
	if err != nil {
		return dbtool.Request{}, dbtool.Kit{}, err
	}

	return dbtool.Request{
		Profile:  profile,
		Password: password,
		Database: ns.Database,
		Schema:   ns.Schema,
		Path:     path,
	}, kit, nil
}

// excludedTables is what the namespace holds that the reader did not tick, which is the only way
// mongodump can be told to narrow. An empty pick is the whole namespace, so nothing is excluded.
func (a *ConnectionAPI) excludedTables(
	ctx context.Context,
	sessionID string,
	ns Namespace,
	picked []string,
) ([]string, error) {
	if len(picked) == 0 {
		return nil, nil
	}

	all, err := a.namespaceTables(ctx, sessionID, ns)
	if err != nil {
		return nil, err
	}

	kept := make(map[string]bool, len(picked))
	for _, table := range picked {
		kept[table] = true
	}

	excluded := make([]string, 0, len(all))
	for _, table := range all {
		if !kept[table] {
			excluded = append(excluded, table)
		}
	}
	return excluded, nil
}

func toolkitDTO(engine domain.Engine, kit dbtool.Kit) Toolkit {
	return Toolkit{
		Engine:          string(engine),
		Dump:            programDTO(kit.Dump),
		Restore:         programDTO(kit.Restore),
		Extension:       kit.Extension,
		Format:          kit.Format,
		SplitsStructure: kit.SplitsStructure,
		Drops:           kit.Drops,
		DryRuns:         kit.DryRuns,
		Compresses:      kit.Compresses,
	}
}

func programDTO(program dbtool.Program) ToolProgram {
	return ToolProgram{
		Name:   program.Name,
		Path:   program.Path,
		Chosen: program.Chosen,
		Found:  program.Found(),
	}
}

// commandText is the command line as a person would type it, which is what the log opens with. A
// path with a space in it is quoted so the line can be pasted into a terminal — the one thing a
// reader does with it when a dump fails for a reason neither of us expected.
//
// It can be shown at all only because **no secret is ever an argument**. What is missing from it is
// exactly the credential file, which is written for the run and gone before the log is read.
func commandText(command dbtool.Command) string {
	parts := make([]string, 0, len(command.Args)+1)
	for _, part := range append([]string{command.Program.Path}, command.Args...) {
		if strings.ContainsAny(part, " \t\"") {
			part = `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}
