package app

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
)

// The wire types for an object store. They sit beside `dto.go`'s rather than in it because they are
// a different kind of connection entirely: there is no engine, no namespace and nothing that is a
// row, so a reader of `dto.go` should not have to hold both in their head at once.

// Store is an object-store connection as the sidebar draws one. It is `Profile`'s counterpart, and
// what it shares with one is exactly what a reader shares between the two: a name, what it is for,
// its colour, what it may do, and the hops that carry it.
//
// **No secret is on it.** `HasSecret` says one is stored, never what it is — the rule `HasPassword`
// follows. The access key **id** is here, because it identifies a key and cannot sign anything.
type Store struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	Colour      string `json:"colour"`
	Access      string `json:"access"`
	Endpoint    string `json:"endpoint"`
	Region      string `json:"region"`
	Bucket      string `json:"bucket"`
	PathStyle   bool   `json:"pathStyle"`
	Auth        string `json:"auth"`
	AccessKeyID string `json:"accessKeyId"`

	TLS   TLSSettings   `json:"tls"`
	SSH   SSHSettings   `json:"ssh"`
	Proxy ProxySettings `json:"proxy"`

	SaveSecret     bool `json:"saveSecret"`
	HasSecret      bool `json:"hasSecret"`
	HasSSHSecret   bool `json:"hasSshSecret"`
	HasProxySecret bool `json:"hasProxySecret"`

	// Target is where this connection goes, as the status bar says it. It is spelled by `domain` so
	// that the bar and the tree cannot word it differently.
	Target string `json:"target"`
}

// StoreInput is the dialog's answer. A blank `SecretKey` means "keep whatever is stored", which is
// the rule a blank `Password` follows.
type StoreInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	Colour      string `json:"colour"`
	Access      string `json:"access"`
	Endpoint    string `json:"endpoint"`
	Region      string `json:"region"`
	Bucket      string `json:"bucket"`
	PathStyle   bool   `json:"pathStyle"`
	Auth        string `json:"auth"`
	AccessKeyID string `json:"accessKeyId"`
	SecretKey   string `json:"secretKey"`

	TLS         TLSSettings   `json:"tls"`
	SSH         SSHSettings   `json:"ssh"`
	Proxy       ProxySettings `json:"proxy"`
	SSHSecret   string        `json:"sshSecret"`
	ProxySecret string        `json:"proxySecret"`

	SaveSecret bool `json:"saveSecret"`
}

// StoreConnection is one open store session, and it carries the **whole** of what the UI has to know
// about what this connection may do — see `ObjectRules`. That matrix is computed once, here, because
// it is a pure function of the profile: eight actions, one hazard each, one environment.
type StoreConnection struct {
	SessionID string `json:"sessionId"`
	ProfileID string `json:"profileId"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	// Bucket is the one the profile opens on, and empty means every bucket the key can list.
	Bucket string `json:"bucket"`
	// ReadOnly is the connection's own fact rather than its provider's, which is why it travels on
	// the session and not on a capability list — the reason `readOnly` does for a database.
	ReadOnly bool        `json:"readOnly"`
	Rules    ObjectRules `json:"rules"`
}

// ObjectRules is what each thing a reader can do to a bucket costs, answered once per session so
// that no component ever re-derives it. Each entry says two things and they are different questions:
// `confirm` is `Environment.NeedsConfirmation` — are you sure — and `refused` is `domain.Access` —
// not at all.
//
// It is a struct of named fields rather than a map so that the frontend's type says which actions
// exist: an action nobody spelled here is an action no dialog can ask about.
type ObjectRules struct {
	Upload       ObjectRule `json:"upload"`
	Delete       ObjectRule `json:"delete"`
	DeleteFolder ObjectRule `json:"deleteFolder"`
	EmptyBucket  ObjectRule `json:"emptyBucket"`
	DeleteBucket ObjectRule `json:"deleteBucket"`
	Copy         ObjectRule `json:"copy"`
	Move         ObjectRule `json:"move"`
	CreateBucket ObjectRule `json:"createBucket"`
	// Headers is changing what an object's headers say, which is a **write** because the request
	// underneath rewrites the object — see `domain.ObjectHeaders`.
	Headers ObjectRule `json:"headers"`
}

// ObjectRule is one action's answer: what it is called, how dangerous it is, whether it has to be
// answered for, and whether it will be sent at all.
type ObjectRule struct {
	Verb    string `json:"verb"`
	Hazard  string `json:"hazard"`
	Confirm bool   `json:"confirm"`
	Refused bool   `json:"refused"`
}

// objectRules answers the matrix for one profile.
func objectRules(profile domain.StoreProfile) ObjectRules {
	rule := func(action domain.ObjectAction) ObjectRule {
		hazard := domain.ObjectHazard(action)
		return ObjectRule{
			Verb:    domain.ObjectVerb(action),
			Hazard:  string(hazard),
			Confirm: profile.Environment.NeedsConfirmation(hazard),
			Refused: profile.Access.Refuses(hazard, ""),
		}
	}

	return ObjectRules{
		Upload:       rule(domain.ObjectUpload),
		Delete:       rule(domain.ObjectDelete),
		DeleteFolder: rule(domain.ObjectDeletePrefix),
		EmptyBucket:  rule(domain.ObjectEmptyBucket),
		DeleteBucket: rule(domain.ObjectDeleteBucket),
		Copy:         rule(domain.ObjectCopy),
		Move:         rule(domain.ObjectMove),
		CreateBucket: rule(domain.ObjectCreateBucket),
		Headers:      rule(domain.ObjectHeaders),
	}
}

// BucketRow is one bucket, as the tree draws it.
type BucketRow struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Region    string `json:"region"`
}

// ListObjectsInput is one page of one folder. `Token` is the service's own continuation token and is
// opaque on both sides of the bridge: a page is forward only, so it only ever reads **onward** — the
// tab appends what comes back as its grid is scrolled, and there is nothing to go back to.
type ListObjectsInput struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
	Token  string `json:"token"`
	Limit  int    `json:"limit"`
	// Flat asks for everything under the prefix rather than the folder alone, which is what the
	// grid's own Flatten does — the same word a key space uses for the same idea.
	Flat bool `json:"flat"`
}

// ObjectRow is one row of the grid. A folder is a row too, and `Folder` is what tells them apart:
// they are one list because they are drawn as one, and sorting them into folders-first is the grid's.
type ObjectRow struct {
	// Key is the whole key, which is what every action is addressed to. It is not what the grid
	// draws — that is `Name` — because a row in a folder is named by its tail.
	Key    string `json:"key"`
	Name   string `json:"name"`
	Folder bool   `json:"folder"`
	Size   int64  `json:"size"`
	// ModifiedAt is RFC 3339, because how a time reads is the reader's locale and belongs on the
	// other side of the bridge — the rule `Backup.ModifiedAt` follows.
	ModifiedAt   string `json:"modifiedAt"`
	ETag         string `json:"etag"`
	StorageClass string `json:"storageClass"`
}

// ObjectPage is one page of a folder: the rows, and whether there is more.
type ObjectPage struct {
	Bucket string      `json:"bucket"`
	Prefix string      `json:"prefix"`
	Rows   []ObjectRow `json:"rows"`
	// Next continues the listing, and empty means this is the last page.
	Next string `json:"next"`
	// Folders and Objects are how many of each the page holds, so the status line can say what it
	// is showing without the grid counting rows it has already been given.
	Folders int `json:"folders"`
	Objects int `json:"objects"`
	// Bytes is what the objects on **this page** come to, and it is deliberately not the folder's
	// total: a folder has no size a listing knows, and claiming one would be a number that changed
	// every time the reader turned a page.
	Bytes int64 `json:"bytes"`
}

// ObjectDetail is one object's own facts, which is a `Head` and therefore a request of its own — a
// listing has no column for a content type.
type ObjectDetail struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	ModifiedAt   string `json:"modifiedAt"`
	ETag         string `json:"etag"`
	StorageClass string `json:"storageClass"`
	ContentType  string `json:"contentType"`
	// Verifiable says the service's tag for this object is a plain checksum of its bytes, which is
	// true of an object written in one request and false of one written in parts. It is said out loud
	// because a download reports the same fact, and a reader should be able to see why.
	Verifiable bool `json:"verifiable"`
}

// ObjectHeader is one line of what the service said about an object. Both halves travel verbatim:
// this is the one place in the app where what is drawn is literally what came off the wire.
type ObjectHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ObjectFacts is everything a `Head` answers with, which is what the **Header** and **Properties**
// panes are drawn from. One request feeds both: they are two readings of one answer — the protocol's
// own lines, and the handful of them that mean something in particular.
type ObjectFacts struct {
	Detail ObjectDetail `json:"detail"`
	// Headers is the whole answer, sorted by name — see `objectstore.Object.Headers` for why the
	// order the service sent them in is not kept.
	Headers []ObjectHeader `json:"headers"`
	// Metadata is what the *uploader* attached, with the `x-amz-meta-` prefix taken off.
	Metadata []ObjectHeader `json:"metadata"`
	// Settable is the header names a service will actually take — six, fixed by the protocol. It is
	// sent rather than known on the other side, so a pane cannot come to disagree with the request
	// that writes them: a row not named here is the **service's own** and there is no request that
	// changes it.
	Settable []string `json:"settable"`
}

// SetHeadersInput is the whole set an object's headers are to become. It is a **set and not an
// edit**, because the request underneath is a copy of the object onto itself with `REPLACE`: a
// header left out of this is a header the object no longer has.
//
// That is why the pane stages what a reader types and applies it in one press, and why an Apply says
// out loud that it rewrites the object.
type SetHeadersInput struct {
	// System is the settable six, keyed by their HTTP names. Anything else is dropped rather than
	// sent and ignored.
	System map[string]string `json:"system"`
	// Meta is the `x-amz-meta-` pairs, without the prefix.
	Meta map[string]string `json:"meta"`
}

// ObjectPermission is one grant, in the service's own words for all three parts.
type ObjectPermission struct {
	Grantee string `json:"grantee"`
	Kind    string `json:"kind"`
	Allow   string `json:"allow"`
}

// ObjectPermissions is an object's access-control policy, **read and never written** — see
// `objectstore.Permissions`. A service that has not got them answers its own refusal, which reaches
// the pane as an error rather than as an empty table that would read as "nobody may do anything".
type ObjectPermissions struct {
	Owner  string             `json:"owner"`
	Grants []ObjectPermission `json:"grants"`
	// Public says a grant here opens the object to anyone at all. It is worked out on the Go side
	// because it is the one thing in this pane a reader must not have to spot for themselves: an
	// `AllUsers` read grant is an object the whole internet can fetch.
	Public bool `json:"public"`
}

// ObjectTag is one of the service's own labels on an object.
type ObjectTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ObjectVersion is one version, newest first. `Deleted` is a **delete marker** — a version whose
// whole content is the fact that the object was deleted at that moment.
type ObjectVersion struct {
	VersionID    string `json:"versionId"`
	Latest       bool   `json:"latest"`
	Deleted      bool   `json:"deleted"`
	Size         int64  `json:"size"`
	ModifiedAt   string `json:"modifiedAt"`
	ETag         string `json:"etag"`
	StorageClass string `json:"storageClass"`
}

// ObjectLogEntry is one thing **this app** did about an object. It is not the service's access log
// and the pane says so — see `objectJournal`.
type ObjectLogEntry struct {
	At      string `json:"at"`
	Action  string `json:"action"`
	Key     string `json:"key"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail"`
	MS      int64  `json:"ms"`
}

// ObjectPreview is as much of an object as is worth putting on a screen, and it is **a byte range and
// never the object**: a preview of a four-gigabyte log reads the first few hundred kilobytes of it.
//
// One of `Text` and `Data` is filled and the other is empty. Which one is the *content type's*
// answer, so nothing on the other side guesses from an extension.
type ObjectPreview struct {
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Text        string `json:"text"`
	// Data is a base64 data URL, for the one kind of object worth drawing rather than reading.
	Data string `json:"data"`
	// Truncated says the object is longer than what is here, which is the whole point of saying it:
	// a reader must never take a preview for the object.
	Truncated bool `json:"truncated"`
}

// SearchObjectsInput is a name to look for and where. `Mode` and `CaseInsensitive` are
// `domain.TextMatch`'s own — the same three fields a Find in Schema takes — so `Contains` means of a
// key exactly what it means of a column name.
type SearchObjectsInput struct {
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	Text            string `json:"text"`
	Mode            string `json:"mode"`
	CaseInsensitive bool   `json:"caseInsensitive"`
	MinSize         int64  `json:"minSize"`
	MaxSize         int64  `json:"maxSize"`
	// Since and Until are RFC 3339, or empty for no bound.
	Since string `json:"since"`
	Until string `json:"until"`
	Hits  int    `json:"hits"`
	Scan  int    `json:"scan"`
	// RunID is minted by the caller so a search already going can be stopped — StopObjectSearch
	// takes it, which is the arrangement a read of every row has.
	RunID string `json:"runId"`
}

// ObjectSearchResult is what a search found and **what it did not see**. The last four fields are
// the honest half: a bucket has no size a reader can know before asking, so a search that stopped
// is the ordinary case rather than the failure one.
type ObjectSearchResult struct {
	Rows []ObjectRow `json:"rows"`
	// Scanned is how many keys were looked at.
	Scanned int `json:"scanned"`
	// Prefix is what the *service* was narrowed by, which is the search's own explanation of why it
	// was quick. It is shown, for the reason a table read shows its statement.
	Prefix string `json:"prefix"`
	// Whole says every key under the prefix was looked at.
	Whole bool `json:"whole"`
	// More says it stopped because it had found enough, so there are more of these.
	More bool `json:"more"`
}

// DownloadObject is one object a reader ticked, with **the three facts the listing they ticked it in
// already carried**. They are sent rather than asked for again, and each of them earns its place: the
// size is what a progress bar's total is made of, the tag is what a finished download is verified
// against, and the clock is what `Only if newer` compares a file's against.
//
// Leaving them out is not a small thing, and it was a real defect until a live run showed it: a job
// whose total counted only the *walked* objects reported a bar that was already past its end, and
// every ticked object cost an extra `Head` before its first byte.
type DownloadObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
	ETag string `json:"etag"`
	// ModifiedAt is RFC 3339, as every other moment on this wire is.
	ModifiedAt string `json:"modifiedAt"`
}

// DownloadInput is what a reader asked to be brought down. `Keys` are objects and `Prefixes` are
// whole folders, and one request may carry both because a reader ticks rows without minding which
// kind they are.
type DownloadInput struct {
	Bucket   string           `json:"bucket"`
	Keys     []DownloadObject `json:"keys"`
	Prefixes []string         `json:"prefixes"`
	// Into is the folder on this machine. It is the reader's own answer from the host's folder
	// picker; nothing here invents one.
	Into string `json:"into"`
	// Under is the prefix the reader was standing in, which is taken off the front of every key so
	// that downloading `logs/2024/` writes `2024/…` and not `logs/2024/…`.
	Under string `json:"under"`
	// Collision is what to do about a file that is already there: `skip` (the default and the one
	// that loses nothing), `overwrite`, `rename`, or `newer`, which is what makes a second run of the
	// same folder a sync.
	Collision string `json:"collision"`
}

// UploadInput is the other direction: files this machine holds, into a prefix.
type UploadInput struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
	// Paths are files on this machine, from the host's own picker.
	Paths []string `json:"paths"`
}

// TransferJob is one job as the panel draws a row for it.
type TransferJob struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Bucket string `json:"bucket"`
	Label  string `json:"label"`
	Root   string `json:"root"`
	State  string `json:"state"`
	Files  int    `json:"files"`
	Done   int    `json:"done"`
	Failed int    `json:"failed"`
	// Skipped is files a reader's own collision rule left alone, and it is counted apart from Done
	// because "nothing was written and that is correct" is a different sentence.
	Skipped int   `json:"skipped"`
	Bytes   int64 `json:"bytes"`
	Moved   int64 `json:"moved"`
	// Counting says the listing that finds the files has not finished, so `Files` and `Bytes` are
	// still climbing — which is why the panel draws a count rather than a percentage while it is
	// true. A bar drawn from a total that is still moving is a bar that goes backwards.
	Counting bool `json:"counting"`
	// Rate is bytes a second, and zero for a job that is not moving.
	Rate      int64  `json:"rate"`
	Detail    string `json:"detail"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

// TransferFile is one file in flight. It is a separate list from the jobs because it moves at a
// different rate: a job's counts step and a file's bytes stream.
type TransferFile struct {
	JobID string `json:"jobId"`
	Key   string `json:"key"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Moved int64  `json:"moved"`
}

// TransferReport is the queue as the panel draws it. It is bounded on both lists — see
// `objectstore.Report` — because it crosses the bridge several times a second.
type TransferReport struct {
	Jobs  []TransferJob  `json:"jobs"`
	Files []TransferFile `json:"files"`
	// Active is how many jobs are still going, and it is what the panel watches: zero is when it
	// stops asking.
	Active int `json:"active"`
}

// RemoveResult is what a delete came to. `Refused` is not a failure: a service reports a key it
// would not remove inside a **successful** response, so a delete of a thousand where two were
// refused is nine hundred and ninety-eight gone and two named.
type RemoveResult struct {
	Deleted int      `json:"deleted"`
	Refused []string `json:"refused"`
}

// CopyResult is what a copy came to. The bytes never came here — a copy is one request naming both
// keys — which is why this counts objects rather than reporting a transfer.
type CopyResult struct {
	Objects int   `json:"objects"`
	Bytes   int64 `json:"bytes"`
}

// storeDTO is a profile on the wire, with the three "is one stored" facts filled in.
func storeDTO(profile domain.StoreProfile, hasSecret, hasSSH, hasProxy bool) Store {
	return Store{
		ID:          profile.ID,
		Name:        profile.Name,
		Provider:    string(profile.Provider),
		Environment: string(profile.Environment),
		Colour:      string(profile.Colour),
		Access:      string(profile.Access),
		Endpoint:    profile.Endpoint,
		Region:      profile.Region,
		Bucket:      profile.Bucket,
		PathStyle:   profile.PathStyle,
		Auth:        string(profile.Auth),
		AccessKeyID: profile.AccessKeyID,
		TLS: TLSSettings{
			Mode:       string(profile.TLS.Mode),
			RootCert:   profile.TLS.RootCert,
			ClientCert: profile.TLS.ClientCert,
			ClientKey:  profile.TLS.ClientKey,
			ServerName: profile.TLS.ServerName,
		},
		SSH: SSHSettings{
			Enabled:       profile.SSH.Enabled,
			Host:          profile.SSH.Host,
			Port:          profile.SSH.Port,
			User:          profile.SSH.User,
			Auth:          string(profile.SSH.Auth),
			KeyPath:       profile.SSH.KeyPath,
			KnownHosts:    profile.SSH.KnownHosts,
			StrictHostKey: profile.SSH.StrictHostKey,
		},
		Proxy: ProxySettings{
			Enabled: profile.Proxy.Enabled,
			Kind:    string(profile.Proxy.Kind),
			Host:    profile.Proxy.Host,
			Port:    profile.Proxy.Port,
			User:    profile.Proxy.User,
		},
		SaveSecret:     profile.SaveSecret,
		HasSecret:      hasSecret,
		HasSSHSecret:   hasSSH,
		HasProxySecret: hasProxy,
		Target:         profile.Target(),
	}
}

func (input StoreInput) toDomain() domain.StoreProfile {
	return domain.StoreProfile{
		ID:          input.ID,
		Name:        input.Name,
		Provider:    domain.StoreProvider(input.Provider),
		Environment: domain.Environment(input.Environment),
		Colour:      domain.Colour(input.Colour),
		Access:      domain.Access(input.Access),
		Endpoint:    input.Endpoint,
		Region:      input.Region,
		Bucket:      input.Bucket,
		PathStyle:   input.PathStyle,
		Auth:        domain.StoreAuth(input.Auth),
		AccessKeyID: input.AccessKeyID,
		TLS: domain.TLSOptions{
			Mode:       domain.TLSMode(input.TLS.Mode),
			RootCert:   input.TLS.RootCert,
			ClientCert: input.TLS.ClientCert,
			ClientKey:  input.TLS.ClientKey,
			ServerName: input.TLS.ServerName,
		},
		SSH: domain.SSHTunnel{
			Enabled:       input.SSH.Enabled,
			Host:          input.SSH.Host,
			Port:          input.SSH.Port,
			User:          input.SSH.User,
			Auth:          domain.SSHAuth(input.SSH.Auth),
			KeyPath:       input.SSH.KeyPath,
			KnownHosts:    input.SSH.KnownHosts,
			StrictHostKey: input.SSH.StrictHostKey,
		},
		Proxy: domain.ProxyOptions{
			Enabled: input.Proxy.Enabled,
			Kind:    domain.ProxyKind(input.Proxy.Kind),
			Host:    input.Proxy.Host,
			Port:    input.Proxy.Port,
			User:    input.Proxy.User,
		},
		SaveSecret: input.SaveSecret,
	}
}

// objectRow is one object as a grid row. A folder is `folderRow`.
func objectRow(object objectstore.Object) ObjectRow {
	return ObjectRow{
		Key:          object.Key,
		Name:         object.Name(),
		Size:         object.Size,
		ModifiedAt:   momentText(object.Modified),
		ETag:         object.ETag,
		StorageClass: object.StorageClass,
	}
}

// folderRow is a common prefix as a grid row. It carries the whole prefix as its key — which is what
// a folder download and a folder delete are both addressed to — and its **tail** as its name.
func folderRow(prefix string) ObjectRow {
	return ObjectRow{
		Key:    prefix,
		Name:   objectstore.LastSegment(prefix),
		Folder: true,
	}
}
