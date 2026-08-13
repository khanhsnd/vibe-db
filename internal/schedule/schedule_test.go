package schedule

import (
	"testing"
	"time"
)

// at is a moment in the machine's own zone, which is the zone every cadence is read in.
func at(text string) time.Time {
	moment, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.Local)
	if err != nil {
		panic(err)
	}
	return moment
}

func TestCadenceNext(t *testing.T) {
	cases := []struct {
		name    string
		cadence Cadence
		after   string
		want    string
	}{
		{
			name:    "minutes counts from the moment given",
			cadence: Cadence{Every: EveryMinutes, Minutes: 15},
			after:   "2026-08-13 10:07:00",
			want:    "2026-08-13 10:22:00",
		},
		{
			name:    "hourly finds the next minute past the hour",
			cadence: Cadence{Every: EveryHourly, Minute: 30},
			after:   "2026-08-13 10:07:00",
			want:    "2026-08-13 10:30:00",
		},
		{
			name:    "hourly rolls over where the minute has gone",
			cadence: Cadence{Every: EveryHourly, Minute: 30},
			after:   "2026-08-13 10:45:00",
			want:    "2026-08-13 11:30:00",
		},
		{
			name:    "daily waits for tomorrow where today's moment has gone",
			cadence: Cadence{Every: EveryDaily, Hour: 2},
			after:   "2026-08-13 10:00:00",
			want:    "2026-08-14 02:00:00",
		},
		{
			name:    "daily takes today where the moment is still to come",
			cadence: Cadence{Every: EveryDaily, Hour: 23, Minute: 30},
			after:   "2026-08-13 10:00:00",
			want:    "2026-08-13 23:30:00",
		},
		{
			// 13 August 2026 is a Thursday.
			name:    "weekly finds the next such day",
			cadence: Cadence{Every: EveryWeekly, Weekday: int(time.Sunday), Hour: 3},
			after:   "2026-08-13 10:00:00",
			want:    "2026-08-16 03:00:00",
		},
		{
			name:    "weekly on today's day waits a week where the moment has gone",
			cadence: Cadence{Every: EveryWeekly, Weekday: int(time.Thursday), Hour: 3},
			after:   "2026-08-13 10:00:00",
			want:    "2026-08-20 03:00:00",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := one.cadence.Next(at(one.after))
			if !got.Equal(at(one.want)) {
				t.Fatalf("next = %s, want %s", got, at(one.want))
			}
		})
	}
}

// A firing recorded at exactly the moment it was due must not be due again on the next tick, which
// is the whole reason Next is strict.
func TestCadenceNextIsStrictlyAfter(t *testing.T) {
	cadence := Cadence{Every: EveryDaily, Hour: 2}
	moment := at("2026-08-13 02:00:00")

	if got := cadence.Next(moment); !got.After(moment) {
		t.Fatalf("next = %s, want a moment after %s", got, moment)
	}
}

func TestTaskNextCountsFromTheLaterOfArmingAndLastRun(t *testing.T) {
	task := Task{
		Enabled: true,
		Cadence: Cadence{Every: EveryDaily, Hour: 2},
		ArmedAt: at("2026-08-10 09:00:00"),
		LastRun: at("2026-08-13 02:00:00"),
	}

	if got, want := task.Next(), at("2026-08-14 02:00:00"); !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

// A task saved at ten in the morning must not fire at ten in the morning: arming is what a cadence
// counts from until the job has run once.
func TestTaskNextCountsFromArmingBeforeAnyRun(t *testing.T) {
	task := Task{
		Enabled: true,
		Cadence: Cadence{Every: EveryDaily, Hour: 2},
		ArmedAt: at("2026-08-13 10:00:00"),
	}

	if got, want := task.Next(), at("2026-08-14 02:00:00"); !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestTaskDue(t *testing.T) {
	task := Task{
		Enabled: true,
		Cadence: Cadence{Every: EveryMinutes, Minutes: 10},
		ArmedAt: at("2026-08-13 10:00:00"),
	}

	if task.Due(at("2026-08-13 10:09:00")) {
		t.Fatal("due before its moment")
	}
	if !task.Due(at("2026-08-13 10:10:00")) {
		t.Fatal("not due at its moment")
	}

	task.Enabled = false
	if task.Due(at("2026-08-13 11:00:00")) {
		t.Fatal("a disabled task is due")
	}
}

func TestTaskValidate(t *testing.T) {
	whole := func() Task {
		return Task{
			Name:      "Nightly",
			Kind:      KindBackup,
			ProfileID: "p1",
			Database:  "shop",
			Cadence:   Cadence{Every: EveryDaily, Hour: 2},
			Options:   Options{WithStructure: true, WithData: true},
		}
	}

	if err := whole().Validate(); err != nil {
		t.Fatalf("a whole task was refused: %v", err)
	}

	cases := map[string]func(*Task){
		"no name":            func(task *Task) { task.Name = "" },
		"no kind":            func(task *Task) { task.Kind = "" },
		"no connection":      func(task *Task) { task.ProfileID = "" },
		"no database":        func(task *Task) { task.Database = "" },
		"no cadence":         func(task *Task) { task.Cadence.Every = "" },
		"no minutes":         func(task *Task) { task.Cadence = Cadence{Every: EveryMinutes} },
		"an impossible hour": func(task *Task) { task.Cadence.Hour = 24 },
		"neither half":       func(task *Task) { task.Options = Options{} },
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			task := whole()
			break_(&task)
			if err := task.Validate(); err == nil {
				t.Fatal("was accepted")
			}
		})
	}
}

// What one kind reads is not asked of another: an export needs a folder and a backup does not, so a
// task switched between them keeps the answers it was given for both.
func TestValidateOnlyAsksWhatTheKindReads(t *testing.T) {
	task := Task{
		Name:      "Nightly",
		Kind:      KindBackup,
		ProfileID: "p1",
		Database:  "shop",
		Cadence:   Cadence{Every: EveryDaily, Hour: 2},
		Options:   Options{WithStructure: true, Folder: ""},
	}

	if err := task.Validate(); err != nil {
		t.Fatalf("a backup was asked for a folder: %v", err)
	}

	task.Kind = KindExport
	task.Options.Format = "csv"
	if err := task.Validate(); err == nil {
		t.Fatal("an export with no folder was accepted")
	}
}
