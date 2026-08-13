package app

import (
	"context"
	"sync"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/schedule"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// tickEvery is how often the clock is looked at. It is half a minute because the finest cadence a
// task has is a minute, and a job that fires "every 5 minutes" landing up to half a minute late is a
// job nobody is watching to the second — while a tick per second would be a file read per second for
// the entire life of the window.
const tickEvery = 30 * time.Second

// scheduler is the clock. It owns two things and no more: which tasks are firing right now, and the
// loop that asks the store what is due.
//
// **It runs only while the app is open.** This is a desktop client and not a service: nothing is
// installed on the machine, no daemon is left behind, and a laptop that was shut at two in the
// morning ran nothing at two in the morning — which is what `CatchUp` is the reader's answer to.
type scheduler struct {
	api *ConnectionAPI

	mu       sync.Mutex
	inFlight map[string]bool
	stop     chan struct{}
}

func newScheduler(api *ConnectionAPI) *scheduler {
	return &scheduler{api: api, inFlight: make(map[string]bool)}
}

// start arms the tasks that slept through their moment and then watches the clock. It is called once
// the window is up, because a job that fires needs somewhere to report to.
func (s *scheduler) start(ctx context.Context) {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.stop = stop
	s.mu.Unlock()

	// Arming is the one write here that happens before any tick: a task that does not catch up is
	// moved past the firing it missed, so opening the app at nine in the morning does not set last
	// night's backup going. See schedule.Store.Arm.
	_ = s.api.tasks.Arm(time.Now())

	go func() {
		ticker := time.NewTicker(tickEvery)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

func (s *scheduler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
}

// tick fires everything whose moment has come, each on its own goroutine.
//
// Its own and not one after another, because a job is given hours: a backup that runs from two in
// the morning until four would otherwise be two hours in which nothing else on this clock could
// fire, and the ticker itself would be the thing waiting. What is *not* allowed to overlap is one
// task with itself — see fire, which is also what keeps a long job from being started again by every
// tick while it is still going.
func (s *scheduler) tick() {
	due, err := s.api.tasks.Due(time.Now())
	if err != nil {
		return
	}

	for _, task := range due {
		go func() { _, _ = s.fire(task) }()
	}
}

// fire runs one task, writes down what it came to, and tells the window twice — once as it starts and
// once as it ends, so a list left open shows a job going rather than a row that changes when nobody
// was looking.
//
// A failure inside the job is **not** an error here: it is an outcome, and it is kept. What is an
// error is a task that is already going, which is the one thing pressing Run cannot be allowed to do
// twice — the two runs would write the same file.
func (s *scheduler) fire(task schedule.Task) (schedule.Run, error) {
	if !s.claim(task.ID) {
		return schedule.Run{}, domain.ErrTaskRunning
	}
	defer s.release(task.ID)

	s.emit(TaskEvent{TaskID: task.ID, Running: true})

	started := time.Now()
	outcome, summary, detail := s.api.doTask(task)

	run := schedule.Run{
		At:        started,
		ElapsedMs: time.Since(started).Milliseconds(),
		Outcome:   outcome,
		Summary:   summary,
		Detail:    detail,
	}

	// The clock moves on the moment the run is written down, and it counts from when the run
	// *started*: a nightly backup that took an hour is still last night's two o'clock backup.
	_ = s.api.tasks.Record(task.ID, run)

	s.emit(TaskEvent{TaskID: task.ID, Running: false, Run: taskRunDTO(task.ID, run)})
	return run, nil
}

func (s *scheduler) running(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.inFlight[id]
}

// claim marks a task as going, and answers false where it already was.
func (s *scheduler) claim(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inFlight[id] {
		return false
	}
	s.inFlight[id] = true
	return true
}

func (s *scheduler) release(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.inFlight, id)
}

func (s *scheduler) emit(event TaskEvent) {
	wailsruntime.EventsEmit(s.api.ctx, EventTaskRun, event)
}
