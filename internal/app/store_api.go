package app

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/profile"
	"github.com/khanhsnd/vibe-db/internal/secret"
	"github.com/khanhsnd/vibe-db/internal/session"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// listTimeout bounds a listing, a head and a delete — the requests that should be quick. A
	// listing that takes ten seconds is a service that is not going to answer.
	listTimeout = 20 * time.Second
	// folderTimeout bounds a change that walks a whole prefix. It is generous because a folder of a
	// million objects is a thousand requests, and it is bounded at all because the alternative is a
	// dialog nobody can get out of — StopObjectRun is the other way out.
	folderTimeout = 30 * time.Minute
	// previewCap is how much of an object a preview reads. It is a **byte range**: a preview of a
	// four-gigabyte log is the first half megabyte of it and never the log.
	previewCap = 512 << 10
	// pictureCap is the bound on the one kind of object that is drawn rather than read. It is larger
	// than a text preview because a picture cut in half is not a picture, and it is bounded because
	// it crosses the bridge as base64 — which is a third longer again.
	//
	// Eight megabytes, and the number is the whole of the decision: at two, the commonest picture in
	// any bucket — a photograph off a phone, a screenshot of a dashboard — was over the bound and a
	// reader was told to download it instead. That is a preview that does not preview the things
	// people actually look at. Eight covers those and still bounds the string that crosses the
	// bridge at about eleven megabytes.
	pictureCap = 8 << 20
	// presignLife is how long a signed link is good for. Fifteen minutes is what an RDS token gets
	// and for the same reason: long enough to be used, short enough that a link left in a chat
	// window is not a key.
	presignLife = 15 * time.Minute
	// reportEvery is how often the transfer panel is pushed a report while anything is moving. Twice
	// a second is what a progress bar needs and is far below what a byte counter changes at — the
	// alternative is an event per buffer, which is five hundred a second.
	reportEvery = 500 * time.Millisecond
	// journalShown is how many entries the Log pane draws. It is a window on what just happened, so
	// the last forty is what a reader reads.
	journalShown = 40
)

// StoreAPI is the object-store half of the app's boundary, and it is a **second bound API rather
// than more methods on `ConnectionAPI`** for the reason `domain.StoreProfile` is a second profile: a
// store has no databases, no namespaces, no statements and no rows, so nothing here shares a
// signature with anything there.
//
// It owns one thing `ConnectionAPI` does not: the transfer queue. A download outlives the tab it was
// started from — which is the first thing a reader finds out by closing one — so the queue belongs to
// the app and the panel reads it.
type StoreAPI struct {
	apiContext

	profiles profile.StoreRepository
	secrets  secret.Store
	sessions *session.StoreManager
	queue    *objectstore.Queue
	runs     *runRegistry
	// journal is what this app has done about the objects a reader has been looking at, which is what
	// the Log pane draws — see objectJournal, and note that it is not the service's access log.
	journal *objectJournal
	// watching is whether the report timer is already running. One timer serves every job there is,
	// so a second Submit joins it rather than starting a second — see watch.
	watching atomic.Bool
}

func NewStoreAPI(
	profiles profile.StoreRepository,
	secrets secret.Store,
	sessions *session.StoreManager,
) *StoreAPI {
	return &StoreAPI{
		apiContext: newAPIContext(),
		profiles:   profiles,
		secrets:    secrets,
		sessions:   sessions,
		queue:      objectstore.NewQueue(objectstore.Workers),
		runs:       newRunRegistry(),
		journal:    &objectJournal{},
	}
}

// ------------------------------------------------------------------ profiles

func (a *StoreAPI) ListStores() ([]Store, error) {
	stored, err := a.profiles.List()
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]Store, 0, len(stored))
	for _, held := range stored {
		out = append(out, a.storeDTO(held))
	}
	return out, nil
}

func (a *StoreAPI) SaveStore(input StoreInput) (Store, error) {
	target := input.toDomain()

	if target.ID == "" {
		id, err := newProfileID()
		if err != nil {
			return Store{}, wrapError(err)
		}
		target.ID = id
	}

	if err := target.Validate(); err != nil {
		return Store{}, wrapError(err)
	}
	if err := a.profiles.Save(target); err != nil {
		return Store{}, wrapError(err)
	}

	if err := a.keepSecret(target.ID, target.SaveSecret && target.Auth == domain.StoreKey, input.SecretKey); err != nil {
		return Store{}, wrapError(err)
	}
	if err := a.keepSecret(sshSecretKey(target.ID), target.SSH.Enabled, input.SSHSecret); err != nil {
		return Store{}, wrapError(err)
	}
	if err := a.keepSecret(proxySecretKey(target.ID), target.Proxy.Enabled, input.ProxySecret); err != nil {
		return Store{}, wrapError(err)
	}

	return a.storeDTO(target), nil
}

func (a *StoreAPI) DeleteStore(id string) error {
	if err := a.profiles.Delete(id); err != nil {
		return wrapError(err)
	}
	// All three entries go with the profile, exactly as a database connection's do.
	for _, key := range []string{id, sshSecretKey(id), proxySecretKey(id)} {
		if err := a.secrets.Delete(key); err != nil {
			return wrapError(err)
		}
	}
	return nil
}

// TestStore opens the connection and **asks for something**, which is the only honest test of a
// service whose every call is a request: nothing is dialled by opening one, so a profile that could
// not reach anything would otherwise pass.
//
// What it asks for is the buckets, or the named bucket's first page where a key may read one bucket
// and not list them — which is the ordinary arrangement in an organisation.
func (a *StoreAPI) TestStore(input StoreInput) (TestResult, error) {
	target := input.toDomain()
	if err := target.Validate(); err != nil {
		return TestResult{}, wrapError(err)
	}

	secrets, err := a.storeSecrets(target, input)
	if err != nil {
		return TestResult{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(connectTimeout + tunnelTimeout + proxyTimeout)
	defer cancel()

	started := time.Now()
	held, err := a.sessions.Open(ctx, target, secrets)
	if err != nil {
		return TestResult{}, wrapError(err)
	}
	defer a.sessions.Close(held.ID)

	if target.Bucket != "" {
		_, err = held.Store().List(ctx, objectstore.ListRequest{
			Bucket: target.Bucket, Delimiter: objectstore.Separator, Limit: 1,
		})
	} else {
		_, err = held.Store().Buckets(ctx)
	}
	if err != nil {
		return TestResult{}, wrapError(err)
	}

	return TestResult{
		ServerVersion: string(target.Provider),
		LatencyMs:     time.Since(started).Milliseconds(),
	}, nil
}

// ------------------------------------------------------------------ sessions

// ConnectStore opens a session. `secretKey` is what the reader typed just now, and blank falls back
// to what is stored — the rule a password follows.
func (a *StoreAPI) ConnectStore(id, secretKey string) (StoreConnection, error) {
	target, err := a.profiles.Get(id)
	if err != nil {
		return StoreConnection{}, wrapError(err)
	}

	secrets, err := a.storeSecrets(target, StoreInput{SecretKey: secretKey})
	if err != nil {
		return StoreConnection{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(connectTimeout + tunnelTimeout + proxyTimeout)
	defer cancel()

	held, err := a.sessions.Open(ctx, target, secrets)
	if err != nil {
		return StoreConnection{}, wrapError(err)
	}

	return StoreConnection{
		SessionID: held.ID,
		ProfileID: target.ID,
		Name:      target.Name,
		Provider:  string(target.Provider),
		Bucket:    target.Bucket,
		ReadOnly:  target.Access.ReadOnly(),
		Rules:     objectRules(target),
	}, nil
}

func (a *StoreAPI) DisconnectStore(sessionID string) error {
	return wrapError(a.sessions.Close(sessionID))
}

// ------------------------------------------------------------------ reading

func (a *StoreAPI) ListBuckets(sessionID string) ([]BucketRow, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	buckets, err := store.Buckets(ctx)
	if err != nil {
		return nil, wrapError(err)
	}

	rows := make([]BucketRow, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, BucketRow{
			Name:      bucket.Name,
			CreatedAt: momentText(bucket.Created),
			Region:    bucket.Region,
		})
	}
	return rows, nil
}

// ListObjects is one page of one folder, and it is **the** read of this feature: a folder at a time,
// with the folders inside it coming from the service itself rather than from anything this app
// worked out. That is what makes browsing a bucket of forty million objects cost one request — the
// alternative, which is what an index would need, is reading all forty million to draw a folder.
func (a *StoreAPI) ListObjects(sessionID string, input ListObjectsInput) (ObjectPage, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectPage{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	// A flat listing asks for no delimiter, which is what "everything under here" means to a
	// service: every key under the prefix and no folders at all.
	delimiter := objectstore.Separator
	if input.Flat {
		delimiter = ""
	}

	listing, err := store.List(ctx, objectstore.ListRequest{
		Bucket:    input.Bucket,
		Prefix:    input.Prefix,
		Delimiter: delimiter,
		Token:     input.Token,
		Limit:     input.Limit,
	})
	if err != nil {
		return ObjectPage{}, wrapError(err)
	}

	page := ObjectPage{
		Bucket: input.Bucket,
		Prefix: input.Prefix,
		Rows:   make([]ObjectRow, 0, len(listing.Folders)+len(listing.Objects)),
		Next:   listing.Next,
	}

	// Folders first, which is what every file manager there is does and what a reader walking a path
	// is looking for.
	for _, folder := range listing.Folders {
		page.Rows = append(page.Rows, folderRow(folder))
	}
	for _, object := range listing.Objects {
		// A zero-length key ending in the separator is a folder marker a console wrote. It is drawn
		// as the folder it stands for rather than as an object nobody meant to see.
		if strings.HasSuffix(object.Key, objectstore.Separator) && object.Size == 0 {
			continue
		}
		page.Rows = append(page.Rows, objectRow(object))
		page.Bytes += object.Size
	}

	page.Folders = len(listing.Folders)
	page.Objects = len(page.Rows) - page.Folders
	return page, nil
}

// HeadObject is the one object's own facts — what the **Properties** and **Header** panes are both
// drawn from, because they are two readings of one answer and asking twice would be one request too
// many.
func (a *StoreAPI) HeadObject(sessionID, bucket, key string) (ObjectFacts, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectFacts{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	object, err := store.Head(ctx, bucket, key)
	a.journal.note("head", bucket, key, started, wrapError(err))
	if err != nil {
		return ObjectFacts{}, wrapError(err)
	}

	facts := ObjectFacts{
		Detail: ObjectDetail{
			Key:          key,
			Size:         object.Size,
			ModifiedAt:   momentText(object.Modified),
			ETag:         object.ETag,
			StorageClass: object.StorageClass,
			ContentType:  object.ContentType,
			Verifiable:   plainChecksum(object.ETag),
		},
		Headers:  make([]ObjectHeader, 0, len(object.Headers)),
		Metadata: make([]ObjectHeader, 0, len(object.Metadata)),
		// Sent rather than known on the other side, so the pane and the request that writes them
		// cannot come to disagree about which six a service will take.
		Settable: objectstore.SettableHeaders,
	}
	for _, header := range object.Headers {
		facts.Headers = append(facts.Headers, ObjectHeader{Name: header.Name, Value: header.Value})
	}
	for name, value := range object.Metadata {
		facts.Metadata = append(facts.Metadata, ObjectHeader{Name: name, Value: value})
	}
	// A map has no order, so one is chosen — the pane draws a table and a table that reshuffled
	// itself between two reads of the same object would read as a change.
	sort.Slice(facts.Metadata, func(one, other int) bool {
		return facts.Metadata[one].Name < facts.Metadata[other].Name
	})
	return facts, nil
}

// SetObjectHeaders changes what an object's headers say, and answers with the object's facts **read
// again** — because what a reader has to see afterwards is what the service now says rather than what
// they typed: the request rewrites the object, so the tag and the modified time have both changed.
//
// It takes the whole set. There is no request that sets one header — see
// `objectstore.Store.WriteMetadata` — so a header left out of the input is a header the object no
// longer has, which is exactly why the pane stages a set and applies it in one press.
//
// The **storage class is carried through** rather than left out: the request underneath is a copy,
// and a copy naming no class moves the object to the service's default.
func (a *StoreAPI) SetObjectHeaders(
	sessionID, bucket, key string,
	input SetHeadersInput,
) (ObjectFacts, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectFacts{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(folderTimeout)
	defer cancel()

	started := time.Now()
	before, err := store.Head(ctx, bucket, key)
	if err != nil {
		a.journal.note("headers", bucket, key, started, wrapError(err))
		return ObjectFacts{}, wrapError(err)
	}

	// A copy is bounded by the protocol's own single-request limit, and an object past it cannot have
	// its headers changed this way at all. Saying so is better than a refusal from the service that
	// mentions a copy the reader never asked for.
	if before.Size > objectstore.CopyLimit {
		err := domain.Unsupported("changing the headers of an object larger than 5 GB")
		a.journal.note("headers", bucket, key, started, wrapError(err))
		return ObjectFacts{}, wrapError(err)
	}

	err = store.WriteMetadata(ctx, bucket, key, objectstore.Metadata{
		System:       input.System,
		User:         input.Meta,
		StorageClass: before.StorageClass,
	})
	a.journal.note("headers", bucket, key, started, wrapError(err))
	if err != nil {
		return ObjectFacts{}, wrapError(err)
	}

	return a.HeadObject(sessionID, bucket, key)
}

// ObjectPermissions is who may do what to one object. It is **one request, asked only when the pane
// is opened** — which is the rule the three panes below share: clicking a row must not cost four
// requests, so nothing here is folded into the listing or into the head.
func (a *StoreAPI) ObjectPermissions(sessionID, bucket, key string) (ObjectPermissions, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectPermissions{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	held, err := store.ObjectPermissions(ctx, bucket, key)
	a.journal.note("permissions", bucket, key, started, wrapError(err))
	if err != nil {
		return ObjectPermissions{}, wrapError(err)
	}

	out := ObjectPermissions{Owner: held.Owner, Grants: make([]ObjectPermission, 0, len(held.Grants))}
	for _, grant := range held.Grants {
		out.Grants = append(out.Grants, ObjectPermission{
			Grantee: grant.Grantee,
			Kind:    grant.Kind,
			Allow:   grant.Allow,
		})
		// The one thing the reader must not have to spot for themselves.
		if grant.Kind == "Group" && strings.EqualFold(grant.Grantee, "AllUsers") {
			out.Public = true
		}
	}
	return out, nil
}

func (a *StoreAPI) ObjectTags(sessionID, bucket, key string) ([]ObjectTag, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	held, err := store.ObjectTags(ctx, bucket, key)
	a.journal.note("tags", bucket, key, started, wrapError(err))
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]ObjectTag, 0, len(held))
	for _, tag := range held {
		out = append(out, ObjectTag{Name: tag.Name, Value: tag.Value})
	}
	return out, nil
}

func (a *StoreAPI) ObjectVersions(sessionID, bucket, key string) ([]ObjectVersion, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	held, err := store.ObjectVersions(ctx, bucket, key, objectstore.VersionLimit)
	a.journal.note("versions", bucket, key, started, wrapError(err))
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]ObjectVersion, 0, len(held))
	for _, one := range held {
		out = append(out, ObjectVersion{
			VersionID:    one.VersionID,
			Latest:       one.Latest,
			Deleted:      one.Deleted,
			Size:         one.Size,
			ModifiedAt:   momentText(one.Modified),
			ETag:         one.ETag,
			StorageClass: one.StorageClass,
		})
	}
	return out, nil
}

// ObjectLog is what **this app** did about one object, newest first. It asks the service nothing at
// all: it is a window on the bound methods, which is the only honest reading of a `Log` pane in a
// client — see `objectJournal`.
func (a *StoreAPI) ObjectLog(bucket, key string) []ObjectLogEntry {
	held := a.journal.forKey(bucket, key, journalShown)

	out := make([]ObjectLogEntry, 0, len(held))
	for _, entry := range held {
		out = append(out, ObjectLogEntry{
			At:      momentText(entry.When),
			Action:  entry.Action,
			Key:     entry.Key,
			Outcome: entry.Outcome,
			Detail:  entry.Detail,
			MS:      entry.Elapsed.Milliseconds(),
		})
	}
	return out
}

// PreviewObject reads as much of an object as is worth putting on a screen, and no more. It is a
// **ranged read** — the one place in this app where reading part of something is the whole point —
// so opening a forty-gigabyte log to look at its first lines costs half a megabyte.
func (a *StoreAPI) PreviewObject(sessionID, bucket, key string) (ObjectPreview, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectPreview{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	object, err := store.Head(ctx, bucket, key)
	if err != nil {
		a.journal.note("preview", bucket, key, started, wrapError(err))
		return ObjectPreview{}, wrapError(err)
	}

	kind := drawnAs(object.ContentType, key)
	picture := strings.HasPrefix(kind, "image/")
	cap := int64(previewCap)
	if picture {
		cap = pictureCap
	}

	preview := ObjectPreview{
		Key: key,
		// The type the preview was **drawn as**, which is not always the one the service reported —
		// see drawnAs. Properties shows the service's own, which is the truth about the object.
		ContentType: kind,
		Size:        object.Size,
		Truncated:   object.Size > cap,
	}

	// A picture larger than the bound is not shown at all rather than shown cut in half: half a JPEG
	// is a grey box, and a reader would take it for a broken object.
	if picture && preview.Truncated {
		a.journal.note("preview", bucket, key, started, nil)
		return preview, nil
	}

	want := object.Size
	if want > cap {
		want = cap
	}
	if want <= 0 {
		a.journal.note("preview", bucket, key, started, nil)
		return preview, nil
	}

	// The whole object where it fits, and a range only where it does not. A range that covers
	// everything is a request some services answer 200 to rather than 206, and asking for the plain
	// thing is both simpler and one less way to be refused — see `s3.Store.Open`.
	span := objectstore.Span{Length: want}
	if want >= object.Size {
		span = objectstore.Span{}
	}

	body, err := store.Open(ctx, bucket, key, span)
	if err != nil {
		a.journal.note("preview", bucket, key, started, wrapError(err))
		return ObjectPreview{}, wrapError(err)
	}
	defer body.Close()

	read, err := io.ReadAll(io.LimitReader(body, want))
	a.journal.note("preview", bucket, key, started, wrapError(err))
	if err != nil {
		return ObjectPreview{}, wrapError(err)
	}

	if picture {
		preview.Data = "data:" + object.ContentType + ";base64," + base64.StdEncoding.EncodeToString(read)
		return preview, nil
	}
	preview.Text = string(read)
	return preview, nil
}

// SearchObjects looks for a name across a bucket. It is bounded and says so — see
// `ObjectSearchResult` — and it has no deadline of its own: how long a bucket takes is the bucket's
// business, so `StopObjectRun` is the honest bound, which is the arrangement a read of every row and
// a Find in Schema both have.
func (a *StoreAPI) SearchObjects(sessionID string, input SearchObjectsInput) (ObjectSearchResult, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return ObjectSearchResult{}, wrapError(err)
	}

	since, err := moment(input.Since)
	if err != nil {
		return ObjectSearchResult{}, wrapError(err)
	}
	until, err := moment(input.Until)
	if err != nil {
		return ObjectSearchResult{}, wrapError(err)
	}

	ctx, release := a.runs.begin(a.ctx, input.RunID, noDeadline)
	defer release()

	answer, err := objectstore.Search(ctx, store, objectstore.SearchRequest{
		Bucket: input.Bucket,
		Prefix: input.Prefix,
		Match: domain.TextMatch{
			Text:            input.Text,
			Mode:            domain.SearchMode(input.Mode),
			CaseInsensitive: input.CaseInsensitive,
		},
		MinSize: input.MinSize,
		MaxSize: input.MaxSize,
		Since:   since,
		Until:   until,
		Hits:    input.Hits,
		Scan:    input.Scan,
	})
	if err != nil {
		return ObjectSearchResult{}, wrapError(err)
	}

	result := ObjectSearchResult{
		Rows:    make([]ObjectRow, 0, len(answer.Hits)),
		Scanned: answer.Scanned,
		Prefix:  answer.Prefix,
		Whole:   answer.Whole,
		More:    answer.Full,
	}
	for _, hit := range answer.Hits {
		row := objectRow(hit)
		// A hit is drawn by its **whole key**, not by its tail: a search answers across folders, and
		// twelve rows all called `index.json` would say nothing about where any of them is.
		row.Name = hit.Key
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

// StopObjectRun ends a search or a folder-wide change that is still going. It is the same registry a
// query's Stop uses, and the id is minted by the caller for the same reason: the call it stops has
// not returned yet.
func (a *StoreAPI) StopObjectRun(runID string) {
	a.runs.cancel(runID)
}

// PresignObject is a link that reads one object without a credential. **Nothing is sent** — the
// signature goes in the query — so this is the one method here that reaches no service at all.
func (a *StoreAPI) PresignObject(sessionID, bucket, key string, seconds int) (string, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	life := presignLife
	if seconds > 0 {
		life = time.Duration(seconds) * time.Second
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	started := time.Now()
	link, err := store.Presign(ctx, bucket, key, life)
	// It is recorded even though nothing was sent, and that is the point: a link handed to somebody
	// else is the one action here whose consequence outlives the app, so the Log pane has to say it
	// happened.
	a.journal.note("link", bucket, key, started, wrapError(err))
	return link, wrapError(err)
}

// ------------------------------------------------------------------ changing

func (a *StoreAPI) DeleteObjects(sessionID, bucket string, keys []string) (RemoveResult, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return RemoveResult{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(folderTimeout)
	defer cancel()

	started := time.Now()
	removed, err := objectstore.RemoveKeys(ctx, store, bucket, keys, nil)
	for _, key := range keys {
		a.journal.note("delete", bucket, key, started, wrapError(err))
	}
	if err != nil {
		return RemoveResult{}, wrapError(err)
	}
	return RemoveResult{Deleted: removed.Deleted, Refused: removed.Refused}, nil
}

// DeleteFolder removes everything under a prefix. It names **nothing** in particular, which is why
// `domain.ObjectHazard` calls it unfiltered and the dialog asks on a laptop as well as in production.
//
// An empty prefix is emptying the bucket, and that is deliberate rather than an accident of the
// arithmetic: it is the same walk and the same batches, and giving it a method of its own would be a
// second way to do one thing.
func (a *StoreAPI) DeleteFolder(sessionID, bucket, prefix, runID string) (RemoveResult, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return RemoveResult{}, wrapError(err)
	}

	ctx, release := a.runs.begin(a.ctx, runID, folderTimeout)
	defer release()

	started := time.Now()
	removed, err := objectstore.RemovePrefix(ctx, store, bucket, prefix, nil)
	// The prefix itself is what is recorded, and `forKey` is what makes it show up under every object
	// that was inside it: a row whose history says "the folder above it was deleted" is the answer a
	// reader is looking for.
	a.journal.note("delete folder", bucket, prefix, started, wrapError(err))
	if err != nil {
		return RemoveResult{Deleted: removed.Deleted, Refused: removed.Refused}, wrapError(err)
	}
	return RemoveResult{Deleted: removed.Deleted, Refused: removed.Refused}, nil
}

// CopyObjects copies a folder without a byte coming here: a copy is one request naming both keys, so
// a hundred gigabytes moves at the service's own speed and over none of the reader's bandwidth.
func (a *StoreAPI) CopyObjects(sessionID, bucket, from, toBucket, to, runID string) (CopyResult, error) {
	store, err := a.store(sessionID)
	if err != nil {
		return CopyResult{}, wrapError(err)
	}

	ctx, release := a.runs.begin(a.ctx, runID, folderTimeout)
	defer release()

	// A folder is copied whole and an object is copied on its own, and which of the two this is is
	// the key's own last character. It is the one place that distinction is made, and it is made
	// here rather than in two bound methods because the reader ticked rows without minding.
	if strings.HasSuffix(from, objectstore.Separator) {
		copied, err := objectstore.CopyPrefix(ctx, store, bucket, from, toBucket, to, nil)
		return CopyResult{Objects: copied.Objects, Bytes: copied.Bytes}, wrapError(err)
	}

	if err := store.Copy(ctx, bucket, from, toBucket, to); err != nil {
		return CopyResult{}, wrapError(err)
	}
	return CopyResult{Objects: 1}, nil
}

// MoveObject is a copy and then a delete, in that order, and the order is the whole of it: a delete
// that went first would be a delete of the only copy.
func (a *StoreAPI) MoveObject(sessionID, bucket, from, to string) error {
	store, err := a.store(sessionID)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(folderTimeout)
	defer cancel()

	started := time.Now()
	if err := store.Copy(ctx, bucket, from, bucket, to); err != nil {
		a.journal.note("move", bucket, from, started, wrapError(err))
		return wrapError(err)
	}
	if _, err := store.Delete(ctx, bucket, []string{from}); err != nil {
		a.journal.note("move", bucket, from, started, wrapError(err))
		return wrapError(err)
	}

	// Both ends are recorded, because a reader looking at either wants to know where it went and
	// where it came from.
	a.journal.note("moved out", bucket, from, started, nil)
	a.journal.note("moved in", bucket, to, started, nil)
	return nil
}

// NewFolder writes the zero-length object every console writes to make a folder appear. A bucket has
// no folders — a prefix is a reading of the keys in it — so this is the one way to have an empty one
// at all, and it says so rather than pretending a folder was created.
func (a *StoreAPI) NewFolder(sessionID, bucket, prefix, name string) error {
	store, err := a.store(sessionID)
	if err != nil {
		return wrapError(err)
	}

	clean := strings.Trim(strings.TrimSpace(name), objectstore.Separator)
	if clean == "" {
		return wrapError(domain.NameError{What: "folder"})
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	key := objectstore.JoinKey(prefix, clean) + objectstore.Separator
	return wrapError(store.Put(ctx, objectstore.PutRequest{Bucket: bucket, Key: key, Size: 0}))
}

func (a *StoreAPI) MakeBucket(sessionID, name string) error {
	store, err := a.store(sessionID)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	return wrapError(store.MakeBucket(ctx, name))
}

func (a *StoreAPI) RemoveBucket(sessionID, name string) error {
	store, err := a.store(sessionID)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(listTimeout)
	defer cancel()

	return wrapError(store.RemoveBucket(ctx, name))
}

// ------------------------------------------------------------------ transfers

// DownloadObjects puts a job on the queue and answers **at once** with its id. Nothing about the
// bytes crosses the bridge: the reader watches a number.
//
// A folder in the list becomes a *walk*, streamed into the queue as it is read, so a folder of a
// million objects starts moving bytes before its listing has finished — and costs a page of names
// rather than a million.
func (a *StoreAPI) DownloadObjects(sessionID string, input DownloadInput) (string, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", wrapError(err)
	}
	if strings.TrimSpace(input.Into) == "" {
		return "", wrapError(domain.NameError{What: "folder to download into"})
	}

	store := held.Store()
	jobID, err := newProfileID()
	if err != nil {
		return "", wrapError(err)
	}

	a.queue.Submit(a.ctx, store, objectstore.Job{
		ID:        jobID,
		Kind:      objectstore.KindDownload,
		Bucket:    input.Bucket,
		Label:     downloadLabel(input),
		Root:      input.Into,
		Collision: objectstore.Collision(input.Collision),
		Each: func(ctx context.Context, yield func(objectstore.Item) error) error {
			for _, object := range input.Keys {
				path, ok := objectstore.LocalPath(input.Into, input.Under, object.Key)
				if !ok {
					continue
				}
				// The three facts come off the listing the reader ticked in — see DownloadObject.
				// Without them the job's total would count only the folders, and every ticked
				// object would cost a `Head` before its first byte.
				when, _ := moment(object.ModifiedAt)
				if err := yield(objectstore.Item{
					Key:      object.Key,
					Path:     path,
					Size:     object.Size,
					ETag:     object.ETag,
					Modified: when.Unix(),
				}); err != nil {
					return err
				}
			}

			for _, prefix := range input.Prefixes {
				base := objectstore.ParentPrefix(prefix)
				_, err := objectstore.Walk(ctx, store, objectstore.WalkRequest{
					Bucket: input.Bucket,
					Prefix: prefix,
				}, func(object objectstore.Object) error {
					path, ok := objectstore.LocalPath(input.Into, base, object.Key)
					if !ok {
						return nil
					}
					return yield(objectstore.Item{
						Key:      object.Key,
						Path:     path,
						Size:     object.Size,
						ETag:     object.ETag,
						Modified: object.Modified.Unix(),
					})
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	})

	// One entry per thing the reader asked for rather than one per file, because the *ask* is what a
	// Log pane is about: forty thousand entries for one folder download would bury everything else.
	started := time.Now()
	for _, object := range input.Keys {
		a.journal.note("download", input.Bucket, object.Key, started, nil)
	}
	for _, prefix := range input.Prefixes {
		a.journal.note("download folder", input.Bucket, prefix, started, nil)
	}

	a.watch()
	return jobID, nil
}

// UploadFiles is the other direction. A folder among the paths is walked on **this** machine, which
// is the same bargain the download makes: the listing streams into the queue.
func (a *StoreAPI) UploadFiles(sessionID string, input UploadInput) (string, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	jobID, err := newProfileID()
	if err != nil {
		return "", wrapError(err)
	}

	a.queue.Submit(a.ctx, held.Store(), objectstore.Job{
		ID:     jobID,
		Kind:   objectstore.KindUpload,
		Bucket: input.Bucket,
		Label:  uploadLabel(input),
		Root:   input.Prefix,
		Each: func(_ context.Context, yield func(objectstore.Item) error) error {
			for _, path := range input.Paths {
				info, err := os.Stat(path)
				if err != nil {
					return domain.FileError{Err: err}
				}

				if !info.IsDir() {
					key := objectstore.JoinKey(input.Prefix, filepath.Base(path))
					if err := yield(objectstore.Item{Key: key, Path: path, Size: info.Size()}); err != nil {
						return err
					}
					continue
				}

				// A folder keeps its own name at the far end, which is what a reader dragging one in
				// means: `D:\logs` becomes `<prefix>logs/…`.
				under := objectstore.JoinKey(input.Prefix, filepath.Base(path))
				if err := walkFiles(path, under, yield); err != nil {
					return err
				}
			}
			return nil
		},
	})

	a.watch()
	return jobID, nil
}

// walkFiles is the local half of an upload's listing, and it streams for the reason the remote half
// does: a folder of a million files must not become a million items before the first byte moves.
func walkFiles(root, under string, yield func(objectstore.Item) error) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return domain.FileError{Err: err}
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return domain.FileError{Err: err}
		}

		info, err := entry.Info()
		if err != nil {
			return domain.FileError{Err: err}
		}

		key := objectstore.JoinKey(under, filepath.ToSlash(relative))
		return yield(objectstore.Item{Key: key, Path: path, Size: info.Size()})
	})
}

// Transfers is the queue as the panel draws it. The panel is *also* pushed this on a timer while
// anything is moving — see watch — so this is what a panel just opened reads to catch up.
func (a *StoreAPI) Transfers() TransferReport {
	return transferReport(a.queue.Report())
}

// CancelTransfer stops one job. What is already written stays written: a download of forty files
// stopped at the thirtieth is thirty files, and a large file part way through keeps its partial, so
// asking for the same job again carries on from there.
func (a *StoreAPI) CancelTransfer(jobID string) {
	a.queue.Cancel(jobID)
}

func (a *StoreAPI) CancelTransfers() {
	a.queue.CancelAll()
}

func (a *StoreAPI) ClearTransfers() {
	a.queue.Clear()
}

// RevealFolder shows a folder in the host's own file manager, which is where a reader goes after a
// download. It is `OpenBackupFolder`'s twin and shares its one implementation.
func (a *StoreAPI) RevealFolder(path string) error {
	return wrapError(openFolder(path))
}

// watch pushes the transfer report while anything is moving, and stops when nothing is. It is
// **pushed rather than polled** for the reason a scheduled job's firing is: a reader watching a
// forty-gigabyte download deserves a number that moves, and a panel that only changed when it was
// re-read would be a panel that lied between reads.
//
// One goroutine at a time, however many jobs there are: `watching` is what makes a second Submit
// join the one already running rather than start a second timer.
func (a *StoreAPI) watch() {
	if !a.watching.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer a.watching.Store(false)

		// One straight away, and then on the clock. Without it a job that started and finished
		// inside the first half second would push **nothing at all**, and the panel would sit
		// looking at a queue that had already emptied — which a live run is how anybody finds out.
		a.push()

		ticker := time.NewTicker(reportEvery)
		defer ticker.Stop()

		for range ticker.C {
			if a.push() == 0 {
				return
			}
		}
	}()
}

// push sends the report and answers with how many jobs are still going, which is what decides
// whether the clock keeps running.
func (a *StoreAPI) push() int {
	report := a.queue.Report()
	wailsruntime.EventsEmit(a.ctx, EventTransfers, transferReport(report))
	return report.Active
}

func transferReport(report objectstore.Report) TransferReport {
	out := TransferReport{
		Jobs:   make([]TransferJob, 0, len(report.Jobs)),
		Files:  make([]TransferFile, 0, len(report.Running)),
		Active: report.Active,
	}

	for _, job := range report.Jobs {
		out.Jobs = append(out.Jobs, TransferJob{
			ID:        job.ID,
			Kind:      string(job.Kind),
			Bucket:    job.Bucket,
			Label:     job.Label,
			Root:      job.Root,
			State:     string(job.State),
			Files:     job.Files,
			Done:      job.Done,
			Failed:    job.Failed,
			Skipped:   job.Skipped,
			Bytes:     job.Bytes,
			Moved:     job.Moved,
			Counting:  job.Counting,
			Rate:      job.Rate,
			Detail:    job.Detail,
			StartedAt: momentText(job.Started),
			EndedAt:   momentText(job.Ended),
		})
	}
	for _, file := range report.Running {
		out.Files = append(out.Files, TransferFile{
			JobID: file.JobID,
			Key:   file.Key,
			Path:  file.Path,
			Size:  file.Size,
			Moved: file.Moved,
		})
	}
	return out
}

// downloadLabel is what the panel calls the job: what the reader asked for, in their words.
func downloadLabel(input DownloadInput) string {
	switch {
	case len(input.Keys) == 1 && len(input.Prefixes) == 0:
		return objectstore.LastSegment(input.Keys[0].Key)
	case len(input.Prefixes) == 1 && len(input.Keys) == 0:
		return input.Prefixes[0]
	default:
		return countedLabel(len(input.Keys)+len(input.Prefixes), "item")
	}
}

func uploadLabel(input UploadInput) string {
	if len(input.Paths) == 1 {
		return filepath.Base(input.Paths[0])
	}
	return countedLabel(len(input.Paths), "file")
}

func countedLabel(count int, noun string) string {
	text := noun
	if count != 1 {
		text += "s"
	}
	return strconv.Itoa(count) + " " + text
}

// ------------------------------------------------------------------ plumbing

// store is the guarded store of an open session. What comes back is already the read-only wrapper on
// a read-only profile, which is why nothing in this file checks access.
func (a *StoreAPI) store(sessionID string) (objectstore.Store, error) {
	held, err := a.sessions.Get(sessionID)
	if err != nil {
		return nil, err
	}
	return held.Store(), nil
}

func (a *StoreAPI) storeDTO(held domain.StoreProfile) Store {
	return storeDTO(
		held,
		a.storedAny(held.ID),
		a.storedAny(sshSecretKey(held.ID)),
		a.storedAny(proxySecretKey(held.ID)),
	)
}

// storedAny says whether the credential store holds an entry under this key. It answers false for
// anything that went wrong, which is the honest reading: a dialog that cannot tell asks.
func (a *StoreAPI) storedAny(key string) bool {
	held, err := a.secrets.Get(key)
	return err == nil && held != ""
}

// keepSecret writes one entry, or takes it away where the reader asked for it not to be kept. A
// blank secret with the box ticked leaves whatever is stored alone — the rule a blank password
// follows, and what lets a reader edit a profile without retyping a key.
func (a *StoreAPI) keepSecret(key string, keep bool, value string) error {
	if !keep {
		return a.secrets.Delete(key)
	}
	if value == "" {
		return nil
	}
	return a.secrets.Set(key, value)
}

// storeSecrets is what a connection is opened with: whatever was typed just now, falling back to
// what is stored. A profile signing in with the machine's own AWS identity reads neither — the
// credentials are the environment's and are read at the moment of connecting.
func (a *StoreAPI) storeSecrets(
	target domain.StoreProfile,
	input StoreInput,
) (session.StoreSecrets, error) {
	secrets := session.StoreSecrets{
		SecretKey: input.SecretKey,
		SSH:       input.SSHSecret,
		Proxy:     input.ProxySecret,
	}

	if secrets.SecretKey == "" && target.Auth == domain.StoreKey {
		secrets.SecretKey = a.stored(target.ID)
	}
	if secrets.SSH == "" && target.SSH.Enabled {
		secrets.SSH = a.stored(sshSecretKey(target.ID))
	}
	if secrets.Proxy == "" && target.Proxy.Enabled {
		secrets.Proxy = a.stored(proxySecretKey(target.ID))
	}
	return secrets, nil
}

// stored reads one entry, answering blank where nothing has been kept. A missing entry is not a
// failure: a profile whose key was never saved is what the prompt is for.
func (a *StoreAPI) stored(key string) string {
	held, err := a.secrets.Get(key)
	if err != nil && !errors.Is(err, domain.ErrPasswordNotFound) {
		return ""
	}
	return held
}

// moment reads a bound off the wire. An empty string is no bound rather than the zero time, which is
// the difference between "written since" and "written since the beginning of time".
func moment(text string) (time.Time, error) {
	if strings.TrimSpace(text) == "" {
		return time.Time{}, nil
	}
	when, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, domain.ValidationError{Field: "since", Message: "is not a date this app can read"}
	}
	return when, nil
}

// generic is a content type that says nothing: the two spellings of "some bytes" that every service
// falls back to when whatever uploaded the object never said what it was.
var generic = map[string]bool{
	"":                         true,
	"application/octet-stream": true,
	"binary/octet-stream":      true,
}

// drawnAs is the type a preview is **drawn** as: the service's own where it said something, and the
// name's own where it did not.
//
// The panel used to trust the service alone, and the note above `ObjectPreview` said so proudly —
// *nothing here guesses from an extension*. That was too strict, and a picture that would not draw is
// how it showed: most objects in most buckets were uploaded by something that never set a type, so a
// `.png` arrives as `application/octet-stream` and was drawn as text. Falling back to the name is not
// guessing over the service — it is the only other fact there is, used exactly when the service said
// nothing.
//
// Properties still shows what the service **said**, because that is the truth about the object and it
// is what a reader would go and fix.
func drawnAs(reported, key string) string {
	if !generic[strings.ToLower(strings.TrimSpace(reported))] {
		return reported
	}
	return objectstore.ContentTypeOf(key)
}

// plainChecksum says the service's tag is an MD5 of the object's bytes, which is true of an object
// written in one request and false of one written in parts. The detail pane says so out loud because
// a download reports the same fact.
func plainChecksum(etag string) bool {
	tag := strings.Trim(etag, `"`)
	if len(tag) != 32 || strings.Contains(tag, "-") {
		return false
	}
	for _, character := range tag {
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}

// close stops the clock and everything still moving, and is what the app's own shutdown calls. It is
// **lower case on purpose**: Wails binds every exported method, so a `Close` here would appear in the
// frontend's own API — and stopping every transfer in the app is not a button anybody should find.
func (a *StoreAPI) close() {
	a.queue.Close()
	a.sessions.CloseAll()
}
