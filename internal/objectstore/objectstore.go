// Package objectstore is a bucket full of objects, and it is `internal/driver`'s sibling rather
// than one of its adapters: there is no catalogue here, no statement, and nothing that is a row.
//
// The port is deliberately *small and low*. A `Store` is the **protocol and nothing else** — sign a
// request, list a page, read a range, write a part — and everything a reader actually asks for is
// written **once above it**: walking a prefix, searching for a name, downloading a folder in
// parallel, resuming what was interrupted. That is `sqlkit.DDL`'s arrangement applied to a service
// instead of to five SQL engines, and it is what keeps a second provider from becoming a second
// downloader.
//
// One rule here is not negotiable and everything else follows from it: **an object's bytes never
// pass through a bound method.** They go from the socket to the file and back, a fixed buffer at a
// time, so a forty-gigabyte download costs what a hundred-megabyte one does. What crosses the
// bridge is a *number* — how far it has got — which is why the panel can watch a transfer nothing
// on this side is holding.
package objectstore

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Dialer opens a connection to an address. It is `driver.Dialer`'s **shape and not its type**: an
// object store is not a database, so nothing in this package has heard of `internal/driver` — and
// both hops that carry a connection, `internal/netproxy` and `internal/sshtunnel`, hand back
// exactly this function. A store behind a bastion therefore costs this package nothing.
type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

// Separator is the one character a key is read as a path by. It is not a setting: `/` is what every
// console, SDK and command-line tool there is draws a folder with, and a key space is flat
// underneath all of them. A prefix ending in it is a *folder*; a key not ending in it is an object.
const Separator = "/"

// Credentials is the key pair a store is reached with. It is the port's own shape rather than the
// signer's: what an adapter does with these three strings is the adapter's business, and the layer
// that reads them out of the credential store should not have to know that one of them is signed
// with and two are sent.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	// SessionToken is present for a temporary credential — an assumed role, an SSO login — and
	// empty for a long-lived key.
	SessionToken string
}

// Opener is what stands where `driver.Registry` stands, and it is a **function rather than a
// registry** because there is one adapter and a registry of one says nothing. The seam is here for
// the reason the registry is there: `internal/session` opens a store without importing the thing
// that speaks the protocol, so an adapter is named in exactly one place (`internal/app/app.go`).
type Opener func(profile domain.StoreProfile, creds Credentials, dial Dialer) (Store, error)

// Bucket is one bucket, as the tree draws a row for it. `Region` is empty on a service that keeps
// its buckets nowhere in particular, which is most of the ones that are not AWS.
type Bucket struct {
	Name    string
	Created time.Time
	Region  string
}

// Object is one object as a *listing* reports it, which is deliberately less than a `Head` knows:
// a list answers with the four facts it carries for every key, and asking a service for a content
// type per row would be one request per row.
type Object struct {
	Key      string
	Size     int64
	Modified time.Time
	// ETag is the service's own tag for the bytes. It is an MD5 sum for an object written in one
	// piece and something else entirely for one written in parts — see `download.go`, which is why
	// nothing here treats it as a checksum without saying so.
	ETag         string
	StorageClass string
	// The three below are filled in by a `Head` and are empty in a listing, there being no such
	// column in the answer a service gives to a list.
	ContentType string
	// Metadata is what the *uploader* attached — the `x-amz-meta-` headers, with that prefix taken
	// off. It is the service's own store of somebody's own facts, so nothing here reads it.
	Metadata map[string]string
	// Headers is the whole of what the service said about the object. It is here because a header
	// **is** the protocol's answer: a content encoding, a cache rule, a disposition and a lock are
	// each one line of it, and a client that showed only the four it happened to parse would be
	// hiding the one the reader is looking for.
	//
	// They arrive **sorted by name** rather than in the order they were sent, because the transport
	// hands them over as a map and that order is gone before this package sees them. Saying so is
	// better than a comment claiming an order nothing preserves.
	Headers []Header
}

// Header is one line of a service's answer. Both halves travel verbatim: this is the one place in
// the app where what is shown is literally what came off the wire.
type Header struct {
	Name  string
	Value string
}

// Metadata is the part of an object's headers a service will let a client **change**, and it is a
// small part: six headers of the protocol's own and any number of the uploader's.
//
// Everything else the Header pane shows — the tag, the length, the last-modified, the request id —
// belongs to the *service* and cannot be written by anybody. Offering to edit those would be offering
// something no request exists for.
type Metadata struct {
	// System is the six below, keyed by their HTTP names so that what a pane shows and what is sent
	// are the same words.
	System map[string]string
	// User is the `x-amz-meta-` pairs with the prefix off.
	User map[string]string
	// StorageClass is carried through rather than changed: the request that rewrites metadata is a
	// **copy**, and a copy that named no class would silently move the object to the default one.
	StorageClass string
}

// SettableHeaders is the whole of what a service takes on that copy, in the order a pane draws them.
// It is a fixed list because the protocol's is fixed — not a guess about what might work.
var SettableHeaders = []string{
	"Content-Type",
	"Content-Disposition",
	"Content-Encoding",
	"Content-Language",
	"Cache-Control",
	"Expires",
}

// Settable says a header is one of the six, case insensitively, which is what tells an editable row
// from one the service owns.
func Settable(name string) bool {
	for _, one := range SettableHeaders {
		if strings.EqualFold(one, name) {
			return true
		}
	}
	return false
}

// Permission is one grant on an object: who, of what kind, and what they may do. The three are the
// service's own words, because who a `Group` grantee *is* — `AuthenticatedUsers`, `AllUsers` — is a
// fact about that service's own model and not something to translate.
type Permission struct {
	Grantee string
	// Kind is `CanonicalUser`, `Group` or `AmazonCustomerByEmail` — whatever the service said.
	Kind string
	// Allow is `READ`, `WRITE`, `READ_ACP`, `WRITE_ACP` or `FULL_CONTROL`.
	Allow string
}

// Permissions is an object's access-control policy. It is **read and never written** here: writing
// one is a whole grammar of canned names and grant lists, and a client that let a reader edit half
// of it would be a client that quietly dropped the other half.
//
// Not every service has them. Cloudflare R2 has no ACLs at all and others answer a canned policy, so
// a reader is shown what the service said — including its refusal — rather than an empty table that
// looks like "nobody may do anything".
type Permissions struct {
	Owner  string
	Grants []Permission
}

// Tag is one of an object's tags, which are the service's own key-value labels — not this app's, and
// not the uploader's metadata above.
type Tag struct {
	Name  string
	Value string
}

// Version is one version of an object, newest first. `Deleted` is a **delete marker**: a version
// whose whole content is the fact that the object was deleted at that moment, which is what makes a
// versioned bucket able to answer "it is gone" and "here is what it was" at once.
type Version struct {
	VersionID    string
	Latest       bool
	Deleted      bool
	Size         int64
	Modified     time.Time
	ETag         string
	StorageClass string
}

// Name is the object's last segment — what the grid draws in the folder it was listed in. A key
// ending in the separator is a folder marker some consoles write, and its name is the segment
// before that.
func (o Object) Name() string {
	return LastSegment(o.Key)
}

// LastSegment is the tail of a key or a prefix, with a trailing separator taken off first. It is
// the one place a name is cut out of a key, so a folder row and an object row cannot disagree about
// what they are called.
func LastSegment(key string) string {
	trimmed := strings.TrimSuffix(key, Separator)
	if at := strings.LastIndex(trimmed, Separator); at >= 0 {
		return trimmed[at+1:]
	}
	return trimmed
}

// ParentPrefix is the folder a key sits in, with its trailing separator, and empty at the top of a
// bucket. It is what Up walks to and what a download reproduces under the folder the reader picked.
func ParentPrefix(key string) string {
	trimmed := strings.TrimSuffix(key, Separator)
	if at := strings.LastIndex(trimmed, Separator); at >= 0 {
		return trimmed[:at+1]
	}
	return ""
}

// JoinKey puts a name under a prefix. The prefix carries its own separator or has none at all, so
// this is the one function that decides where that character goes and there is never a doubled one.
func JoinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return strings.TrimSuffix(prefix, Separator) + Separator + name
}

// ListRequest is one page of one folder. `Delimiter` is what makes it a folder rather than
// everything underneath: with it the service answers with the prefixes it found and the objects
// directly inside, and without it — which is what a walk asks for — it answers with every key under
// the prefix and no prefixes at all.
type ListRequest struct {
	Bucket    string
	Prefix    string
	Delimiter string
	// Token continues a listing the service did not finish. It is the service's own opaque
	// continuation token and is never read here.
	Token string
	// Limit is how many keys to ask for at most. Zero is the service's own default, which is a
	// thousand on every provider this app speaks to.
	Limit int
}

// Listing is one page of one folder: the folders inside it, the objects inside it, and whether
// there is more.
type Listing struct {
	// Folders are whole prefixes, each ending in the separator, in the service's own order.
	Folders []string
	Objects []Object
	// Next is what to hand back to read the page after this one, and empty when the listing is
	// complete. A page is *forward only* — that is what a continuation token is — so a caller reads
	// onward and appends, and going back would be a re-read of the whole bucket.
	Next string
}

// Span is a byte range of one object. A zero `Length` reads to the end, and a zero Span reads the
// whole object, which is what makes the common case the plain one.
type Span struct {
	Offset int64
	Length int64
}

// Whole says the span asks for the object entire, which is the one case that needs no Range header
// — a service that has never seen one still answers it.
func (s Span) Whole() bool {
	return s.Offset == 0 && s.Length == 0
}

// Part is one piece of a multipart upload, as the service acknowledged it. Both fields go into the
// completion request, so this is the whole of what an upload has to remember about a part.
type Part struct {
	Number int
	ETag   string
}

// PutRequest is one object written in a single request. Nothing streams here without a `Size`: an
// S3 request has to say how long its body is before the body goes, so an upload of unknown length
// would have to be buffered whole — which is the one thing this package refuses to do.
type PutRequest struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
}

// Store is one object-storage service as this app reads it: the protocol, and nothing above it.
//
// Every method here is one request. Anything that is *several* — a walk, a search, a download of a
// folder, a resume, a multipart upload — is a function of this package written over these, so a
// second provider is a second `Store` and never a second downloader.
type Store interface {
	// Buckets is every bucket the credentials may list. A key that may read one bucket and not
	// list them answers `domain.ErrForbidden` here, which is why a profile may name its own.
	Buckets(ctx context.Context) ([]Bucket, error)
	List(ctx context.Context, request ListRequest) (Listing, error)
	// Head is the one object's own facts, including the content type, the uploader's metadata and
	// the service's whole answer, none of which a listing has a column for.
	Head(ctx context.Context, bucket, key string) (Object, error)

	// The three below are each **one request about one object**, and each is asked for only when a
	// reader opens the pane that shows it — never on the way past. That is the whole reason they are
	// three methods rather than fields of a `Head`: clicking a row must not cost four requests.
	//
	// A service that has not got one answers its own refusal, which reaches the reader as the
	// service's own words: an object store is a protocol several companies implement to different
	// depths, and inventing an empty answer would say "nobody may do anything" where the truth is
	// "this service does not keep that".
	ObjectPermissions(ctx context.Context, bucket, key string) (Permissions, error)
	ObjectTags(ctx context.Context, bucket, key string) ([]Tag, error)

	// WriteMetadata changes what an object's headers say, and it is the one write in this port that
	// is not what its name suggests: there is **no request that sets a header**. What there is, is a
	// copy of the object onto itself with the new headers, so the whole set is replaced at once — a
	// header left out of `meta` is a header the object no longer has.
	//
	// That is why it takes the whole shape rather than one pair, and why what calls it stages a set
	// and applies it in one press. It also **rewrites the object**: a new tag, a new modified time,
	// and a new version on a bucket that keeps them.
	WriteMetadata(ctx context.Context, bucket, key string, meta Metadata) error
	// ObjectVersions is bounded, for the reason every read here is: an object overwritten nightly for
	// three years has a thousand versions and a reader wants the last few.
	ObjectVersions(ctx context.Context, bucket, key string, limit int) ([]Version, error)
	// Open reads a range of one object. The caller closes it, and a body left open is a socket held
	// open, which is why every reader here is closed on the same line it stops being read.
	Open(ctx context.Context, bucket, key string, span Span) (io.ReadCloser, error)
	Put(ctx context.Context, request PutRequest) error

	// The four halves of an upload written in parts. They are on the port rather than behind a
	// `Put` that decides for itself, because *how many at once*, *which order* and *what to do
	// about one that failed* are the questions a big upload actually is — and they are the same
	// questions on every provider, so they are answered once in `upload.go`.
	CreateUpload(ctx context.Context, bucket, key, contentType string) (string, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, number int, body io.Reader, size int64) (Part, error)
	CompleteUpload(ctx context.Context, bucket, key, uploadID string, parts []Part) error
	AbortUpload(ctx context.Context, bucket, key, uploadID string) error

	// Delete removes up to `DeleteBatch` keys in one request. It answers with the keys it could not
	// remove, because a service reports a partial failure inside a successful response — a delete
	// of a thousand keys where one was refused is not an error, and pretending it is would leave
	// the reader unable to tell which.
	Delete(ctx context.Context, bucket string, keys []string) ([]string, error)
	// Copy writes one object to another key without the bytes coming here at all. It is what a
	// move, a rename and a folder copy are all made of, and it is bounded by the service's own
	// single-request limit — see `CopyLimit`.
	Copy(ctx context.Context, bucket, from, toBucket, to string) error

	MakeBucket(ctx context.Context, name string) error
	RemoveBucket(ctx context.Context, name string) error

	// Presign is a URL that reads this object without a credential, good for `life`. It is signed
	// here and goes nowhere: nothing about it is a request.
	Presign(ctx context.Context, bucket, key string, life time.Duration) (string, error)

	Close()
}

const (
	// DeleteBatch is how many keys one delete request carries. A thousand is the limit every S3
	// service publishes, and it is what makes emptying a folder of a million objects a thousand
	// requests rather than a million.
	DeleteBatch = 1000
	// CopyLimit is the largest object a single copy request will move — five gigabytes, which is
	// every provider's published bound. Past it a copy has to be a multipart one, and this app
	// answers `domain.Unsupported` naming it rather than starting something it cannot finish.
	CopyLimit = 5 << 30
	// PageSize is how many keys a listing asks for. It is the service's own maximum on purpose: a
	// page of a thousand is one round trip where a page of a hundred is ten, and the rows are cut
	// to what the grid shows on this side anyway.
	PageSize = 1000
	// VersionLimit is how many versions of one object are read. Twenty is what a reader looks at: an
	// object overwritten nightly for three years has a thousand of them, and a pane that read all of
	// them would be a pane nobody opened twice.
	VersionLimit = 20
)
