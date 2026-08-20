package session

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/cloudauth"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/sshtunnel"
)

// StoreSecrets is everything an object-store connection is opened with that the profile does not
// hold. It is `Secrets` for a store, and the two are separate types because the first field means a
// different thing: a database takes a password and a store takes the secret half of a key pair.
type StoreSecrets struct {
	SecretKey string
	SSH       string
	Proxy     string
}

// StoreSession is one open connection to an object store. It is `Session`'s counterpart and holds
// the same three things: what was opened, what it was opened *with*, and the bastion it is carried
// over.
type StoreSession struct {
	ID      string
	Profile domain.StoreProfile

	store objectstore.Store
	// tunnel is the bastion this session is carried over, and is nil for a direct connection. It
	// lives exactly as long as the session, for the reason a database's does.
	tunnel *sshtunnel.Tunnel
}

// Store is what the app layer reads and writes through. It is already the **guarded** one on a
// read-only profile — see readOnlyStore — so there is no second seam an upload could go round.
func (s *StoreSession) Store() objectstore.Store {
	return s.store
}

// StoreManager is every open store connection, and it is `Manager`'s sibling rather than a second
// map inside it: a store session has no databases, no schemas and no statements, so the forty
// methods that forward a catalogue read would all have been nonsense here.
type StoreManager struct {
	open objectstore.Opener

	mu       sync.RWMutex
	sessions map[string]*StoreSession
}

func NewStoreManager(open objectstore.Opener) *StoreManager {
	return &StoreManager{open: open, sessions: make(map[string]*StoreSession)}
}

// Open dials a store, doing in the same order the same four things `Manager.Open` does: the profile
// is checked, the hops that carry the connection are stood up — the proxy first and the bastion
// through it — the credentials are resolved, and a read-only profile is wrapped.
//
// The order of the hops is the fixed one and for the fixed reason: a proxy is how this machine
// reaches the network at all, so the bastion is dialled *through* it and the service through the
// bastion. Nothing in `internal/objectstore` has heard of either.
//
// It opens no socket. Every call to an S3 service is a request of its own, so there is nothing to
// dial here — which is why the reader's Test Connection asks for the buckets rather than calling
// this and believing it.
func (m *StoreManager) Open(
	ctx context.Context,
	profile domain.StoreProfile,
	secrets StoreSecrets,
) (*StoreSession, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	hops := Secrets{SSH: secrets.SSH, Proxy: secrets.Proxy}

	proxied, err := openProxy(profile.Proxy, hops)
	if err != nil {
		return nil, err
	}

	tunnel, err := openTunnel(ctx, profile.SSH, hops, proxied)
	if err != nil {
		return nil, err
	}

	fail := func(err error) (*StoreSession, error) {
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, err
	}

	creds, err := storeCredentials(profile, secrets)
	if err != nil {
		return fail(err)
	}

	var dial objectstore.Dialer
	if carried := carrier(tunnel, proxied); carried != nil {
		dial = objectstore.Dialer(carried)
	}

	store, err := m.open(profile, creds, dial)
	if err != nil {
		return fail(err)
	}

	if profile.Access.ReadOnly() {
		store = readOnlyStore{Store: store}
	}

	id, err := newID()
	if err != nil {
		store.Close()
		return fail(err)
	}

	session := &StoreSession{ID: id, Profile: profile, store: store, tunnel: tunnel}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session, nil
}

// storeCredentials is the key pair the service will be signed for. A profile that names a key reads
// its secret out of the credential store; one that names the machine's own AWS identity reads the
// environment **now**, per connection, and stores nothing — which is `credentials`' rule for a cloud
// token applied to a key that may be a temporary one.
func storeCredentials(
	profile domain.StoreProfile,
	secrets StoreSecrets,
) (objectstore.Credentials, error) {
	if profile.Auth == domain.StoreEnvironment {
		found, ok := cloudauth.AWSEnvironment()
		if !ok {
			return objectstore.Credentials{}, fmt.Errorf(
				"%w: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, "+
					"or run `aws configure export-credentials --format env`",
				domain.ErrTokenUnavailable,
			)
		}
		return objectstore.Credentials{
			AccessKeyID:     found.AccessKeyID,
			SecretAccessKey: found.SecretAccessKey,
			SessionToken:    found.SessionToken,
		}, nil
	}

	return objectstore.Credentials{
		AccessKeyID:     profile.AccessKeyID,
		SecretAccessKey: secrets.SecretKey,
	}, nil
}

func (m *StoreManager) Get(id string) (*StoreSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return session, nil
}

func (m *StoreManager) Close(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()

	if !ok {
		return domain.ErrSessionNotFound
	}
	session.close()
	return nil
}

func (m *StoreManager) CloseAll() {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*StoreSession)
	m.mu.Unlock()

	for _, session := range sessions {
		session.close()
	}
}

// close lets the connection pool go and then the tunnel, because every connection in that pool was
// carried over it.
func (s *StoreSession) close() {
	s.store.Close()
	if s.tunnel != nil {
		s.tunnel.Close()
	}
}

// readOnlyStore is what a read-only store connection means, and it is `readOnly`'s counterpart:
// the store the manager hands out is already this one, so an upload, a delete and a copy are all
// guarded at one seam.
//
// It is much simpler than a database's guard for a plain reason: **an object store has no statement
// to review.** A `Put` writes and a `List` does not, and there is nothing here whose meaning depends
// on what a reader typed — so the classification is the method itself and `domain.ObjectHazard` is
// what names it.
//
// What it does *not* refuse is a `Presign`. A signed URL reads, and reading is what this connection
// is for; that it can be handed to somebody else is true of every row the reader can already copy.
type readOnlyStore struct {
	objectstore.Store
}

func (readOnlyStore) Put(context.Context, objectstore.PutRequest) error {
	return refuseObject(domain.ObjectUpload)
}

func (readOnlyStore) CreateUpload(context.Context, string, string, string) (string, error) {
	return "", refuseObject(domain.ObjectUpload)
}

func (readOnlyStore) UploadPart(
	context.Context, string, string, string, int, io.Reader, int64,
) (objectstore.Part, error) {
	return objectstore.Part{}, refuseObject(domain.ObjectUpload)
}

func (readOnlyStore) CompleteUpload(context.Context, string, string, string, []objectstore.Part) error {
	return refuseObject(domain.ObjectUpload)
}

// AbortUpload is refused too, and it is the one that looks wrong until you see why: nothing in a
// read-only connection can have *started* an upload, so an abort here could only be aimed at parts
// somebody else left — which is a write to their bucket.
func (readOnlyStore) AbortUpload(context.Context, string, string, string) error {
	return refuseObject(domain.ObjectUpload)
}

func (readOnlyStore) Delete(context.Context, string, []string) ([]string, error) {
	return nil, refuseObject(domain.ObjectDelete)
}

// WriteMetadata is refused, and it is worth saying why it is not a borderline case: changing a
// header is a **copy of the object onto itself**, so it rewrites the object — a new tag, a new
// modified time, a new version. There is nothing read-only about it.
func (readOnlyStore) WriteMetadata(context.Context, string, string, objectstore.Metadata) error {
	return refuseObject(domain.ObjectHeaders)
}

func (readOnlyStore) Copy(context.Context, string, string, string, string) error {
	return refuseObject(domain.ObjectCopy)
}

func (readOnlyStore) MakeBucket(context.Context, string) error {
	return refuseObject(domain.ObjectCreateBucket)
}

func (readOnlyStore) RemoveBucket(context.Context, string) error {
	return refuseObject(domain.ObjectDeleteBucket)
}

// refuseObject names what was not sent, in the reader's own word for it — the same word the dialog
// that would have asked about it uses, because `domain.ObjectVerb` is where both read it from.
func refuseObject(action domain.ObjectAction) error {
	return domain.ReadOnly(domain.ObjectVerb(action))
}
