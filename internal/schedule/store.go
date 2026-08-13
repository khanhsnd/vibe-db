package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	fileName = "schedules.json"
	version  = 1
)

// document is the file itself. One document read whole and written whole, exactly as the profiles
// and the snippet library are, so a reader can open it, read it and copy it to another machine.
type document struct {
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

// Store is every task this app knows about. It is the only thing that writes the file, and every
// change goes through one `Update`, so a run being recorded cannot write away a task the reader was
// editing at the same moment.
type Store struct {
	file *jsonstore.File[document]
}

func New(path string) *Store {
	return &Store{file: jsonstore.New[document](path)}
}

func DefaultPath() (string, error) {
	return jsonstore.Path(fileName)
}

func (s *Store) List() ([]Task, error) {
	doc, err := s.file.Read()
	if err != nil {
		return nil, err
	}
	return doc.Tasks, nil
}

func (s *Store) Get(id string) (Task, error) {
	tasks, err := s.List()
	if err != nil {
		return Task{}, err
	}

	for _, task := range tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return Task{}, domain.ErrTaskNotFound
}

// Save writes one task, minting an id for one that has none, and answers with the task as it now
// stands. It **arms** what it wrote: a task that was just edited counts its next firing from now, so
// changing a daily job's time never fires it on the way out of the dialog.
//
// What it deliberately does not touch is `LastRun` and `Runs`: those are what happened, and an edit
// to when a job runs next is not an edit to what it has already done.
func (s *Store) Save(task Task, now time.Time) (Task, error) {
	if err := task.Validate(); err != nil {
		return Task{}, err
	}

	if task.ID == "" {
		id, err := newID()
		if err != nil {
			return Task{}, err
		}
		task.ID = id
	}
	task.ArmedAt = now

	var saved Task
	err := s.file.Update(func(doc *document) error {
		doc.Version = version

		at := indexOf(doc.Tasks, task.ID)
		if at == -1 {
			doc.Tasks = append(doc.Tasks, task)
			saved = task
			return nil
		}

		task.LastRun = doc.Tasks[at].LastRun
		task.Runs = doc.Tasks[at].Runs
		doc.Tasks[at] = task
		saved = task
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return saved, nil
}

// Delete forgets one task. Nothing it ever wrote is touched: the backups it took are still in the
// backup folder and the files it exported are still where they landed, which is why this asks the
// reader nothing that the dialog offering it does not already.
func (s *Store) Delete(id string) error {
	return s.change(id, func(task *Task) error {
		task.ID = ""
		return nil
	})
}

// Enable turns one task on or off, arming it as it goes: a job switched back on waits a whole
// cadence rather than firing for every moment it slept through.
func (s *Store) Enable(id string, enabled bool, now time.Time) (Task, error) {
	var saved Task
	err := s.change(id, func(task *Task) error {
		task.Enabled = enabled
		task.ArmedAt = now
		saved = *task
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return saved, nil
}

// Record writes down what one firing came to, newest first, and moves the task's clock on. Both
// halves are one write because they are one fact: a run that was kept but did not move the clock
// would fire again on the next tick.
func (s *Store) Record(id string, run Run) error {
	return s.change(id, func(task *Task) error {
		task.LastRun = run.At
		task.Runs = append([]Run{run}, task.Runs...)
		if len(task.Runs) > RunsKept {
			task.Runs = task.Runs[:RunsKept]
		}
		return nil
	})
}

// Due is every task whose moment has come, in the order the file holds them.
func (s *Store) Due(now time.Time) ([]Task, error) {
	tasks, err := s.List()
	if err != nil {
		return nil, err
	}

	due := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Due(now) {
			due = append(due, task)
		}
	}
	return due, nil
}

// Arm moves past every firing that went by while the app was shut, for the tasks that said not to
// catch up. It is called once, when the scheduler starts.
//
// A task that **does** catch up is left exactly as it is, which is what makes it fire: its moment is
// in the past, so the first tick finds it due. This is the whole of what `CatchUp` means, and it is
// one function rather than a rule inside the tick so that a laptop opened at nine in the morning
// cannot fire a job twice for having been opened twice.
func (s *Store) Arm(now time.Time) error {
	tasks, err := s.List()
	if err != nil {
		return err
	}

	// Read before writing, so an app opened with nothing scheduled — which is most of them — does
	// not write a file to say so.
	if !slices.ContainsFunc(tasks, func(task Task) bool { return missed(task, now) }) {
		return nil
	}

	return s.file.Update(func(doc *document) error {
		doc.Version = version
		for at, task := range doc.Tasks {
			if missed(task, now) {
				doc.Tasks[at].ArmedAt = now
			}
		}
		return nil
	})
}

// missed is a task whose moment went by while the app was shut and which said not to catch up.
func missed(task Task, now time.Time) bool {
	return task.Enabled && !task.CatchUp && task.Due(now)
}

// change reads the document, hands one task to `edit`, and writes it back — under the one lock every
// other write here takes. An edit that blanks the id is how Delete says "take this one out", which
// keeps the read, the guard and the write in one place rather than in two nearly identical ones.
func (s *Store) change(id string, edit func(*Task) error) error {
	return s.file.Update(func(doc *document) error {
		at := indexOf(doc.Tasks, id)
		if at == -1 {
			return domain.ErrTaskNotFound
		}

		if err := edit(&doc.Tasks[at]); err != nil {
			return err
		}

		doc.Version = version
		if doc.Tasks[at].ID == "" {
			doc.Tasks = append(doc.Tasks[:at], doc.Tasks[at+1:]...)
		}
		return nil
	})
}

func indexOf(tasks []Task, id string) int {
	for at, task := range tasks {
		if task.ID == id {
			return at
		}
	}
	return -1
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
