package objectstore

import (
	"context"
	"strings"
)

// WalkRequest is every object under a prefix, however deep. It is what a search, a folder download,
// a folder delete and a folder copy are all made of — one shape, so none of the four is a second
// way of reading a bucket.
type WalkRequest struct {
	Bucket string
	Prefix string
	// Cap is the most keys to look at before stopping, and zero is no bound at all. It exists
	// because a bucket has no size a reader can see before they ask: a walk that read forty million
	// keys because somebody typed a letter into a box is the failure this bounds, and `Walked.Whole`
	// is what says the answer is a window rather than the bucket.
	Cap int
}

// Walked is what a walk came to: how many keys it looked at, and whether that was all of them.
// `Whole` false is the only honest thing a bounded read can say about a bucket — see
// `docs/features/object-search.md`.
type Walked struct {
	Scanned int
	Whole   bool
}

// ErrStopWalk ends a walk early without being a failure, which is what a search that has found
// enough answers with. It is the walk's own sentinel rather than a `domain` one because nothing
// outside this package ever sees it: `Walk` swallows it and reports the walk as bounded.
var ErrStopWalk = stopWalk{}

type stopWalk struct{}

func (stopWalk) Error() string { return "walk stopped" }

// Walk hands every object under a prefix to `each`, one at a time, as the pages arrive.
//
// **Nothing is accumulated.** A folder holding a million objects is a million calls to `each` and
// one page of memory, which is what lets a download of that folder start moving bytes before the
// listing has finished — and is the whole reason a walk is a function over `List` rather than a
// method that answers with a slice. It is the bargain `transfer`'s readers and `dump.go` already
// struck.
//
// A walk asks for **no delimiter**, which is what "however deep" means to a service: the answer is
// every key under the prefix and no folders at all.
func Walk(ctx context.Context, store Store, request WalkRequest, each func(Object) error) (Walked, error) {
	walked := Walked{Whole: true}
	token := ""

	for {
		if err := ctx.Err(); err != nil {
			return walked, err
		}

		page, err := store.List(ctx, ListRequest{
			Bucket: request.Bucket,
			Prefix: request.Prefix,
			Token:  token,
			Limit:  PageSize,
		})
		if err != nil {
			return walked, err
		}

		for at, object := range page.Objects {
			// A key ending in the separator is a folder marker a console wrote, not an object
			// anybody meant to download. It is skipped rather than reported, and it is skipped here
			// so that every caller of a walk is spared the same line.
			if strings.HasSuffix(object.Key, Separator) && object.Size == 0 {
				continue
			}

			walked.Scanned++
			if err := each(object); err != nil {
				if err == ErrStopWalk {
					walked.Whole = false
					return walked, nil
				}
				return walked, err
			}

			// A cap reached on the last key of the last page really has seen the whole prefix, and
			// saying otherwise would put "there may be more" under a listing that is complete.
			if request.Cap > 0 && walked.Scanned >= request.Cap {
				walked.Whole = page.Next == "" && at == len(page.Objects)-1
				return walked, nil
			}
		}

		if page.Next == "" {
			return walked, nil
		}
		token = page.Next
	}
}
