package objectstore

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Kind is which way the bytes are going. There are two, and there will not be a third: a delete and
// a copy move no bytes through this machine at all, so neither is a transfer — see `folder.go`.
type Kind string

const (
	KindDownload Kind = "download"
	KindUpload   Kind = "upload"
)

// Item is one file's worth of work. It means the same thing both ways round — an object and a file
// on this machine — and which of the two is the source is the job's `Kind`.
type Item struct {
	Key  string
	Path string
	Size int64
	ETag string
	// Modified is the object's own clock as a Unix second, which is what `CollisionNewer` compares
	// a file's against.
	Modified int64
}

// Job is a piece of work a reader asked for — one object, a folder, the rows they ticked — and it is
// a **producer rather than a list**.
//
// That is the design decision this whole file exists for. A folder holding a million objects must
// not become a million-entry queue in memory before the first byte moves, so `Each` hands items over
// one at a time and the queue's own back pressure is what bounds how far ahead the listing runs. It
// is `Walk`'s bargain, one level up: what a job costs is a page of names, whatever it is a job *of*.
type Job struct {
	ID     string
	Kind   Kind
	Bucket string
	// Label is what the reader asked for, in their words, for the panel to draw: `logs/2024/`, or an
	// object's own name.
	Label string
	// Root is the local folder a download writes under, or the folder an upload was picked from. It
	// is here so the panel can say where the bytes went without the caller keeping a second copy.
	Root      string
	Collision Collision
	Each      func(ctx context.Context, yield func(Item) error) error
}

// JobState is where a job has got to. The last four are all *finished* and are four different
// sentences: everything went, nothing went, some went, and the reader stopped it.
type JobState string

const (
	JobWaiting   JobState = "waiting"
	JobRunning   JobState = "running"
	JobDone      JobState = "done"
	JobFailed    JobState = "failed"
	JobPartial   JobState = "partial"
	JobCancelled JobState = "cancelled"
)

// Finished says the job will not move again, which is what decides whether the panel keeps watching
// it and whether Clear may forget it.
func (s JobState) Finished() bool {
	return s == JobDone || s == JobFailed || s == JobPartial || s == JobCancelled
}

// Running is one file in flight, with how much of it has landed. It is what makes a transfer panel
// worth having open: a job says how many files are left and this says how the one under way is
// doing.
type Running struct {
	JobID string
	Key   string
	Path  string
	Size  int64
	Moved int64
}

// JobReport is a job as the panel draws a row for it.
//
// `Counting` is the field that keeps it honest. A folder's size is not known until its listing has
// finished, so `Files` and `Bytes` climb while the walk runs — and a progress bar drawn from a total
// that is still moving is a bar that goes backwards. While this is true the panel draws the count
// and not a percentage, which is the truth: nobody knows yet.
type JobReport struct {
	ID       string
	Kind     Kind
	Bucket   string
	Label    string
	Root     string
	State    JobState
	Files    int
	Done     int
	Failed   int
	Skipped  int
	Bytes    int64
	Moved    int64
	Counting bool
	// Rate is bytes a second over the last few samples, and zero for a job that is not moving.
	Rate int64
	// Detail is why it failed, or what was refused — the service's own words where there are any.
	Detail  string
	Started time.Time
	Ended   time.Time
}

// Report is the whole queue as one answer: the jobs, and the files in flight. It is bounded on
// purpose — `Jobs` is capped by `historyCap` and `Running` by the worker count — because it crosses
// the bridge several times a second and a queue of ten thousand files must not become ten thousand
// rows of JSON on a timer.
type Report struct {
	Jobs    []JobReport
	Running []Running
	// Active is how many jobs are still going, which is what decides whether the panel keeps asking.
	Active int
}

const (
	// Workers is how many files move at once across the whole queue. Four times the four ranges a
	// large download reads is sixteen sockets, which is a link busy and a service unbothered.
	Workers = 4
	// lookahead is how many items a job's listing may run ahead of its workers. It is the back
	// pressure: a walk that filled a channel of a million would be a walk that held a million names.
	lookahead = 32
	// historyCap is how many finished jobs are kept for the panel to show. Past it the oldest is
	// forgotten — a transfer panel is a thing a reader glances at, not a ledger.
	historyCap = 50
	// sampleEvery is how often the queue takes a rate sample. One second is short enough to feel
	// live and long enough that the number is not noise.
	sampleEvery = time.Second
)

// Queue is every transfer this app has been asked for. One of them belongs to the whole app rather
// than to a tab: a download outlives the tab it was started from, which is the first thing a reader
// finds out by closing one.
type Queue struct {
	// slots is the whole of the global bound: a worker takes one before it moves a byte and gives it
	// back afterwards, so the number of files in flight is this deep however many jobs are running.
	slots chan struct{}

	mu      sync.Mutex
	jobs    []*jobRun
	running map[string]*Running

	stop chan struct{}
	once sync.Once
}

type jobRun struct {
	report JobReport
	cancel context.CancelFunc
	// sampled is what `Moved` was at the last rate sample, which is how a rate is worked out without
	// keeping a history of anything.
	sampled int64
}

func NewQueue(workers int) *Queue {
	if workers <= 0 {
		workers = Workers
	}

	queue := &Queue{
		slots:   make(chan struct{}, workers),
		running: make(map[string]*Running),
		stop:    make(chan struct{}),
	}
	go queue.sample()
	return queue
}

// Submit starts a job and answers at once: everything about a transfer happens on its own
// goroutines, because the bound method that asked for it has to return before the first byte moves.
func (q *Queue) Submit(ctx context.Context, store Store, job Job) {
	jobCtx, cancel := context.WithCancel(ctx)

	run := &jobRun{
		report: JobReport{
			ID:       job.ID,
			Kind:     job.Kind,
			Bucket:   job.Bucket,
			Label:    job.Label,
			Root:     job.Root,
			State:    JobRunning,
			Counting: true,
			Started:  time.Now(),
		},
		cancel: cancel,
	}

	q.mu.Lock()
	q.jobs = append(q.jobs, run)
	q.trim()
	q.mu.Unlock()

	go q.work(jobCtx, store, job, run)
}

// work is one job: a producer walking what the reader asked for, and workers taking items off it.
func (q *Queue) work(ctx context.Context, store Store, job Job, run *jobRun) {
	items := make(chan Item, lookahead)
	var group sync.WaitGroup

	group.Add(1)
	go func() {
		defer group.Done()
		defer close(items)

		err := job.Each(ctx, func(item Item) error {
			q.mu.Lock()
			run.report.Files++
			run.report.Bytes += item.Size
			q.mu.Unlock()

			select {
			case items <- item:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		q.mu.Lock()
		run.report.Counting = false
		if err != nil && !errors.Is(err, context.Canceled) {
			run.report.Detail = err.Error()
			run.report.Failed++
		}
		q.mu.Unlock()
	}()

	for range cap(q.slots) {
		group.Add(1)
		go func() {
			defer group.Done()
			for item := range items {
				q.move(ctx, store, job, run, item)
			}
		}()
	}

	group.Wait()
	q.settle(ctx, run)
}

// move is one file, and it is where a slot is held. The slot is taken **here** rather than around
// the whole job, so a job of one file does not hold a quarter of the queue while its listing runs.
func (q *Queue) move(ctx context.Context, store Store, job Job, run *jobRun, item Item) {
	select {
	case q.slots <- struct{}{}:
		defer func() { <-q.slots }()
	case <-ctx.Done():
		return
	}
	if ctx.Err() != nil {
		return
	}

	live := &Running{JobID: job.ID, Key: item.Key, Path: item.Path, Size: item.Size}
	key := job.ID + "\x00" + item.Key

	q.mu.Lock()
	q.running[key] = live
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		delete(q.running, key)
		q.mu.Unlock()
	}()

	// The progress call comes from several goroutines at once — a download reads four ranges — so
	// the counter is taken under the same lock the report is read under. It is a lock per buffer
	// rather than per byte: at 256 KiB a call, a gigabit link takes it about five hundred times a
	// second.
	moved := func(delta int64) {
		q.mu.Lock()
		live.Moved += delta
		run.report.Moved += delta
		q.mu.Unlock()
	}

	var err error
	skipped := false

	switch job.Kind {
	case KindUpload:
		_, err = Upload(ctx, store, UploadPlan{
			Bucket: job.Bucket,
			Key:    item.Key,
			Path:   item.Path,
		}, moved)
	default:
		var done Downloaded
		done, err = Download(ctx, store, DownloadPlan{
			Bucket:    job.Bucket,
			Key:       item.Key,
			Path:      item.Path,
			Size:      item.Size,
			ETag:      item.ETag,
			Modified:  item.Modified,
			Collision: job.Collision,
		}, moved)
		skipped = done.Skipped
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	switch {
	case err != nil && errors.Is(err, context.Canceled):
		// A cancelled file is neither done nor failed: the reader stopped it, and `settle` is what
		// says so about the job as a whole.
	case err != nil:
		run.report.Failed++
		run.report.Detail = err.Error()
	case skipped:
		run.report.Skipped++
		// A file that was left alone still counts towards the total, or the bar would never fill.
		run.report.Moved += item.Size
	default:
		run.report.Done++
	}
}

// settle is the sentence a finished job gets. Four outcomes, because they are four different things
// to have happened and a reader acts differently on each.
func (q *Queue) settle(ctx context.Context, run *jobRun) {
	q.mu.Lock()
	defer q.mu.Unlock()

	run.report.Counting = false
	run.report.Ended = time.Now()
	run.report.Rate = 0

	switch {
	case ctx.Err() != nil:
		run.report.State = JobCancelled
	case run.report.Failed == 0:
		run.report.State = JobDone
	case run.report.Done > 0 || run.report.Skipped > 0:
		run.report.State = JobPartial
	default:
		run.report.State = JobFailed
	}
}

// sample is the queue's own clock, and the only thing here that runs when nothing is happening. It
// turns the byte counter into a rate, which cannot be worked out from one reading of it.
func (q *Queue) sample() {
	ticker := time.NewTicker(sampleEvery)
	defer ticker.Stop()

	for {
		select {
		case <-q.stop:
			return
		case <-ticker.C:
			q.mu.Lock()
			for _, run := range q.jobs {
				if run.report.State != JobRunning {
					continue
				}
				run.report.Rate = (run.report.Moved - run.sampled) * int64(time.Second/sampleEvery)
				run.sampled = run.report.Moved
			}
			q.mu.Unlock()
		}
	}
}

// Report is the queue as the panel draws it, newest job first.
func (q *Queue) Report() Report {
	q.mu.Lock()
	defer q.mu.Unlock()

	report := Report{
		Jobs:    make([]JobReport, 0, len(q.jobs)),
		Running: make([]Running, 0, len(q.running)),
	}

	for at := len(q.jobs) - 1; at >= 0; at-- {
		run := q.jobs[at]
		report.Jobs = append(report.Jobs, run.report)
		if !run.report.State.Finished() {
			report.Active++
		}
	}
	for _, live := range q.running {
		report.Running = append(report.Running, *live)
	}
	return report
}

// Cancel stops one job. The files already written stay written — a download of forty files that was
// stopped at the thirtieth is thirty files, and pretending otherwise by deleting them would be the
// surprise — and a large file part way through keeps its partial and its plan, so starting the same
// job again carries on from there.
func (q *Queue) Cancel(jobID string) {
	q.mu.Lock()
	var cancel context.CancelFunc
	for _, run := range q.jobs {
		if run.report.ID == jobID && !run.report.State.Finished() {
			cancel = run.cancel
		}
	}
	q.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// CancelAll stops everything, which is what closing the app does and what the panel's own Stop All
// is.
func (q *Queue) CancelAll() {
	q.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(q.jobs))
	for _, run := range q.jobs {
		if !run.report.State.Finished() {
			cancels = append(cancels, run.cancel)
		}
	}
	q.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// Clear forgets the jobs that have finished. It takes nothing off the disk and stops nothing: it is
// the panel being tidied.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	kept := q.jobs[:0]
	for _, run := range q.jobs {
		if !run.report.State.Finished() {
			kept = append(kept, run)
		}
	}
	q.jobs = kept
}

// trim keeps the finished list to `historyCap`, oldest first out. It is called with the lock held.
func (q *Queue) trim() {
	finished := 0
	for _, run := range q.jobs {
		if run.report.State.Finished() {
			finished++
		}
	}
	if finished <= historyCap {
		return
	}

	drop := finished - historyCap
	kept := make([]*jobRun, 0, len(q.jobs)-drop)
	for _, run := range q.jobs {
		if drop > 0 && run.report.State.Finished() {
			drop--
			continue
		}
		kept = append(kept, run)
	}
	q.jobs = kept
}

// Close stops the clock and everything still moving. It is the app shutting down, and it is why a
// transfer does not outlive the window it was started from: the bytes go through this process.
func (q *Queue) Close() {
	q.CancelAll()
	q.once.Do(func() { close(q.stop) })
}
