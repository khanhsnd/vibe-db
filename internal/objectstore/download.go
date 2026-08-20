package objectstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

const (
	// CopyBuffer is how much of an object is in this process at one time, per range in flight. It is
	// the whole of what a download costs in memory: the bytes go socket → buffer → file and are
	// forgotten, so a forty-gigabyte object and a forty-kilobyte one cost the same.
	CopyBuffer = 256 << 10
	// PartSize is how much of an object one ranged request asks for. Eight megabytes is large
	// enough that the request's own round trip is noise and small enough that a part which failed
	// is cheap to ask for again.
	PartSize = 8 << 20
	// PartThreshold is the size at which an object is worth reading in ranges at all. Below it the
	// parallelism would be four requests racing over one file's worth of bytes, which is slower
	// than one request and harder to resume.
	PartThreshold = 16 << 20
	// Parts is how many ranges of **one** object are read at once. A single connection to an S3
	// service is throttled well below what a link can carry, so this is the number that makes a
	// large download fast — and it is per object, so the ceiling on sockets is this times the
	// queue's own workers.
	Parts = 4
	// partialSuffix is what a download in progress is called. The final name is taken by a rename
	// when it is complete, which is `jsonstore`'s own bargain: a file that is there is a file that
	// is whole.
	partialSuffix = ".vibedl"
	// planSuffix is the few hundred bytes beside it that say which ranges are already down. It is
	// what makes a twenty-gigabyte download survive a laptop lid.
	planSuffix = ".vibedl.json"
)

// Collision is what to do about a local file that is already there. It is the reader's answer and
// never a default this side picks: overwriting somebody's file is not a thing to be guessed at.
type Collision string

const (
	// CollisionAsk is no answer yet, and is what makes the wizard ask. A transfer started with it
	// skips, which is the one of the three that loses nothing.
	CollisionAsk       Collision = ""
	CollisionOverwrite Collision = "overwrite"
	CollisionSkip      Collision = "skip"
	// CollisionRename writes beside it — `report (2).csv` — which is what a browser does and what a
	// reader downloading the same object twice nearly always means.
	CollisionRename Collision = "rename"
	// CollisionNewer overwrites only where the object is newer than the file, which is what makes
	// a second run of a folder download a *sync* rather than a repeat.
	CollisionNewer Collision = "newer"
)

// DownloadPlan is one object onto this machine. `Size` and `ETag` come off the listing the reader
// is looking at, so an ordinary download asks the service nothing before it starts reading.
type DownloadPlan struct {
	Bucket string
	Key    string
	// Path is the final file. Its folder is made if it is not there.
	Path      string
	Size      int64
	ETag      string
	Modified  int64
	Collision Collision
	// PartSize and Parts override the constants above, and zero on either takes them. They are here
	// for the reader with a very fast link or a very unreliable one, and for a test.
	PartSize int64
	Parts    int
}

// Downloaded is what a download came to. `Verified` is the field worth reading twice: it says the
// bytes were checked against the service's **own** sum rather than merely counted, and it is false
// for every object the service uploaded in parts — see `verifiable`.
type Downloaded struct {
	Path  string
	Bytes int64
	// Skipped is a file that was already there and that the reader's own rule left alone.
	Skipped bool
	// Resumed is a download that carried on from what a previous run had already written.
	Resumed  bool
	Verified bool
}

// Download reads one object onto this machine, in ranges and in parallel where that is worth doing,
// resuming whatever a previous attempt got through.
//
// `moved` is called with a **delta** of bytes as they land, from several goroutines at once, so it
// has to be safe to call concurrently — the queue's counter is an atomic for exactly this reason.
// It is a number and not the bytes: nothing about a transfer's progress crosses the bridge except
// how far it has got.
func Download(ctx context.Context, store Store, plan DownloadPlan, moved func(int64)) (Downloaded, error) {
	if moved == nil {
		moved = func(int64) {}
	}

	path, skip, err := resolveCollision(plan)
	if err != nil {
		return Downloaded{}, err
	}
	if skip {
		return Downloaded{Path: plan.Path, Skipped: true}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Downloaded{}, domain.FileError{Err: err}
	}

	size := plan.Size
	etag := plan.ETag
	// A plan with no size is one made from something other than a listing — a key typed in, a
	// resume of a job whose listing is long gone — so the one fact the ranges are cut from is
	// asked for.
	if size <= 0 {
		object, err := store.Head(ctx, plan.Bucket, plan.Key)
		if err != nil {
			return Downloaded{}, err
		}
		size = object.Size
		etag = object.ETag
	}

	partSize := plan.PartSize
	if partSize <= 0 {
		partSize = PartSize
	}
	parts := plan.Parts
	if parts <= 0 {
		parts = Parts
	}

	partial := path + partialSuffix
	if size < PartThreshold || partSize <= 0 {
		return streamWhole(ctx, store, plan, path, partial, etag, moved)
	}
	return streamParts(ctx, store, plan, path, partial, size, etag, partSize, parts, moved)
}

// resolveCollision answers with the file to write and whether to write anything at all. It is the
// reader's rule applied once, before a single byte is asked for: a skip that read the object first
// would be a skip that cost the download.
func resolveCollision(plan DownloadPlan) (string, bool, error) {
	info, err := os.Stat(plan.Path)
	if errors.Is(err, os.ErrNotExist) {
		return plan.Path, false, nil
	}
	if err != nil {
		return "", false, domain.FileError{Err: err}
	}
	if info.IsDir() {
		// A folder where a file should go is not a collision the reader can answer with a rule:
		// nothing can be written there whatever they picked.
		return "", false, domain.FileError{Err: errors.New(plan.Path + " is a folder")}
	}

	switch plan.Collision {
	case CollisionOverwrite:
		return plan.Path, false, nil
	case CollisionRename:
		return besideIt(plan.Path), false, nil
	case CollisionNewer:
		// The object's own clock against the file's. Equal counts as unchanged, which is what makes
		// running the same folder download twice cost one listing and no bytes.
		if plan.Modified > info.ModTime().Unix() {
			return plan.Path, false, nil
		}
		return plan.Path, true, nil
	default:
		return plan.Path, true, nil
	}
}

// besideIt is the next free `name (n).ext`, which is the name a browser gives the same download
// twice and therefore the one a reader is not surprised by.
func besideIt(path string) string {
	extension := filepath.Ext(path)
	stem := strings.TrimSuffix(path, extension)

	for at := 2; at < 1000; at++ {
		candidate := stem + " (" + strconv.Itoa(at) + ")" + extension
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return stem + " (copy)" + extension
}

// streamWhole is one request, one file, one buffer. It is also the only path that can **verify**:
// the bytes go past in order, so the sum is taken as they go and costs nothing — where a parallel
// read cannot hash what it writes out of order without reading the file a second time.
func streamWhole(
	ctx context.Context,
	store Store,
	plan DownloadPlan,
	path, partial string,
	etag string,
	moved func(int64),
) (Downloaded, error) {
	// Whatever a previous attempt left is not carried on from here: a small object is quicker to
	// read again than it is to reason about, and a sum over half a file is no sum at all.
	os.Remove(partial)
	os.Remove(path + planSuffix)

	file, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Downloaded{}, domain.FileError{Err: err}
	}

	sum := md5.New()
	written := int64(0)

	err = Retry(ctx, func() error {
		// A retry starts the file over. That is the honest thing for a single stream: the service
		// may answer the second request from a different point, and a body appended to half of one
		// is a corrupt file that verifies as nothing.
		if written > 0 {
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return domain.FileError{Err: err}
			}
			if err := file.Truncate(0); err != nil {
				return domain.FileError{Err: err}
			}
			moved(-written)
			written = 0
			sum.Reset()
		}

		body, err := store.Open(ctx, plan.Bucket, plan.Key, Span{})
		if err != nil {
			return err
		}
		defer body.Close()

		count, err := copyBuffered(ctx, io.MultiWriter(file, sum), body, moved)
		written += count
		return err
	})
	if err != nil {
		file.Close()
		os.Remove(partial)
		return Downloaded{}, err
	}

	if err := finish(file, partial, path); err != nil {
		return Downloaded{}, err
	}

	verified := false
	if verifiable(etag) {
		if !strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), strings.Trim(etag, `"`)) {
			os.Remove(path)
			return Downloaded{}, domain.FileError{Err: errors.New(
				plan.Key + ": the bytes that arrived do not match the service's own checksum",
			)}
		}
		verified = true
	}

	return Downloaded{Path: path, Bytes: written, Verified: verified}, nil
}

// verifiable says the service's tag for this object is a plain MD5 of its bytes, which is true for
// an object uploaded in **one** request and false for one uploaded in parts — a multipart ETag is a
// sum of sums with a `-<count>` after it, and checking a file against one would fail every time.
// Saying so is the point: a download reports `Verified` false rather than claiming a check it did
// not make.
func verifiable(etag string) bool {
	tag := strings.Trim(etag, `"`)
	if len(tag) != 32 || strings.Contains(tag, "-") {
		return false
	}
	_, err := hex.DecodeString(tag)
	return err == nil
}

// downloadPlan is the few hundred bytes beside a partial file that say what is already down. It is
// read back on the next attempt and is why a lid closed at nineteen gigabytes costs nineteen
// gigabytes rather than twenty.
//
// It carries the **object's own identity** as well as the ranges: an object rewritten under the
// same key is a different object, so a plan whose etag or size no longer match is thrown away
// rather than resumed into a file that would be half of each.
type downloadPlan struct {
	ETag string `json:"etag"`
	Size int64  `json:"size"`
	Part int64  `json:"part"`
	// Done is one bit per part, hex encoded — three hundred and twenty bytes for a twenty-gigabyte
	// object, which is what makes writing it after every part free.
	Done string `json:"done"`
}

// streamParts reads one object as several ranges at once, into one file, and remembers which of
// them landed.
func streamParts(
	ctx context.Context,
	store Store,
	plan DownloadPlan,
	path, partial string,
	size int64,
	etag string,
	partSize int64,
	workers int,
	moved func(int64),
) (Downloaded, error) {
	count := int((size + partSize - 1) / partSize)
	planPath := path + planSuffix

	done, resumed := readPlan(planPath, etag, size, partSize, count)
	if !resumed {
		os.Remove(partial)
	}

	file, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Downloaded{}, domain.FileError{Err: err}
	}
	// The file is given its final length up front, so the operating system allocates it once
	// instead of growing it a part at a time — and so a resume can seek into it.
	if err := file.Truncate(size); err != nil {
		file.Close()
		return Downloaded{}, domain.FileError{Err: err}
	}

	already := int64(0)
	for at := 0; at < count; at++ {
		if done[at] {
			already += spanOf(at, count, partSize, size).Length
		}
	}
	if already > 0 {
		moved(already)
	}

	var (
		guard   sync.Mutex
		pending = make(chan int, workers)
		group   sync.WaitGroup
		failure atomic.Pointer[error]
	)

	// One context for the lot, cancelled by the first part that gives up: three ranges still
	// reading after a fourth has failed are three ranges of somebody's bandwidth spent on a file
	// that is not going to be written.
	partCtx, stop := context.WithCancel(ctx)
	defer stop()

	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for number := range pending {
				err := Retry(partCtx, func() error {
					return readPart(partCtx, store, plan, file, spanOf(number, count, partSize, size), moved)
				})
				if err != nil {
					failure.CompareAndSwap(nil, &err)
					stop()
					return
				}

				guard.Lock()
				done[number] = true
				writePlan(planPath, etag, size, partSize, done)
				guard.Unlock()
			}
		}()
	}

	for at := 0; at < count; at++ {
		if done[at] {
			continue
		}
		select {
		case pending <- at:
		case <-partCtx.Done():
		}
	}
	close(pending)
	group.Wait()

	if failed := failure.Load(); failed != nil {
		file.Close()
		// The partial file and its plan are **kept**: that is the whole point of them, and the next
		// attempt is what removes them by finishing.
		return Downloaded{}, *failed
	}
	if err := partCtx.Err(); err != nil {
		file.Close()
		return Downloaded{}, err
	}

	if err := finish(file, partial, path); err != nil {
		return Downloaded{}, err
	}
	os.Remove(planPath)

	return Downloaded{Path: path, Bytes: size, Resumed: resumed}, nil
}

// spanOf is which bytes part `number` is, with the last one being whatever is left over.
func spanOf(number, count int, partSize, size int64) Span {
	offset := int64(number) * partSize
	length := partSize
	if number == count-1 {
		length = size - offset
	}
	return Span{Offset: offset, Length: length}
}

// readPart reads one range and writes it where it belongs. `WriteAt` is what makes the parallelism
// possible at all — four goroutines writing four disjoint stretches of one file need no lock between
// them, and nothing is buffered to be stitched together afterwards.
func readPart(
	ctx context.Context,
	store Store,
	plan DownloadPlan,
	file *os.File,
	span Span,
	moved func(int64),
) error {
	body, err := store.Open(ctx, plan.Bucket, plan.Key, span)
	if err != nil {
		return err
	}
	defer body.Close()

	buffer := make([]byte, CopyBuffer)
	at := span.Offset
	remaining := span.Length

	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}

		room := buffer
		if remaining < int64(len(room)) {
			room = room[:remaining]
		}

		read, readErr := body.Read(room)
		if read > 0 {
			if _, err := file.WriteAt(room[:read], at); err != nil {
				return domain.FileError{Err: err}
			}
			at += int64(read)
			remaining -= int64(read)
			moved(int64(read))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			// A body that stopped early is the commonest failure of a long download, and it is
			// worth another attempt — which the part loop's own Retry then makes. The bytes already
			// written stay where they are, and the retry reads the range again from its start, so
			// the count is put back.
			moved(-(at - span.Offset))
			return Retryable{Err: readErr}
		}
	}

	if remaining > 0 {
		moved(-(at - span.Offset))
		return Retryable{Err: errors.New("the service sent less of the range than it promised")}
	}
	return nil
}

// copyBuffered is `io.Copy` with a buffer this package owns and a progress call per read. It is
// written out rather than using `io.CopyBuffer` because the delta is the whole point: a copy that
// reported at the end would be a progress bar that moved once.
func copyBuffered(ctx context.Context, into io.Writer, from io.Reader, moved func(int64)) (int64, error) {
	buffer := make([]byte, CopyBuffer)
	total := int64(0)

	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		read, readErr := from.Read(buffer)
		if read > 0 {
			written, err := into.Write(buffer[:read])
			total += int64(written)
			moved(int64(written))
			if err != nil {
				return total, domain.FileError{Err: err}
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, Retryable{Err: readErr}
		}
	}
}

// finish closes the file and puts it in place under its real name, which is the moment a download
// becomes a file anything else on this machine can open.
func finish(file *os.File, partial, path string) error {
	if err := file.Sync(); err != nil {
		file.Close()
		return domain.FileError{Err: err}
	}
	if err := file.Close(); err != nil {
		return domain.FileError{Err: err}
	}
	// Windows will not rename onto a name that is taken, and an overwrite has already been agreed
	// to by this point — see resolveCollision.
	os.Remove(path)
	if err := os.Rename(partial, path); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

// readPlan reads what a previous attempt got through, and answers with nothing at all where the
// object has changed underneath it. `resumed` false is a download starting over.
func readPlan(path, etag string, size, partSize int64, count int) ([]bool, bool) {
	done := make([]bool, count)

	contents, err := os.ReadFile(path)
	if err != nil {
		return done, false
	}

	var saved downloadPlan
	if err := json.Unmarshal(contents, &saved); err != nil {
		return done, false
	}
	if saved.Size != size || saved.Part != partSize || saved.ETag != etag {
		return done, false
	}

	bits, err := hex.DecodeString(saved.Done)
	if err != nil {
		return done, false
	}

	any := false
	for at := 0; at < count; at++ {
		byteAt := at / 8
		if byteAt >= len(bits) {
			break
		}
		if bits[byteAt]&(1<<(at%8)) != 0 {
			done[at] = true
			any = true
		}
	}
	return done, any
}

// writePlan puts the ranges that are down beside the partial file. A failure to write it is
// deliberately ignored: it would cost the reader a resume and not the download, and refusing to
// carry on because a hint could not be saved would be the wrong trade.
func writePlan(path, etag string, size, partSize int64, done []bool) {
	bits := make([]byte, (len(done)+7)/8)
	for at, ok := range done {
		if ok {
			bits[at/8] |= 1 << (at % 8)
		}
	}

	contents, err := json.Marshal(downloadPlan{
		ETag: etag,
		Size: size,
		Part: partSize,
		Done: hex.EncodeToString(bits),
	})
	if err != nil {
		return
	}
	os.WriteFile(path, contents, 0o600)
}
