package domain

import (
	"errors"
	"net/url"
	"strings"
)

// StoreProvider is which service an object store *is*, and it is a **preset and not an adapter**.
// Every one of them speaks the same protocol — S3's REST API, signed with SigV4 — so what a
// provider decides is three fields of a profile the reader would otherwise have to know by heart:
// the endpoint, whether a bucket is named in the host or in the path, and what a region means to
// it. One speller, several front doors.
//
// ProviderOther is the escape hatch and the honest one: a reader who has an S3 endpoint nobody
// listed here types it in, and nothing about the protocol changes.
type StoreProvider string

const (
	ProviderAWS    StoreProvider = "aws"
	ProviderMinIO  StoreProvider = "minio"
	ProviderR2     StoreProvider = "r2"
	ProviderSpaces StoreProvider = "spaces"
	ProviderWasabi StoreProvider = "wasabi"
	ProviderGCS    StoreProvider = "gcs"
	ProviderOther  StoreProvider = "other"
)

func (p StoreProvider) Valid() bool {
	switch p {
	case ProviderAWS, ProviderMinIO, ProviderR2, ProviderSpaces, ProviderWasabi,
		ProviderGCS, ProviderOther:
		return true
	default:
		return false
	}
}

// NeedsEndpoint says the profile has to carry one, because this provider's address cannot be
// worked out from a region. AWS's can — s3.<region>.amazonaws.com — and every provider that is
// somebody's own installation cannot.
func (p StoreProvider) NeedsEndpoint() bool {
	return p != ProviderAWS
}

// PathStyle says a bucket is named in the *path* rather than as a label of the host. It is the one
// difference between providers that would otherwise reach the reader as a 404 with nothing in it:
// a MinIO in a container answers /bucket/key and nothing else, and virtual-host style there
// resolves a name that does not exist.
//
// It is a **default** and not a rule: the profile carries its own flag, because a reader with a
// wildcard DNS entry in front of their own MinIO may well want the other form.
func (p StoreProvider) PathStyle() bool {
	switch p {
	case ProviderMinIO, ProviderGCS, ProviderOther:
		return true
	default:
		return false
	}
}

// DefaultRegion is the name this service expects a signature to carry when the profile names none.
// Two of them have no regions at all and still insist on one being signed for, which is exactly the
// sort of fact a reader should not have to know.
func (p StoreProvider) DefaultRegion() string {
	switch p {
	case ProviderR2:
		return "auto"
	case ProviderGCS, ProviderMinIO, ProviderOther:
		return "us-east-1"
	default:
		return ""
	}
}

// StoreAuth is what the *service* authenticates the reader by, and it is AuthMethod's counterpart
// for a store: one is a key pair the reader holds and one is the identity the machine already has.
//
// There is no "no credentials" here. An anonymous S3 endpoint exists, but a bucket a reader wants
// listed is one they hold a key for, and an unsigned request is refused by every provider above.
type StoreAuth string

const (
	// StoreKey is an access key id and a secret. The id is on the profile and the secret is in the
	// credential store, exactly as a password is.
	StoreKey StoreAuth = "key"
	// StoreEnvironment is the credentials the machine's AWS tools already agree on, read at the
	// moment a connection is made and never stored — the arrangement AuthAWSIAM has, and for the
	// same reason: a session token good for an hour is not a secret worth keeping.
	StoreEnvironment StoreAuth = "environment"
)

func (a StoreAuth) Valid() bool {
	return a == StoreKey || a == StoreEnvironment
}

// Minted says the credentials are read at connect time rather than known, which is what decides
// whether there is anything to save, prompt for, or read back out of the credential store.
func (a StoreAuth) Minted() bool {
	return a == StoreEnvironment
}

// errNoHost is an endpoint that parsed and named nothing — `https://` on its own, or a path with
// no authority in front of it. It is not a sentinel the UI switches on: Validate turns it into the
// ValidationError the endpoint box draws.
var errNoHost = errors.New("no host in the endpoint")

// StoreProfile is a connection to an object store, and it is ConnectionProfile's sibling rather
// than one of its engines: there is no catalogue here, no statement, and nothing that is a row —
// see docs/architecture.md, *Buckets full of objects*.
//
// The fields it shares with a database connection are the ones about the *reader* rather than about
// the protocol — a name, what the connection is for, its colour, what this app will let it do, and
// the hops that carry it — and they are shared on purpose: a bucket beside a production database
// wants the same red stripe and the same refusal.
//
// **No secret is here.** A secret access key lives in the credential store under the profile's id,
// a bastion's pass phrase under `<id>:ssh` and a proxy's password under `<id>:proxy`, exactly as a
// database connection's do.
type StoreProfile struct {
	ID          string
	Name        string
	Provider    StoreProvider
	Environment Environment
	Colour      Colour
	// Access is what this app will let the connection do at all. A read-only store refuses an
	// upload, a delete and a copy, and the guard is one wrapper for the reason a database's is.
	Access Access
	// Endpoint is the service's address, with or without a scheme. A missing scheme is https,
	// because a bucket reached in the clear is a bucket whose keys are on the wire.
	Endpoint string
	Region   string
	// Bucket is the one the tree opens on, and empty is every bucket the credentials can list.
	// It is not a filter but an *answer*: a key that may read one bucket and not list them is the
	// ordinary case in an organisation, and a tree that could only start from a listing would show
	// such a reader nothing at all.
	Bucket string
	// PathStyle names the bucket in the path rather than in the host — see StoreProvider.PathStyle.
	PathStyle bool
	Auth      StoreAuth
	// AccessKeyID is the public half of the key pair, and is not a secret: it identifies the key
	// and cannot sign anything on its own.
	AccessKeyID string
	TLS         TLSOptions
	SSH         SSHTunnel
	Proxy       ProxyOptions
	SaveSecret  bool
}

func (p StoreProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ValidationError{Field: "name", Message: "is required"}
	}
	if !p.Provider.Valid() {
		return ValidationError{Field: "provider", Message: "is not a service this app knows"}
	}
	if !p.Auth.Valid() {
		return ValidationError{Field: "storeAuth", Message: "is not a known authentication method"}
	}
	if !p.Environment.Valid() {
		return ValidationError{Field: "environment", Message: "is not a known environment"}
	}
	if !p.Colour.Valid() {
		return ValidationError{Field: "colour", Message: "is not a known colour"}
	}
	if !p.Access.Valid() {
		return ValidationError{Field: "access", Message: "is not a known access mode"}
	}

	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" && p.Provider.NeedsEndpoint() {
		return ValidationError{Field: "endpoint", Message: "is required for this service"}
	}
	if endpoint != "" {
		if _, err := p.BaseURL(); err != nil {
			return ValidationError{Field: "endpoint", Message: "is not an address this app can read"}
		}
	}
	// AWS is the one provider whose address *is* its region, so a profile with neither has nothing
	// to dial. Everywhere else an empty region is signed with the provider's own.
	if p.Provider == ProviderAWS && strings.TrimSpace(p.Region) == "" && endpoint == "" {
		return ValidationError{Field: "region", Message: "is required to reach an AWS endpoint"}
	}
	if p.Auth == StoreKey && strings.TrimSpace(p.AccessKeyID) == "" {
		return ValidationError{Field: "accessKeyId", Message: "is required"}
	}
	// A bucket name reaches a URL, so the characters that would make it into somewhere else are
	// refused here rather than signed.
	if strings.ContainsAny(p.Bucket, "/?#") {
		return ValidationError{Field: "bucket", Message: "is one bucket's name, not a path"}
	}

	if err := p.TLS.Validate(); err != nil {
		return err
	}
	if err := p.Proxy.Validate(); err != nil {
		return err
	}
	return p.SSH.Validate()
}

// SigningRegion is the region a request of this profile is signed for: the profile's own, the
// provider's fixed one, or us-east-1 — which is what the services that have no regions at all
// still expect a signature to name.
func (p StoreProfile) SigningRegion() string {
	if region := strings.TrimSpace(p.Region); region != "" {
		return region
	}
	if region := p.Provider.DefaultRegion(); region != "" {
		return region
	}
	return "us-east-1"
}

// BaseURL is the service's address as a URL, with the scheme filled in and the path taken off. A
// missing scheme is **https**, and that is the one default here that is a security decision rather
// than a convenience: a reader who means plain text says so out loud.
func (p StoreProfile) BaseURL() (*url.URL, error) {
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" {
		endpoint = "s3." + p.SigningRegion() + ".amazonaws.com"
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, errNoHost
	}
	// A path on the endpoint would be prepended to every key, so a reader who pasted a console URL
	// gets the service rather than a prefix nobody meant.
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

// Target is where this connection goes, as the status bar says it: the bucket the tree opens on
// over the service's own host, which is the sentence a database connection's makes.
func (p StoreProfile) Target() string {
	host := p.Endpoint
	if parsed, err := p.BaseURL(); err == nil {
		host = parsed.Host
	}
	if p.Bucket == "" {
		return "(all buckets)@" + host
	}
	return p.Bucket + "@" + host
}

// ObjectAction is a thing a reader does to what a bucket holds, and the list is deliberately short:
// everything else an object store offers is a *read*, and a read is never answered for.
type ObjectAction string

const (
	// ObjectUpload writes an object, which may be over one that is already there.
	ObjectUpload ObjectAction = "upload"
	// ObjectDelete removes objects that were **named** — the rows a reader ticked.
	ObjectDelete ObjectAction = "delete"
	// ObjectDeletePrefix removes everything under a prefix, which names no object at all.
	ObjectDeletePrefix ObjectAction = "delete-prefix"
	// ObjectEmptyBucket removes everything in a bucket.
	ObjectEmptyBucket ObjectAction = "empty-bucket"
	// ObjectDeleteBucket removes the bucket itself.
	ObjectDeleteBucket ObjectAction = "delete-bucket"
	// ObjectCopy writes a copy beside the original.
	ObjectCopy ObjectAction = "copy"
	// ObjectMove is that copy with a delete after it, so the original goes.
	ObjectMove ObjectAction = "move"
	// ObjectCreateBucket makes one, which takes nothing away.
	ObjectCreateBucket ObjectAction = "create-bucket"
	// ObjectHeaders changes what an object's headers say. It is a **write** and not a mere edit, and
	// the reason is the protocol's: there is no request that sets a header, so what happens is a copy
	// of the object onto itself — a new tag, a new modified time, and a new version on a bucket that
	// keeps them. It is *not* unfiltered: it is aimed at one object and nothing else goes with it.
	ObjectHeaders ObjectAction = "headers"
)

// ObjectHazard is StatementHazard for a bucket — the **eighth** grammar for one rule, and here for
// the reason RedisHazard and ProfileHazard are: what is dangerous is a domain rule and only its
// spelling changes. There is no statement to read a keyword off, so the verb *is* the action.
//
// The three that name **nothing** are `unfiltered` rather than merely destructive, which is the
// distinction this rule exists for: `DELETE FROM users` with no `WHERE` is a mistake on a laptop as
// surely as in production, and so is emptying a bucket. A delete of the rows a reader ticked is
// destructive, and is answered for on a guarded connection alone.
func ObjectHazard(action ObjectAction) Hazard {
	switch action {
	case ObjectUpload, ObjectCopy, ObjectCreateBucket, ObjectHeaders:
		return HazardWrite
	case ObjectDelete, ObjectMove:
		return HazardDestructive
	case ObjectDeletePrefix, ObjectEmptyBucket, ObjectDeleteBucket:
		return HazardUnfiltered
	default:
		return HazardNone
	}
}

// ObjectVerb is what an action is *called* where one is refused or answered for, so a dialog and a
// read-only refusal name the same thing the same way.
func ObjectVerb(action ObjectAction) string {
	switch action {
	case ObjectUpload:
		return "Upload"
	case ObjectDelete:
		return "Delete"
	case ObjectDeletePrefix:
		return "Delete Folder"
	case ObjectEmptyBucket:
		return "Empty Bucket"
	case ObjectDeleteBucket:
		return "Delete Bucket"
	case ObjectCreateBucket:
		return "New Bucket"
	case ObjectHeaders:
		return "Change Headers"
	case ObjectCopy:
		return "Copy"
	case ObjectMove:
		return "Move"
	default:
		return string(action)
	}
}
