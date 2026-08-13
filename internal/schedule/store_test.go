package schedule

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "schedules.json"))
}

func nightly() Task {
	return Task{
		Name:      "Nightly backup",
		Kind:      KindBackup,
		Enabled:   true,
		ProfileID: "p1",
		Database:  "shop",
		Cadence:   Cadence{Every: EveryDaily, Hour: 2},
		Options:   Options{WithStructure: true, WithData: true},
	}
}

func TestStoreSaveMintsAnIDAndArms(t *testing.T) {
	store := testStore(t)
	now := at("2026-08-13 10:00:00")

	saved, err := store.Save(nightly(), now)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("no id was minted")
	}
	if !saved.ArmedAt.Equal(now) {
		t.Fatalf("armed at %s, want %s", saved.ArmedAt, now)
	}

	// Armed at ten in the morning, a daily two o'clock job waits for tomorrow.
	if got, want := saved.Next(), at("2026-08-14 02:00:00"); !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

// An edit is an edit to when a job runs next, and never to what it has already done.
func TestStoreSaveKeepsWhatHappened(t *testing.T) {
	store := testStore(t)

	saved, err := store.Save(nightly(), at("2026-08-13 10:00:00"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ran := Run{At: at("2026-08-14 02:00:00"), Outcome: OutcomeOK, Summary: "20260814020000"}
	if err := store.Record(saved.ID, ran); err != nil {
		t.Fatalf("record: %v", err)
	}

	saved.Name = "Nightly backup of shop"
	edited, err := store.Save(saved, at("2026-08-14 09:00:00"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if len(edited.Runs) != 1 {
		t.Fatalf("kept %d runs, want 1", len(edited.Runs))
	}
	if !edited.LastRun.Equal(ran.At) {
		t.Fatalf("last run = %s, want %s", edited.LastRun, ran.At)
	}
}

func TestStoreRecordKeepsTheNewestFew(t *testing.T) {
	store := testStore(t)

	saved, err := store.Save(nightly(), at("2026-08-13 10:00:00"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	for day := 1; day <= RunsKept+5; day++ {
		moment := at("2026-08-13 02:00:00").AddDate(0, 0, day)
		if err := store.Record(saved.ID, Run{At: moment, Outcome: OutcomeOK}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	read, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(read.Runs) != RunsKept {
		t.Fatalf("kept %d runs, want %d", len(read.Runs), RunsKept)
	}
	// Newest first, so the last firing is the one at the top.
	if want := at("2026-08-13 02:00:00").AddDate(0, 0, RunsKept+5); !read.Runs[0].At.Equal(want) {
		t.Fatalf("newest run = %s, want %s", read.Runs[0].At, want)
	}
}

func TestStoreDue(t *testing.T) {
	store := testStore(t)

	if _, err := store.Save(nightly(), at("2026-08-13 10:00:00")); err != nil {
		t.Fatalf("save: %v", err)
	}

	due, err := store.Due(at("2026-08-13 23:00:00"))
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("%d tasks due before their moment", len(due))
	}

	due, err = store.Due(at("2026-08-14 02:00:01"))
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("%d tasks due, want 1", len(due))
	}
}

// A moment that went by while the app was shut fires for a task that catches up, and is stepped over
// for one that does not.
func TestStoreArm(t *testing.T) {
	store := testStore(t)

	missed := nightly()
	missed.Name = "Missed"
	if _, err := store.Save(missed, at("2026-08-13 10:00:00")); err != nil {
		t.Fatalf("save: %v", err)
	}

	catching := nightly()
	catching.Name = "Catching up"
	catching.CatchUp = true
	if _, err := store.Save(catching, at("2026-08-13 10:00:00")); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The app opens two days later, having missed both firings.
	opened := at("2026-08-15 09:00:00")
	if err := store.Arm(opened); err != nil {
		t.Fatalf("arm: %v", err)
	}

	due, err := store.Due(opened)
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("%d tasks due, want 1", len(due))
	}
	if due[0].Name != "Catching up" {
		t.Fatalf("due task is %q, want the one that catches up", due[0].Name)
	}
}

func TestStoreEnableArms(t *testing.T) {
	store := testStore(t)

	off := nightly()
	off.Enabled = false
	saved, err := store.Save(off, at("2026-08-10 10:00:00"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// Turned on days later, it waits for the next two o'clock rather than firing for each it slept
	// through.
	on, err := store.Enable(saved.ID, true, at("2026-08-13 10:00:00"))
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if got, want := on.Next(), at("2026-08-14 02:00:00"); !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestStoreDelete(t *testing.T) {
	store := testStore(t)

	saved, err := store.Save(nightly(), time.Now())
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Delete(saved.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := store.Get(saved.ID); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Fatalf("get after delete = %v, want ErrTaskNotFound", err)
	}
	if err := store.Delete(saved.ID); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Fatalf("second delete = %v, want ErrTaskNotFound", err)
	}
}

func TestStoreRefusesAnInvalidTask(t *testing.T) {
	store := testStore(t)

	broken := nightly()
	broken.Name = ""
	if _, err := store.Save(broken, time.Now()); err == nil {
		t.Fatal("a task with no name was saved")
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("%d tasks written, want none", len(tasks))
	}
}
