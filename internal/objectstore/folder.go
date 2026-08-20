package objectstore

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// FolderProgress is how far a folder-wide change has got, and it is called as the batches go out
// rather than at the end. A delete of a million objects is a thousand requests and several minutes:
// a reader watching it deserves a number that moves, and the alternative is a dialog that says
// nothing until it is over.
type FolderProgress func(done int, last string)

// Removed is what a delete of a whole prefix came to. `Refused` is the field that has to be there:
// a service reports a key it would not remove **inside a successful response**, so a delete of a
// thousand keys where two were refused is not a failure — and pretending it is would leave the
// reader unable to see which two.
type Removed struct {
	Deleted int
	Refused []string
}

// RemoveKeys deletes the keys it is given, in the batches the protocol takes. It is what a reader's
// ticked rows become, and it is the one place a delete request is made from.
func RemoveKeys(
	ctx context.Context,
	store Store,
	bucket string,
	keys []string,
	progress FolderProgress,
) (Removed, error) {
	removed := Removed{}

	for at := 0; at < len(keys); at += DeleteBatch {
		if err := ctx.Err(); err != nil {
			return removed, err
		}

		end := min(at+DeleteBatch, len(keys))
		batch := keys[at:end]

		refused, err := store.Delete(ctx, bucket, batch)
		if err != nil {
			return removed, err
		}

		removed.Deleted += len(batch) - len(refused)
		removed.Refused = append(removed.Refused, refused...)
		if progress != nil && len(batch) > 0 {
			progress(removed.Deleted, batch[len(batch)-1])
		}
	}
	return removed, nil
}

// RemovePrefix deletes everything under a prefix, streaming the walk into the batches. **Nothing is
// accumulated**: a folder of a million objects is a thousand requests and one batch of names in
// memory, which is the same bargain every other read of this package makes.
//
// The prefix itself goes last, because some consoles write a zero-length object to make a folder
// appear and a walk skips it — see `Walk`.
func RemovePrefix(
	ctx context.Context,
	store Store,
	bucket, prefix string,
	progress FolderProgress,
) (Removed, error) {
	removed := Removed{}
	batch := make([]string, 0, DeleteBatch)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		refused, err := store.Delete(ctx, bucket, batch)
		if err != nil {
			return err
		}
		removed.Deleted += len(batch) - len(refused)
		removed.Refused = append(removed.Refused, refused...)
		if progress != nil {
			progress(removed.Deleted, batch[len(batch)-1])
		}
		batch = batch[:0]
		return nil
	}

	_, err := Walk(ctx, store, WalkRequest{Bucket: bucket, Prefix: prefix}, func(object Object) error {
		batch = append(batch, object.Key)
		if len(batch) < DeleteBatch {
			return nil
		}
		return flush()
	})
	if err != nil {
		return removed, err
	}
	if err := flush(); err != nil {
		return removed, err
	}

	// The marker object, if the reader's console ever wrote one. A prefix that never had one is a
	// key that is not there, which every service answers to a delete without complaint.
	if strings.HasSuffix(prefix, Separator) {
		if _, err := store.Delete(ctx, bucket, []string{prefix}); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// Copied is what a copy of a prefix came to.
type Copied struct {
	Objects int
	Bytes   int64
}

// CopyPrefix copies everything under one prefix to another, **without a single byte coming here**:
// a copy is one request to the service naming both keys, so a hundred gigabytes moves between two
// buckets at the service's own speed and over none of the reader's bandwidth. That is the whole
// reason a copy is not a download followed by an upload.
//
// It refuses an object past the protocol's single-request bound rather than starting something it
// cannot finish — `domain.Unsupported` naming it, which is what this app says everywhere else about
// a thing an engine has not got.
func CopyPrefix(
	ctx context.Context,
	store Store,
	bucket, from string,
	toBucket, to string,
	progress FolderProgress,
) (Copied, error) {
	copied := Copied{}

	// A folder copied onto itself, or into something beneath itself, is a copy that would find its
	// own output and go round for ever. It is refused here because a walk cannot notice.
	if bucket == toBucket && (to == from || strings.HasPrefix(to, from)) {
		return copied, domain.Unsupported("copying a folder into itself")
	}

	_, err := Walk(ctx, store, WalkRequest{Bucket: bucket, Prefix: from}, func(object Object) error {
		if object.Size > CopyLimit {
			return domain.Unsupported("copying an object larger than 5 GB in one request")
		}

		target := to + strings.TrimPrefix(object.Key, from)
		if err := Retry(ctx, func() error {
			return store.Copy(ctx, bucket, object.Key, toBucket, target)
		}); err != nil {
			return err
		}

		copied.Objects++
		copied.Bytes += object.Size
		if progress != nil {
			progress(copied.Objects, object.Key)
		}
		return nil
	})
	return copied, err
}
