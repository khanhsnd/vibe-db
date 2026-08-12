package app

import (
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// An action that arrived off the wire as something this app does not do is refused here, before it
// reaches anything that spells a statement with it — the arrangement an account change's kind has.
func TestProcessKillRefusesAnActionThisAppDoesNotDo(t *testing.T) {
	in := ProcessKillInput{Process: ProcessRef{ID: "42"}, Action: "terminate"}

	_, err := in.toDriver()

	var invalid domain.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("toDriver() error = %v, want a validation error", err)
	}
	if invalid.Field != "action" {
		t.Errorf("field = %q, want action", invalid.Field)
	}
}

func TestProcessKillReadsTheTwoActions(t *testing.T) {
	for word, want := range map[string]domain.ProcessAction{
		"cancel": domain.ProcessCancel,
		"close":  domain.ProcessClose,
	} {
		kill, err := ProcessKillInput{
			Process: ProcessRef{ID: "42", Serial: "3421"},
			Action:  word,
		}.toDriver()
		if err != nil {
			t.Fatalf("%q: %v", word, err)
		}
		if kill.Action != want {
			t.Errorf("%q became %q, want %q", word, kill.Action, want)
		}
		if kill.Process.ID != "42" || kill.Process.Serial != "3421" {
			t.Errorf("%q carried %+v", word, kill.Process)
		}
	}
}

// The hazard of a batch is the **worst** of it: a reader pressing the button ends all of them, so one
// row that is this app's own connection is answered for even where the rows around it are ordinary.
func TestProcessHazardIsTheWorstOfTheBatch(t *testing.T) {
	kills := []driver.ProcessKill{
		{Process: driver.ProcessRef{ID: "1"}, Action: domain.ProcessCancel},
		{Process: driver.ProcessRef{ID: "2", Own: true}, Action: domain.ProcessClose},
		{Process: driver.ProcessRef{ID: "3"}, Action: domain.ProcessClose},
	}

	if got := processHazard(kills); got != domain.HazardUnfiltered {
		t.Errorf("processHazard = %q, want %q", got, domain.HazardUnfiltered)
	}
}

func TestProcessHazardOfCancellingIsAWrite(t *testing.T) {
	kills := []driver.ProcessKill{
		{Process: driver.ProcessRef{ID: "1"}, Action: domain.ProcessCancel},
	}

	if got := processHazard(kills); got != domain.HazardWrite {
		t.Errorf("processHazard = %q, want %q", got, domain.HazardWrite)
	}
}

// An empty list is an empty list on the wire rather than a null, which is what a grid draws as
// "nothing here" instead of falling over.
func TestProcessListRowsAreNeverNull(t *testing.T) {
	list := toProcessListDTO(driver.ProcessList{})

	if list.Rows == nil {
		t.Error("rows are null")
	}
	if toServerFactsDTO(driver.ServerFacts{}).Rows == nil {
		t.Error("facts rows are null")
	}
}
