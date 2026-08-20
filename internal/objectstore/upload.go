package objectstore

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// UploadPlan is one file on this machine written into a bucket. It is `DownloadPlan` the other way
// about, and the asymmetry between the two is worth naming: a download can be resumed because the
// object it is reading does not change, and an upload cannot, because the file it is reading can.
type UploadPlan struct {
	Bucket string
	Key    string
	Path   string
	// ContentType is what the object is served as. Empty reads it from the file's own extension,
	// which is the one thing this side knows about a file's contents without opening it.
	ContentType string
	PartSize    int64
	Parts       int
}

// Uploaded is what an upload came to. `Multipart` says it went in pieces, which is the fact worth
// keeping: an object written that way has an ETag that is not a checksum, and a download of it can
// only be verified by its length — see `verifiable`.
type Uploaded struct {
	Bytes     int64
	Multipart bool
}

// Upload writes one file into a bucket, in parts and in parallel where it is big enough to be worth
// it.
//
// The parts are read out of the file with `ReadAt`, which is `WriteAt`'s counterpart in the
// download: four goroutines read four disjoint stretches of one file with no lock between them and
// nothing is held in memory but the buffers the requests are sending from.
//
// **A multipart upload that fails is aborted.** Parts already accepted by a service are parts a
// reader is being charged rent on until somebody notices, so the abort goes out on a context of its
// own — a reader who pressed Stop still wants the pieces cleaned up.
func Upload(ctx context.Context, store Store, plan UploadPlan, moved func(int64)) (Uploaded, error) {
	if moved == nil {
		moved = func(int64) {}
	}

	file, err := os.Open(plan.Path)
	if err != nil {
		return Uploaded{}, domain.FileError{Err: err}
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return Uploaded{}, domain.FileError{Err: err}
	}
	if info.IsDir() {
		return Uploaded{}, domain.FileError{Err: errors.New(plan.Path + " is a folder")}
	}

	contentType := plan.ContentType
	if contentType == "" {
		contentType = ContentTypeOf(plan.Path)
	}

	size := info.Size()
	partSize := plan.PartSize
	if partSize <= 0 {
		partSize = PartSize
	}
	if size < PartThreshold {
		return Uploaded{Bytes: size}, putWhole(ctx, store, plan, file, size, contentType, moved)
	}

	workers := plan.Parts
	if workers <= 0 {
		workers = Parts
	}
	return uploadParts(ctx, store, plan, file, size, partSize, workers, contentType, moved)
}

// putWhole is one request. It re-opens nothing on a retry — the file is seeked back to its start,
// because a service that refused the first attempt has read the body and will want it again.
func putWhole(
	ctx context.Context,
	store Store,
	plan UploadPlan,
	file *os.File,
	size int64,
	contentType string,
	moved func(int64),
) error {
	sent := int64(0)

	return Retry(ctx, func() error {
		if sent > 0 {
			moved(-sent)
			sent = 0
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return domain.FileError{Err: err}
		}

		return store.Put(ctx, PutRequest{
			Bucket:      plan.Bucket,
			Key:         plan.Key,
			Body:        &counter{from: file, moved: moved, total: &sent},
			Size:        size,
			ContentType: contentType,
		})
	})
}

// counter is a reader that reports what went past it. It is how an upload has a progress bar at
// all: the bytes are the request's body, so the only place to count them is on the way in.
type counter struct {
	from  io.Reader
	moved func(int64)
	total *int64
}

func (c *counter) Read(into []byte) (int, error) {
	read, err := c.from.Read(into)
	if read > 0 {
		*c.total += int64(read)
		c.moved(int64(read))
	}
	return read, err
}

func uploadParts(
	ctx context.Context,
	store Store,
	plan UploadPlan,
	file *os.File,
	size, partSize int64,
	workers int,
	contentType string,
	moved func(int64),
) (Uploaded, error) {
	uploadID, err := store.CreateUpload(ctx, plan.Bucket, plan.Key, contentType)
	if err != nil {
		return Uploaded{}, err
	}

	count := int((size + partSize - 1) / partSize)

	var (
		guard   sync.Mutex
		parts   = make([]Part, 0, count)
		pending = make(chan int, workers)
		group   sync.WaitGroup
		failure atomic.Pointer[error]
	)

	partCtx, stop := context.WithCancel(ctx)
	defer stop()

	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for number := range pending {
				span := spanOf(number, count, partSize, size)
				sent := int64(0)

				err := Retry(partCtx, func() error {
					if sent > 0 {
						moved(-sent)
						sent = 0
					}
					section := io.NewSectionReader(file, span.Offset, span.Length)
					part, err := store.UploadPart(
						partCtx, plan.Bucket, plan.Key, uploadID, number+1,
						&counter{from: section, moved: moved, total: &sent}, span.Length,
					)
					if err != nil {
						return err
					}

					guard.Lock()
					parts = append(parts, part)
					guard.Unlock()
					return nil
				})
				if err != nil {
					failure.CompareAndSwap(nil, &err)
					stop()
					return
				}
			}
		}()
	}

	for at := 0; at < count; at++ {
		select {
		case pending <- at:
		case <-partCtx.Done():
		}
	}
	close(pending)
	group.Wait()

	failed := failure.Load()
	if failed == nil && partCtx.Err() != nil {
		cancelled := partCtx.Err()
		failed = &cancelled
	}
	if failed != nil {
		abandon(store, plan, uploadID)
		return Uploaded{}, *failed
	}

	// A service takes the parts in order however they arrived, so the list is put back in order
	// here rather than the workers being made to finish in one.
	sort.Slice(parts, func(a, b int) bool { return parts[a].Number < parts[b].Number })

	if err := store.CompleteUpload(ctx, plan.Bucket, plan.Key, uploadID, parts); err != nil {
		abandon(store, plan, uploadID)
		return Uploaded{}, err
	}
	return Uploaded{Bytes: size, Multipart: true}, nil
}

// abandon throws away the parts a failed upload left at the service. It runs on a context of its
// own — `WithoutCancel`, and a short deadline — because the commonest reason to be here is a reader
// who pressed Stop, and the pieces have to go whether or not their context has.
func abandon(store Store, plan UploadPlan, uploadID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 15*time.Second)
	defer cancel()
	store.AbortUpload(ctx, plan.Bucket, plan.Key, uploadID)
}

// ContentTypeOf is what an object is served as, read off the file's extension. **This app's own list
// wins**, and the machine's table answers for everything else.
//
// That order is the whole of the decision here, and it is deliberate: the machine's table is the
// Windows registry or somebody's `mime.types`, so the same CSV uploaded from two laptops would be
// served as two different things — this one says `application/vnd.ms-excel`, because Excel claimed
// the extension when it was installed. An object's type is a fact about the object and not about
// whoever happened to upload it.
//
// It matters more than it looks: a service serves back whatever it was told, so an HTML file
// uploaded as `application/octet-stream` is a page every browser downloads instead of showing.
func ContentTypeOf(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if kind, ok := contentTypes[extension]; ok {
		return kind
	}
	if kind := mime.TypeByExtension(extension); kind != "" {
		return kind
	}
	return "application/octet-stream"
}

// contentTypes is the list this app decides for itself. Every one of them is a file this app writes
// or reads — a dump, an export, a log — so a round trip through a bucket cannot change what it is,
// whatever the machine that did the uploading thinks.
var contentTypes = map[string]string{
	".csv":   "text/csv",
	".json":  "application/json",
	".jsonl": "application/x-ndjson",
	".log":   "text/plain",
	".md":    "text/markdown",
	".sql":   "application/sql",
	".tsv":   "text/tab-separated-values",
	".xml":   "application/xml",
	".yaml":  "application/yaml",
	".yml":   "application/yaml",
}
