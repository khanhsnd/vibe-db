package objectstore

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// SearchRequest is a name to look for and where to look for it. It is
// [rule 14](../../CLAUDE.md#rules-that-must-not-be-broken) in a bucket: **what the reader's text
// matches is a `domain` rule** — `domain.TextMatch`, the same one a Find in Schema is spelled from,
// so `Contains` means of a key exactly what it means of a column name — and how much of the bucket
// has to be read to answer it is this package's.
//
// The three bounds beside the text are free. They are facts a listing already carries for every
// key, so narrowing by them costs no request at all — which is exactly why they are here and not a
// second read.
type SearchRequest struct {
	Bucket string
	// Prefix is where the reader was standing, and it is the **whole** of what the service can be
	// asked to do about a name: S3 narrows a listing one way and one way only. See PruneTo.
	Prefix string
	Match  domain.TextMatch
	// MinSize and MaxSize bound the size in bytes, and zero on either is no bound.
	MinSize int64
	MaxSize int64
	// Since and Until bound when the object was last written, and a zero time on either is no bound.
	Since time.Time
	Until time.Time
	// Hits is how many answers to stop at, and zero is DefaultHits. A search of a bucket is not a
	// listing of it: a reader looking for a name wants the first screenful of matches, and reading
	// forty million keys to number them all is a read nobody asked for.
	Hits int
	// Scan is how many keys to look at before giving up, and zero is DefaultScan.
	Scan int
}

const (
	// DefaultHits is how many matches a search answers with. Two hundred is chosen for the reason a
	// page of rows is: it is more than a reader reads and it is one bound method's answer.
	DefaultHits = 200
	// DefaultScan is how many keys a search will look at. A quarter of a million is about two
	// hundred and fifty listings — a few seconds against a service, and small enough that a search
	// nobody meant to run cannot become an afternoon of somebody's bandwidth.
	DefaultScan = 250_000
)

// SearchAnswer is what a search found and what it cost. Every field but the first is there so the
// answer can say **what it did not see**: a bucket has no size a reader can know before asking, so
// a search that stopped is the ordinary case rather than the failure one.
type SearchAnswer struct {
	Hits []Object
	// Scanned is how many keys were looked at, which is what the pane draws beside the count.
	Scanned int
	// Prefix is what the service was actually narrowed by, which is the search's own explanation of
	// why it was quick — see PruneTo. It is shown, for the reason a table read shows its statement.
	Prefix string
	// Whole says every key under the prefix was looked at. False is a window: either the scan
	// bound was reached, or enough matches were found to stop.
	Whole bool
	// Full says the search stopped because it had found `Hits` matches rather than because it ran
	// out of keys, which is a different sentence to the reader: there are more of these.
	Full bool
}

// Search reads the bucket and answers with the keys whose names match. It is a walk with a
// predicate, and it holds **nothing but the hits**: a quarter of a million keys go past one at a
// time and the ones that do not match are forgotten as they go, which is what makes searching a
// forty-million-object bucket cost a page of memory rather than a copy of somebody's index.
//
// There is **no index and there will not be one.** An index of a bucket is a copy of somebody
// else's data on this machine, stale from the moment it is written and wrong in the one way that
// matters — it would answer confidently about an object that is no longer there. What this does
// instead is make the *service* do as much of the work as it can (PruneTo) and stream the rest.
func Search(ctx context.Context, store Store, request SearchRequest) (SearchAnswer, error) {
	matcher, err := request.Match.Matcher()
	if err != nil {
		return SearchAnswer{}, err
	}

	hits := request.Hits
	if hits <= 0 {
		hits = DefaultHits
	}
	scan := request.Scan
	if scan <= 0 {
		scan = DefaultScan
	}

	prefix := PruneTo(request.Prefix, request.Match)
	answer := SearchAnswer{Prefix: prefix}

	walked, err := Walk(ctx, store, WalkRequest{
		Bucket: request.Bucket,
		Prefix: prefix,
		Cap:    scan,
	}, func(object Object) error {
		if !request.keeps(object) || !matcher.Matches(object.Key) {
			return nil
		}

		answer.Hits = append(answer.Hits, object)
		if len(answer.Hits) >= hits {
			answer.Full = true
			return ErrStopWalk
		}
		return nil
	})
	if err != nil {
		return answer, err
	}

	answer.Scanned = walked.Scanned
	answer.Whole = walked.Whole
	return answer, nil
}

// keeps is the three bounds beside the text, and they are applied **before** the matcher on
// purpose: a size comparison is two integers and a regular expression is not, so the cheap
// question goes first over a quarter of a million keys.
func (r SearchRequest) keeps(object Object) bool {
	if r.MinSize > 0 && object.Size < r.MinSize {
		return false
	}
	if r.MaxSize > 0 && object.Size > r.MaxSize {
		return false
	}
	if !r.Since.IsZero() && object.Modified.Before(r.Since) {
		return false
	}
	if !r.Until.IsZero() && object.Modified.After(r.Until) {
		return false
	}
	return true
}

// PruneTo is the prefix the *service* is narrowed by, and it is this feature's answer to the one
// question a bucket makes expensive: how much has to be read.
//
// A service can be asked exactly one thing about a name — that it begins with these characters —
// which is the same bargain [a key space](../../docs/features/key-space.md) strikes with its glob.
// So a match whose front is **literal** hands that front to the service and reads nothing else:
// `MatchPrefix` with `logs/2024-01` at the top of a bucket becomes a listing of `logs/2024-01`, and a
// bucket of forty million keys answers in one request. It is also, for that reason, matched against
// the **whole key** — which is why the bar does not make it the default: a file name typed into it
// finds nothing, a key being a path. See docs/features/object-search.md.
//
// Two things deliberately prune to nothing rather than to something clever:
//
//   - **A match that ignores case.** Keys are case sensitive at every service there is, so
//     `logs/` would not find `Logs/` — a pruned listing would be missing rows the reader asked
//     for, which is worse than a slow answer. Where the reader is *standing* still prunes.
//   - **A regular expression that is not anchored.** `[0-9]+\.log` may match anywhere in a key, so
//     nothing about its front is known. One that opens with `^` is anchored and its literal head
//     is used, which is what makes an anchored expression the fast one to type.
func PruneTo(prefix string, match domain.TextMatch) string {
	literal := literalHead(match)
	if literal == "" {
		return prefix
	}
	// The reader's own folder wins where the two disagree: a search run inside `logs/` for names
	// beginning `2024` means `logs/2024`, and a literal that is already the whole path is not
	// doubled onto it.
	if strings.HasPrefix(literal, prefix) {
		return literal
	}
	return prefix + literal
}

// literalHead is the run of characters every match must begin with, and empty where nothing about
// the front of a match is known. Only two of the four modes can say anything at all: a fragment
// may sit anywhere in a key and a word has an edge in front of it.
func literalHead(match domain.TextMatch) string {
	if match.Text == "" || match.CaseInsensitive {
		return ""
	}

	switch match.Mode {
	case domain.MatchPrefix:
		return patternHead(match.Text)
	case domain.MatchRegex:
		if !strings.HasPrefix(match.Text, "^") {
			return ""
		}
		compiled, err := regexp.Compile(match.Text)
		if err != nil {
			return ""
		}
		head, _ := compiled.LiteralPrefix()
		return head
	default:
		return ""
	}
}

// patternHead is a LIKE pattern's literal front — everything before its first wildcard, with an
// escaped one counting as the character it stands for. It reads the pattern the same way
// `domain.patternExpression` does, which is what keeps a pruned listing and the matcher that runs
// over it from disagreeing about where the literal ends.
func patternHead(pattern string) string {
	var head strings.Builder
	escaped := false

	for _, character := range pattern {
		switch {
		case escaped:
			head.WriteRune(character)
			escaped = false
		case character == '\\':
			escaped = true
		case character == '%' || character == '_':
			return head.String()
		default:
			head.WriteRune(character)
		}
	}
	return head.String()
}
