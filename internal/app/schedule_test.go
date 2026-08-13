package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/schedule"
)

// tasksAPI is the smallest ConnectionAPI a scheduled job's *wire* half needs: the store it reads and
// the clock it asks whether a job is going. Nothing here dials anything.
func tasksAPI(t *testing.T) *ConnectionAPI {
	t.Helper()
	return NewConnectionAPI(nil, nil, nil, nil, nil,
		schedule.New(filepath.Join(t.TempDir(), "schedules.json")))
}

// Every answer the dialog gives has to survive the trip out to the file and back, or a reader edits
// a job and silently loses the half of it this side forgot to carry.
func TestATaskSurvivesTheRoundTrip(t *testing.T) {
	api := tasksAPI(t)

	input := TaskInput{
		Namespace:     Namespace{Database: "shop", Schema: "public"},
		Name:          "Nightly compare",
		Kind:          string(schedule.KindCompare),
		Enabled:       true,
		CatchUp:       true,
		ProfileID:     "p1",
		Every:         string(schedule.EveryWeekly),
		Minutes:       15,
		Hour:          2,
		Minute:        30,
		Weekday:       3,
		Tables:        []string{"users", "orders"},
		WithStructure: true,
		WithData:      true,
		Comment:       "before the release",
		Keep:          7,
		Format:        "csv",
		Folder:        `C:\out`,
		MaxRows:       500,
		Compress:      true,
		Table:         "users",
		Right: TaskTarget{
			ProfileID: "p2",
			Database:  "shop",
			Schema:    "public",
			Table:     "users",
		},
	}

	saved, err := api.SaveTask(input)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	listed, err := api.ListTasks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("%d tasks listed, want 1", len(listed))
	}

	read := listed[0]
	if read.ID != saved.ID {
		t.Fatalf("id = %q, want %q", read.ID, saved.ID)
	}

	// Compared as the input it was, so a field added later and forgotten in one of the two
	// directions fails here rather than in front of a reader.
	input.ID = saved.ID
	if read.TaskInput.ID != input.ID ||
		read.Name != input.Name ||
		read.Kind != input.Kind ||
		read.Enabled != input.Enabled ||
		read.CatchUp != input.CatchUp ||
		read.ProfileID != input.ProfileID ||
		read.Database != input.Database ||
		read.Schema != input.Schema ||
		read.Every != input.Every ||
		read.Minutes != input.Minutes ||
		read.Hour != input.Hour ||
		read.Minute != input.Minute ||
		read.Weekday != input.Weekday ||
		strings.Join(read.Tables, ",") != strings.Join(input.Tables, ",") ||
		read.WithStructure != input.WithStructure ||
		read.WithData != input.WithData ||
		read.Comment != input.Comment ||
		read.Keep != input.Keep ||
		read.Format != input.Format ||
		read.Folder != input.Folder ||
		read.MaxRows != input.MaxRows ||
		read.Compress != input.Compress ||
		read.Table != input.Table ||
		read.Right != input.Right {
		t.Fatalf("the task came back changed:\n got %+v\nwant %+v", read.TaskInput, input)
	}

	if read.NextRunAt == "" {
		t.Fatal("an enabled task came back with no next firing")
	}
	if read.LastRunAt != "" {
		t.Fatalf("a task that has never fired says it last ran at %q", read.LastRunAt)
	}
	if read.Running {
		t.Fatal("a task nothing has fired says it is running")
	}
}

// A job that is off has no next firing, and saying so is honester than a date it will not keep.
func TestADisabledTaskNamesNoNextFiring(t *testing.T) {
	api := tasksAPI(t)

	saved, err := api.SaveTask(TaskInput{
		Namespace:     Namespace{Database: "shop"},
		Name:          "Nightly",
		Kind:          string(schedule.KindBackup),
		ProfileID:     "p1",
		Every:         string(schedule.EveryDaily),
		Hour:          2,
		WithStructure: true,
		WithData:      true,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.NextRunAt != "" {
		t.Fatalf("next firing = %q, want none", saved.NextRunAt)
	}
}

func TestSaveTaskRefusesAnIncompleteJob(t *testing.T) {
	api := tasksAPI(t)

	if _, err := api.SaveTask(TaskInput{Name: "Nowhere", Kind: "backup"}); err == nil {
		t.Fatal("a task naming no connection was saved")
	}
}

// A dump is one file per firing, so two firings never write over each other — and a namespace whose
// name a file system will not take is still written somewhere.
func TestDumpFileNameIsPerFiringAndFileSafe(t *testing.T) {
	task := schedule.Task{Database: `shop:live`, Schema: "public"}

	name := dumpFileName(task, "sql")
	if strings.ContainsAny(name, `<>:"/\|?*`) {
		t.Fatalf("name holds a character a file name may not: %q", name)
	}
	if !strings.HasSuffix(name, ".sql") {
		t.Fatalf("name = %q, want the kit's own extension", name)
	}
	if !strings.HasPrefix(name, "shop_live-public-") {
		t.Fatalf("name = %q, want it to open with the namespace", name)
	}
}

// A job that failed half way through says most in what it said last.
func TestLastFailureIsTheLastOne(t *testing.T) {
	messages := []TransferMessage{
		{Text: "Writing to C:\\out"},
		{Text: "users: connection reset", Error: true},
		{Text: "orders: table does not exist", Error: true},
	}

	if got := lastFailure(messages); got != "orders: table does not exist" {
		t.Fatalf("last failure = %q", got)
	}
	if got := lastFailure(nil); got == "" {
		t.Fatal("a log with nothing in it answered with nothing")
	}
}
