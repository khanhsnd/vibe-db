// Package s3 is the one adapter `internal/objectstore` has, and one is the honest number: AWS S3,
// MinIO, Cloudflare R2, DigitalOcean Spaces, Wasabi, Ceph and Google's interoperability endpoint all
// speak the same protocol, so what looks like seven services is **one speller and a preset** —
// `domain.StoreProvider`. A second adapter here would mean somebody had shipped a genuinely
// different protocol, not a different company.
//
// It takes **no SDK**. The REST API is documented, the signature is documented, and the answers are
// six XML documents (`wire.go`) — where an SDK is fifteen megabytes in the binary and a dependency
// tree to keep. That is `internal/transfer`'s decision about a spreadsheet and
// `internal/cloudauth`'s about a token, applied where it saves the most.
package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/sigv4"
	"github.com/khanhsnd/vibe-db/internal/tlscert"
)

// service is what a signature names, and it is `s3` for every provider above — including the ones
// that are not AWS, because the protocol they implement is AWS's.
const service = "s3"

const (
	// dialTimeout, headerTimeout and handshakeTimeout are the three bounds this client has, and
	// there is deliberately **no timeout on the whole request**: a forty-gigabyte body is a request
	// that takes an hour, and `http.Client.Timeout` would cut it off at whatever number somebody
	// guessed. What is bounded is the parts that should be quick — reaching the host, negotiating
	// TLS, and getting the *headers* back — and the body is bounded by the reader's own Stop.
	dialTimeout      = 15 * time.Second
	headerTimeout    = 60 * time.Second
	handshakeTimeout = 15 * time.Second
	// idleConns is how many connections are kept warm per host. Sixteen ranges in flight is the
	// queue's own ceiling, and a pool smaller than that would close and reopen a connection between
	// every part.
	idleConns = 32
)

// Options is everything the adapter needs and nothing it can work out. The credentials arrive
// **already resolved** — read from the credential store or from the machine's AWS environment by
// the layer above — which is the arrangement a database adapter has with a cloud token, and is why
// nothing here has heard of a keyring.
type Options struct {
	Profile     domain.StoreProfile
	Credentials sigv4.Credentials
	// Dial carries the connection: a bastion, a proxy, or nil for this machine's own network. The
	// adapter neither knows nor cares which — see `objectstore.Dialer`.
	Dial objectstore.Dialer
	// Now is the clock a signature is stamped with, and is `time.Now` everywhere but a test.
	Now func() time.Time
}

// The port is what this file is measured against, and it is asserted here so that a method added to
// `objectstore.Store` is a build failure rather than a runtime one.
var _ objectstore.Store = (*Store)(nil)

// Store is one service, dialled. It holds a connection pool and nothing else: every method is one
// request, and everything that is several is written above it.
type Store struct {
	client    *http.Client
	base      *url.URL
	pathStyle bool
	region    string
	creds     sigv4.Credentials
	now       func() time.Time
}

// Open builds the client. It opens no connection — there is nothing to open in a protocol whose
// every call is a request — so a profile that cannot be reached says so on the first read rather
// than here, and `Buckets` is what a Test Connection asks.
func Open(options Options) (*Store, error) {
	base, err := options.Profile.BaseURL()
	if err != nil {
		return nil, domain.ValidationError{Field: "endpoint", Message: "is not an address this app can read"}
	}

	host := base.Hostname()
	tls, err := tlscert.Config(options.Profile.TLS, host)
	if err != nil {
		return nil, err
	}

	dial := options.Dial
	if dial == nil {
		dialler := &net.Dialer{Timeout: dialTimeout}
		dial = dialler.DialContext
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &Store{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext:           dial,
				TLSClientConfig:       tls,
				TLSHandshakeTimeout:   handshakeTimeout,
				ResponseHeaderTimeout: headerTimeout,
				MaxIdleConns:          idleConns,
				MaxIdleConnsPerHost:   idleConns,
				IdleConnTimeout:       90 * time.Second,
				// HTTP/2 is deliberately off. Sixteen ranges over one TCP connection share one
				// congestion window and one head of line, which is slower than sixteen connections
				// for exactly the workload this app has — reading big objects fast. Every S3 SDK
				// there is makes the same choice.
				ForceAttemptHTTP2: false,
			},
		},
		base:      base,
		pathStyle: options.Profile.PathStyle,
		region:    options.Profile.SigningRegion(),
		creds:     options.Credentials,
		now:       now,
	}, nil
}

func (s *Store) Close() {
	s.client.CloseIdleConnections()
}

// address is where a request goes: the bucket as a label of the host, or as the first segment of the
// path. It is the one structural difference between the providers and it is a **flag**, which is the
// form every difference in this codebase takes.
func (s *Store) address(bucket, key string, query map[string]string) *url.URL {
	target := &url.URL{Scheme: s.base.Scheme, Host: s.base.Host}

	path := ""
	switch {
	case bucket == "":
	case s.pathStyle:
		path = "/" + bucket
	default:
		target.Host = bucket + "." + s.base.Host
	}
	if key != "" {
		path += "/" + strings.TrimPrefix(key, "/")
	}
	if path == "" {
		path = "/"
	}

	// Both forms are set: `Path` is what Go compares and unescapes, and `RawPath` is the escaping a
	// **signature** was computed over. Letting `net/url` escape it instead would leave `+` and `:`
	// alone in the request and escape them in the signature, which is a signature every service
	// works out differently.
	target.Path = path
	target.RawPath = sigv4.EscapePath(path)
	target.RawQuery = sigv4.EncodeQuery(query)
	return target
}

// send signs the request and makes it. Everything in this file goes through it, so there is one
// place a signature is made and one place a refusal is read.
func (s *Store) send(
	ctx context.Context,
	method string,
	target *url.URL,
	body io.Reader,
	size int64,
	payload string,
	headers map[string]string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	// `NewRequest` re-parses the URL and loses the strict escaping above, so the one this method
	// built is put back.
	request.URL = target
	request.Host = target.Host
	if size >= 0 {
		request.ContentLength = size
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	sigv4.SignRequest(request, s.creds, s.region, service, payload, s.now())

	response, err := s.client.Do(request)
	if err != nil {
		return nil, wire(err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, refusal(response)
	}
	return response, nil
}

// read makes a request whose answer is an XML document, and closes the body. Every listing goes
// through it.
func (s *Store) read(ctx context.Context, method string, target *url.URL, into any) error {
	response, err := s.send(ctx, method, target, nil, 0, sigv4.EmptyPayload, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// An answer is bounded because it is a *listing*: a thousand keys is a few hundred kilobytes,
	// and a service answering a listing with a gigabyte is a service to stop reading.
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return wire(err)
	}
	if into == nil {
		return nil
	}
	if err := xml.Unmarshal(body, into); err != nil {
		return domain.ServiceError{Code: "unreadable", Message: "the service's answer is not the XML this app expects"}
	}
	return nil
}

func (s *Store) Buckets(ctx context.Context) ([]objectstore.Bucket, error) {
	var result listBucketsResult
	if err := s.read(ctx, http.MethodGet, s.address("", "", nil), &result); err != nil {
		return nil, err
	}

	buckets := make([]objectstore.Bucket, 0, len(result.Buckets.Bucket))
	for _, entry := range result.Buckets.Bucket {
		buckets = append(buckets, objectstore.Bucket{
			Name:    entry.Name,
			Created: stamp(entry.CreationDate),
		})
	}
	return buckets, nil
}

func (s *Store) List(ctx context.Context, request objectstore.ListRequest) (objectstore.Listing, error) {
	if request.Bucket == "" {
		return objectstore.Listing{}, domain.ErrUnknownBucket
	}

	limit := request.Limit
	if limit <= 0 || limit > objectstore.PageSize {
		limit = objectstore.PageSize
	}

	query := map[string]string{
		"list-type": "2",
		"max-keys":  strconv.Itoa(limit),
		// Asking for encoded keys is what makes a key holding a newline or a control character
		// readable at all — see `decodeKey`.
		"encoding-type": "url",
	}
	if request.Prefix != "" {
		query["prefix"] = request.Prefix
	}
	if request.Delimiter != "" {
		query["delimiter"] = request.Delimiter
	}
	if request.Token != "" {
		query["continuation-token"] = request.Token
	}

	var result listObjectsResult
	if err := s.read(ctx, http.MethodGet, s.address(request.Bucket, "", query), &result); err != nil {
		return objectstore.Listing{}, err
	}
	return objectsOf(result), nil
}

func (s *Store) Head(ctx context.Context, bucket, key string) (objectstore.Object, error) {
	response, err := s.send(ctx, http.MethodHead, s.address(bucket, key, nil), nil, 0, sigv4.EmptyPayload, nil)
	if err != nil {
		return objectstore.Object{}, err
	}
	response.Body.Close()

	return objectstore.Object{
		Key:          key,
		Size:         response.ContentLength,
		Modified:     stamp(response.Header.Get("Last-Modified")),
		ETag:         strings.Trim(response.Header.Get("ETag"), `"`),
		StorageClass: response.Header.Get("X-Amz-Storage-Class"),
		ContentType:  response.Header.Get("Content-Type"),
		Metadata:     metadataOf(response.Header),
		Headers:      headersOf(response.Header),
	}, nil
}

// metaPrefix is what a service puts in front of a header the *uploader* attached.
const metaPrefix = "X-Amz-Meta-"

// metadataOf is the uploader's own facts, with the prefix taken off — which is what makes them
// readable as the pairs somebody meant rather than as protocol.
func metadataOf(header http.Header) map[string]string {
	meta := map[string]string{}
	for name, values := range header {
		if !strings.HasPrefix(name, metaPrefix) || len(values) == 0 {
			continue
		}
		meta[strings.TrimPrefix(name, metaPrefix)] = values[0]
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// headersOf is the whole answer, sorted by name. The order the service sent them in is gone by the
// time this sees them — `http.Header` is a map — so one is chosen rather than a claim made about it.
//
// A header sent twice keeps **both** values, joined, because that is what was said: a service may
// send two `Cache-Control` lines and showing one would be showing half the answer.
func headersOf(header http.Header) []objectstore.Header {
	out := make([]objectstore.Header, 0, len(header))
	for name, values := range header {
		out = append(out, objectstore.Header{Name: name, Value: strings.Join(values, ", ")})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// WriteMetadata is a **copy of the object onto itself** with `REPLACE`, because that is the only
// request there is: no service has one that sets a header. Everything about how it behaves follows
// from that, and the two consequences are the caller's to warn about — the whole set is replaced, and
// the object is rewritten.
//
// The storage class is sent back deliberately: a copy that named none would move the object to the
// service's default, which is a bill nobody asked for.
func (s *Store) WriteMetadata(
	ctx context.Context,
	bucket, key string,
	meta objectstore.Metadata,
) error {
	source := "/" + bucket + "/" + strings.TrimPrefix(key, "/")
	headers := map[string]string{
		"X-Amz-Copy-Source":        sigv4.EscapePath(source),
		"X-Amz-Metadata-Directive": "REPLACE",
	}

	for name, value := range meta.System {
		// Only the six. Anything else is the service's own and there is no request that writes it, so
		// it is dropped here rather than sent and ignored.
		if value != "" && objectstore.Settable(name) {
			headers[name] = value
		}
	}
	for name, value := range meta.User {
		if name != "" {
			headers[metaPrefix+name] = value
		}
	}
	if meta.StorageClass != "" {
		headers["X-Amz-Storage-Class"] = meta.StorageClass
	}

	response, err := s.send(
		ctx, http.MethodPut, s.address(bucket, key, nil), nil, 0, sigv4.EmptyPayload, headers,
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// A copy answers 200 with a failure in the body, for the reason `Copy` does: the service holds
	// the connection open while it moves the bytes.
	answer, err := io.ReadAll(io.LimitReader(response.Body, errorBody))
	if err != nil {
		return wire(err)
	}
	var failure serviceError
	if xml.Unmarshal(answer, &failure) == nil && failure.Code != "" {
		return domain.ServiceError{Code: failure.Code, Message: failure.Message}
	}
	return nil
}

func (s *Store) ObjectPermissions(ctx context.Context, bucket, key string) (objectstore.Permissions, error) {
	var result aclResult
	target := s.address(bucket, key, map[string]string{"acl": ""})
	if err := s.read(ctx, http.MethodGet, target, &result); err != nil {
		return objectstore.Permissions{}, err
	}

	permissions := objectstore.Permissions{
		Owner:  nameOf(result.Owner.DisplayName, result.Owner.ID),
		Grants: make([]objectstore.Permission, 0, len(result.Grants)),
	}
	for _, grant := range result.Grants {
		grantee, kind := granteeOf(grant.Grantee)
		permissions.Grants = append(permissions.Grants, objectstore.Permission{
			Grantee: grantee,
			Kind:    kind,
			Allow:   grant.Permission,
		})
	}
	return permissions, nil
}

// granteeOf reads who a grant is *for* off whichever child the service filled in. A URI is a
// **group** and is the one that matters most: `AllUsers` on a read grant is an object the whole
// internet can fetch, so it is named rather than shown as a URL nobody reads.
func granteeOf(grantee struct {
	ID           string `xml:"ID"`
	DisplayName  string `xml:"DisplayName"`
	URI          string `xml:"URI"`
	EmailAddress string `xml:"EmailAddress"`
}) (string, string) {
	switch {
	case grantee.URI != "":
		return objectstore.LastSegment(grantee.URI), "Group"
	case grantee.EmailAddress != "":
		return grantee.EmailAddress, "AmazonCustomerByEmail"
	default:
		return nameOf(grantee.DisplayName, grantee.ID), "CanonicalUser"
	}
}

// nameOf prefers the name a person would recognise and falls back to the id, which is what a service
// answers with where it keeps no display name at all.
func nameOf(display, id string) string {
	if display != "" {
		return display
	}
	return id
}

func (s *Store) ObjectTags(ctx context.Context, bucket, key string) ([]objectstore.Tag, error) {
	var result taggingResult
	target := s.address(bucket, key, map[string]string{"tagging": ""})
	if err := s.read(ctx, http.MethodGet, target, &result); err != nil {
		return nil, err
	}

	tags := make([]objectstore.Tag, 0, len(result.Tags))
	for _, tag := range result.Tags {
		tags = append(tags, objectstore.Tag{Name: tag.Key, Value: tag.Value})
	}
	return tags, nil
}

// ObjectVersions asks the **bucket** about a prefix and then keeps the one key, because that is the
// only shape the protocol has: there is no request for one object's versions. The prefix is the key
// itself, so what comes back is that object's versions and those of anything whose name begins with
// it — `report.csv` and `report.csv.bak` — and the second is filtered out here rather than shown as
// a version of the first.
func (s *Store) ObjectVersions(
	ctx context.Context,
	bucket, key string,
	limit int,
) ([]objectstore.Version, error) {
	if limit <= 0 {
		limit = objectstore.VersionLimit
	}

	query := map[string]string{
		"versions":      "",
		"prefix":        key,
		"encoding-type": "url",
		// More than asked for, because the filter below drops the neighbours a prefix let through.
		"max-keys": strconv.Itoa(min(limit*4, objectstore.PageSize)),
	}

	var result versionsResult
	if err := s.read(ctx, http.MethodGet, s.address(bucket, "", query), &result); err != nil {
		return nil, err
	}

	versions := make([]objectstore.Version, 0, limit)
	for _, one := range result.Version {
		if decodeKey(one.Key) != key {
			continue
		}
		versions = append(versions, objectstore.Version{
			VersionID:    one.VersionID,
			Latest:       one.IsLatest,
			Size:         one.Size,
			Modified:     stamp(one.LastModified),
			ETag:         strings.Trim(one.ETag, `"`),
			StorageClass: one.StorageClass,
		})
	}
	for _, one := range result.DeleteMarker {
		if decodeKey(one.Key) != key {
			continue
		}
		versions = append(versions, objectstore.Version{
			VersionID: one.VersionID,
			Latest:    one.IsLatest,
			Deleted:   true,
			Modified:  stamp(one.LastModified),
		})
	}

	// Newest first, which is the order a reader reads a history in — and the two lists above arrive
	// separately, so one order has to be put back over both.
	sort.Slice(versions, func(a, b int) bool { return versions[a].Modified.After(versions[b].Modified) })
	if len(versions) > limit {
		versions = versions[:limit]
	}
	return versions, nil
}

func (s *Store) Open(ctx context.Context, bucket, key string, span objectstore.Span) (io.ReadCloser, error) {
	headers := map[string]string{}
	if !span.Whole() {
		// A range is inclusive at both ends, which is the one thing about the header worth stating:
		// `bytes=0-8388607` is the first eight megabytes and not the first eight and one byte.
		end := ""
		if span.Length > 0 {
			end = strconv.FormatInt(span.Offset+span.Length-1, 10)
		}
		headers["Range"] = "bytes=" + strconv.FormatInt(span.Offset, 10) + "-" + end
	}

	response, err := s.send(ctx, http.MethodGet, s.address(bucket, key, nil), nil, 0, sigv4.EmptyPayload, headers)
	if err != nil {
		return nil, err
	}
	// A service that ignored the range answers 200 with the **whole** object, and whether that is
	// dangerous depends entirely on where the range started.
	//
	// From an **offset**, it is: the body begins at byte nought and would be written into the middle
	// of the file, corrupting everything after it. Refused.
	//
	// From **nought**, it is not: the body begins where it was asked to, so the caller reads the
	// bytes it wanted and stops. A service is even allowed to answer 200 to a range that covers the
	// whole object, which is exactly what a preview of a small object asks for — refusing that was a
	// bug, and it took an image that would not draw to find it.
	if span.Offset > 0 && response.StatusCode != http.StatusPartialContent {
		response.Body.Close()
		return nil, domain.Unsupported("reading part of an object (this service ignores a byte range)")
	}
	return response.Body, nil
}

func (s *Store) Put(ctx context.Context, request objectstore.PutRequest) error {
	headers := map[string]string{}
	if request.ContentType != "" {
		headers["Content-Type"] = request.ContentType
	}

	// UNSIGNED-PAYLOAD, because hashing the body would mean reading the whole file once to sign it
	// and again to send it — see `sigv4.Unsigned`.
	response, err := s.send(
		ctx, http.MethodPut, s.address(request.Bucket, request.Key, nil),
		request.Body, request.Size, sigv4.Unsigned, headers,
	)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (s *Store) CreateUpload(ctx context.Context, bucket, key, contentType string) (string, error) {
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}

	response, err := s.send(
		ctx, http.MethodPost, s.address(bucket, key, map[string]string{"uploads": ""}),
		nil, 0, sigv4.EmptyPayload, headers,
	)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, errorBody))
	if err != nil {
		return "", wire(err)
	}

	var result initiateResult
	if err := xml.Unmarshal(body, &result); err != nil || result.UploadID == "" {
		return "", domain.ServiceError{Code: "unreadable", Message: "the service did not answer with an upload id"}
	}
	return result.UploadID, nil
}

func (s *Store) UploadPart(
	ctx context.Context,
	bucket, key, uploadID string,
	number int,
	body io.Reader,
	size int64,
) (objectstore.Part, error) {
	query := map[string]string{
		"partNumber": strconv.Itoa(number),
		"uploadId":   uploadID,
	}

	response, err := s.send(
		ctx, http.MethodPut, s.address(bucket, key, query), body, size, sigv4.Unsigned, nil,
	)
	if err != nil {
		return objectstore.Part{}, err
	}
	response.Body.Close()

	etag := response.Header.Get("ETag")
	if etag == "" {
		// A part with no tag cannot be named in the completion, so the upload could never finish.
		// Saying so here is better than a completion the service refuses for a reason nobody reads.
		return objectstore.Part{}, domain.ServiceError{
			Code:    "unreadable",
			Message: "the service accepted a part without giving it a tag",
		}
	}
	return objectstore.Part{Number: number, ETag: etag}, nil
}

func (s *Store) CompleteUpload(ctx context.Context, bucket, key, uploadID string, parts []objectstore.Part) error {
	document := completeRequest{Part: make([]completePart, 0, len(parts))}
	for _, part := range parts {
		document.Part = append(document.Part, completePart{PartNumber: part.Number, ETag: part.ETag})
	}

	body, err := xml.Marshal(document)
	if err != nil {
		return err
	}

	response, err := s.send(
		ctx, http.MethodPost, s.address(bucket, key, map[string]string{"uploadId": uploadID}),
		bytes.NewReader(body), int64(len(body)), sigv4.SHA256Hex(string(body)),
		map[string]string{"Content-Type": "application/xml"},
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// A completion is the one request that answers **200 with a failure in the body**: the service
	// holds the connection open while it stitches the parts together, so it has already sent the
	// status by the time it knows. A caller that read the status alone would report a broken object
	// as a finished upload.
	answer, err := io.ReadAll(io.LimitReader(response.Body, errorBody))
	if err != nil {
		return wire(err)
	}
	var failure serviceError
	if xml.Unmarshal(answer, &failure) == nil && failure.Code != "" {
		return domain.ServiceError{Code: failure.Code, Message: failure.Message}
	}
	return nil
}

func (s *Store) AbortUpload(ctx context.Context, bucket, key, uploadID string) error {
	response, err := s.send(
		ctx, http.MethodDelete, s.address(bucket, key, map[string]string{"uploadId": uploadID}),
		nil, 0, sigv4.EmptyPayload, nil,
	)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (s *Store) Delete(ctx context.Context, bucket string, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	if len(keys) > objectstore.DeleteBatch {
		return nil, fmt.Errorf("%w: %d keys in one request", domain.ErrServiceRefused, len(keys))
	}

	document := deleteRequest{Quiet: true, Object: make([]deleteObject, 0, len(keys))}
	for _, key := range keys {
		document.Object = append(document.Object, deleteObject{Key: key})
	}

	body, err := xml.Marshal(document)
	if err != nil {
		return nil, err
	}

	// `Content-MD5` is required on this request and on no other, and it is the base64 of the raw
	// digest rather than hex. A service that has it wrong refuses with a message about the header
	// and not about the keys, which is why it is spelled here and not left to a helper.
	digest := md5.Sum(body)
	headers := map[string]string{
		"Content-Type": "application/xml",
		"Content-MD5":  base64.StdEncoding.EncodeToString(digest[:]),
	}

	response, err := s.send(
		ctx, http.MethodPost, s.address(bucket, "", map[string]string{"delete": ""}),
		bytes.NewReader(body), int64(len(body)), sigv4.SHA256Hex(string(body)), headers,
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	answer, err := io.ReadAll(io.LimitReader(response.Body, errorBody))
	if err != nil {
		return nil, wire(err)
	}

	var result deleteResult
	if xml.Unmarshal(answer, &result) != nil {
		return nil, nil
	}

	refused := make([]string, 0, len(result.Error))
	for _, entry := range result.Error {
		refused = append(refused, entry.Key)
	}
	return refused, nil
}

func (s *Store) Copy(ctx context.Context, bucket, from, toBucket, to string) error {
	// The source is one header and it carries the bucket as well as the key, escaped — a key with a
	// space or a plus in it would otherwise name something else.
	source := "/" + bucket + "/" + strings.TrimPrefix(from, "/")
	headers := map[string]string{"X-Amz-Copy-Source": sigv4.EscapePath(source)}

	response, err := s.send(
		ctx, http.MethodPut, s.address(toBucket, to, nil), nil, 0, sigv4.EmptyPayload, headers,
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// A copy answers 200 with a failure in the body for the reason a completion does: the service
	// is holding the connection open while it moves the bytes.
	answer, err := io.ReadAll(io.LimitReader(response.Body, errorBody))
	if err != nil {
		return wire(err)
	}
	var failure serviceError
	if xml.Unmarshal(answer, &failure) == nil && failure.Code != "" {
		return domain.ServiceError{Code: failure.Code, Message: failure.Message}
	}
	return nil
}

func (s *Store) MakeBucket(ctx context.Context, name string) error {
	var body io.Reader
	size := int64(0)
	payload := sigv4.EmptyPayload

	// us-east-1 is the one region that must **not** be named: the request that carries it is refused
	// there, and the request that omits it is refused everywhere else.
	if s.region != "" && s.region != "us-east-1" {
		document, err := xml.Marshal(createBucketRequest{LocationConstraint: s.region})
		if err != nil {
			return err
		}
		body = bytes.NewReader(document)
		size = int64(len(document))
		payload = sigv4.SHA256Hex(string(document))
	}

	response, err := s.send(ctx, http.MethodPut, s.address(name, "", nil), body, size, payload, nil)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (s *Store) RemoveBucket(ctx context.Context, name string) error {
	response, err := s.send(ctx, http.MethodDelete, s.address(name, "", nil), nil, 0, sigv4.EmptyPayload, nil)
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

// Presign is a URL that reads one object without a credential. **Nothing here is a request**: the
// signature goes in the query, so the URL is made on this machine and handed to whoever needs it.
//
// It is the one thing in this adapter that leaves the app carrying a credential's *authority* rather
// than a credential — which is exactly why the life is short and the caller says how short.
func (s *Store) Presign(ctx context.Context, bucket, key string, life time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	stampedAt, day := sigv4.Stamp(s.now())
	scope := sigv4.Scope(day, s.region, service)

	query := map[string]string{
		"X-Amz-Algorithm":     sigv4.Algorithm,
		"X-Amz-Credential":    s.creds.AccessKeyID + "/" + scope,
		"X-Amz-Date":          stampedAt,
		"X-Amz-Expires":       strconv.Itoa(int(life.Seconds())),
		"X-Amz-SignedHeaders": "host",
	}
	if s.creds.SessionToken != "" {
		query["X-Amz-Security-Token"] = s.creds.SessionToken
	}

	target := s.address(bucket, key, query)

	canonical := strings.Join([]string{
		http.MethodGet,
		sigv4.CanonicalPath(target.EscapedPath()),
		target.RawQuery,
		"host:" + target.Host + "\n",
		"host",
		sigv4.Unsigned,
	}, "\n")

	query["X-Amz-Signature"] = sigv4.Signature(
		s.creds.SecretAccessKey, day, s.region, service, stampedAt, canonical,
	)
	return s.address(bucket, key, query).String(), nil
}
