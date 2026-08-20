package s3

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/sigv4"
)

// asked is what the adapter actually sent, which is the whole of what these tests check: the
// protocol is somebody else's and the only thing worth verifying is that this app speaks it.
type asked struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   string
}

// serve stands a service up and answers every request with `answer`. The endpoint is path style,
// because a virtual-host endpoint would need a wildcard DNS entry that no test can have.
func serve(t *testing.T, answer func(*asked) (int, string)) (*Store, *[]asked) {
	t.Helper()

	var seen []asked

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		record := asked{
			Method: request.Method,
			// The escaped form, because that is what a signature was computed over and what a key
			// with a space in it has to survive as.
			Path:   request.URL.EscapedPath(),
			Query:  request.URL.Query(),
			Header: request.Header.Clone(),
			Body:   string(body),
		}
		seen = append(seen, record)

		status, text := answer(&record)
		writer.Header().Set("Content-Type", "application/xml")
		writer.WriteHeader(status)
		writer.Write([]byte(text))
	}))
	t.Cleanup(server.Close)

	store, err := Open(Options{
		Profile: domain.StoreProfile{
			Provider:  domain.ProviderOther,
			Endpoint:  server.URL,
			Region:    "eu-west-1",
			PathStyle: true,
		},
		Credentials: sigv4.Credentials{AccessKeyID: "key", SecretAccessKey: "secret"},
		Now:         func() time.Time { return time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(store.Close)

	return store, &seen
}

const listing = `<?xml version="1.0"?>
<ListBucketResult>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>token-2</NextContinuationToken>
  <Contents>
    <Key>logs%2F2024%2Fone%20two.log</Key>
    <LastModified>2026-08-19T10:11:12.000Z</LastModified>
    <ETag>&quot;5d41402abc4b2a76b9719d911017c592&quot;</ETag>
    <Size>1234</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
  <CommonPrefixes><Prefix>logs%2F2025%2F</Prefix></CommonPrefixes>
</ListBucketResult>`

func TestListReadsAPageAndDecodesTheKeys(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, listing })

	page, err := store.List(context.Background(), objectstore.ListRequest{
		Bucket:    "shop",
		Prefix:    "logs/",
		Delimiter: "/",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(page.Objects) != 1 || len(page.Folders) != 1 {
		t.Fatalf("List() = %d objects and %d folders, want one of each", len(page.Objects), len(page.Folders))
	}
	// The whole point of `encoding-type=url`: a key with a space in it comes back encoded and has to
	// be handed on decoded, or nothing downstream can address the object.
	if page.Objects[0].Key != "logs/2024/one two.log" {
		t.Errorf("Key = %q, want it decoded", page.Objects[0].Key)
	}
	if page.Folders[0] != "logs/2025/" {
		t.Errorf("Folder = %q, want it decoded", page.Folders[0])
	}
	if page.Objects[0].ETag != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("ETag = %q, want it unquoted", page.Objects[0].ETag)
	}
	if page.Next != "token-2" {
		t.Errorf("Next = %q, want the continuation token", page.Next)
	}

	request := (*seen)[0]
	if request.Path != "/shop" {
		t.Errorf("path = %q, want the bucket in the path on a path-style endpoint", request.Path)
	}
	for name, want := range map[string]string{
		"list-type":     "2",
		"prefix":        "logs/",
		"delimiter":     "/",
		"encoding-type": "url",
	} {
		if got := request.Query.Get(name); got != want {
			t.Errorf("query %s = %q, want %q", name, got, want)
		}
	}
	if request.Header.Get("Authorization") == "" {
		t.Error("the request went unsigned")
	}
}

// A listing that says it is truncated and carries no token would loop for ever, so the token is what
// is read rather than the flag.
func TestListEndsWhereThereIsNoToken(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) {
		return http.StatusOK, `<ListBucketResult><IsTruncated>true</IsTruncated></ListBucketResult>`
	})

	page, err := store.List(context.Background(), objectstore.ListRequest{Bucket: "shop"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page.Next != "" {
		t.Errorf("Next = %q, want empty where the service sent no token", page.Next)
	}
}

func TestOpenAsksForTheRangeInclusively(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusPartialContent, "abc" })

	body, err := store.Open(context.Background(), "shop", "a/b.bin", objectstore.Span{Offset: 0, Length: 8 << 20})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	body.Close()

	if got := (*seen)[0].Header.Get("Range"); got != "bytes=0-8388607" {
		t.Errorf("Range = %q, want the last byte to be inside the range", got)
	}
}

// A service that ignores a range answers 200 with the whole object. Writing that into one part's
// stretch of a file would corrupt everything after it, so it is refused.
func TestOpenRefusesAServiceThatIgnoredTheRange(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) { return http.StatusOK, "the whole object" })

	if _, err := store.Open(context.Background(), "shop", "a", objectstore.Span{Offset: 10, Length: 5}); !errors.Is(err, domain.ErrUnsupported) {
		t.Errorf("Open() error = %v, want ErrUnsupported", err)
	}
}

func TestDeleteCarriesTheDigestAndReportsWhatWasRefused(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) {
		return http.StatusOK, `<DeleteResult><Error><Key>b</Key><Code>AccessDenied</Code></Error></DeleteResult>`
	})

	refused, err := store.Delete(context.Background(), "shop", []string{"a", "b"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(refused) != 1 || refused[0] != "b" {
		t.Errorf("Delete() refused = %v, want [b]", refused)
	}

	request := (*seen)[0]
	if request.Query.Get("delete") != "" || !request.Query.Has("delete") {
		t.Errorf("query = %v, want a bare ?delete", request.Query)
	}
	// The one request in the protocol that requires it, and it has to be base64 of the raw digest of
	// the body that was actually sent — which is what this compares it against.
	digest := md5.Sum([]byte(request.Body))
	if got := request.Header.Get("Content-Md5"); got != base64.StdEncoding.EncodeToString(digest[:]) {
		t.Errorf("Content-MD5 = %q, want the base64 digest of the body it sent", got)
	}
	if !strings.Contains(request.Body, "<Quiet>true</Quiet>") {
		t.Errorf("body = %q, want it to ask to be quiet", request.Body)
	}
}

// A completion answers 200 and puts the failure in the body, because the service has already sent
// the status by the time it knows. Reading the status alone would report a broken object as a
// finished upload — which is the one failure a reader would never find out about.
func TestCompleteUploadReadsAFailureOutOfASuccessfulResponse(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) {
		return http.StatusOK, `<Error><Code>InternalError</Code><Message>we lost a part</Message></Error>`
	})

	err := store.CompleteUpload(context.Background(), "shop", "a", "upload-1", []objectstore.Part{{Number: 1, ETag: "x"}})
	if !errors.Is(err, domain.ErrServiceRefused) {
		t.Fatalf("CompleteUpload() error = %v, want ErrServiceRefused", err)
	}
	if !strings.Contains(err.Error(), "we lost a part") {
		t.Errorf("CompleteUpload() error = %q, want the service's own words", err)
	}
}

func TestCopySendsTheSourceEscapedInAHeader(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, `<CopyObjectResult/>` })

	if err := store.Copy(context.Background(), "shop", "a/one two.txt", "shop", "b/one two.txt"); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := (*seen)[0].Header.Get("X-Amz-Copy-Source"); got != "/shop/a/one%20two.txt" {
		t.Errorf("X-Amz-Copy-Source = %q, want the source escaped", got)
	}
}

func TestErrorsAreMappedByWhatTheServiceSaid(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"a bucket that is not there", http.StatusNotFound, `<Error><Code>NoSuchBucket</Code></Error>`, domain.ErrUnknownBucket},
		{"an object that is not there", http.StatusNotFound, `<Error><Code>NoSuchKey</Code></Error>`, domain.ErrUnknownObject},
		{"a key that is wrong", http.StatusForbidden, `<Error><Code>SignatureDoesNotMatch</Code></Error>`, domain.ErrBadCredentials},
		{"a key that is not allowed", http.StatusForbidden, `<Error><Code>AccessDenied</Code></Error>`, domain.ErrForbidden},
		{"anything else", http.StatusBadRequest, `<Error><Code>InvalidArgument</Code><Message>no</Message></Error>`, domain.ErrServiceRefused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := serve(t, func(*asked) (int, string) { return tt.status, tt.body })

			_, err := store.List(context.Background(), objectstore.ListRequest{Bucket: "shop"})
			if !errors.Is(err, tt.want) {
				t.Errorf("List() error = %v, want %v", err, tt.want)
			}
		})
	}
}

// A service that is unwell or asking to be slowed down is worth another attempt, and that is the
// adapter's to say — nothing above it can read a status code.
func TestATiredServiceIsWorthAnotherAttempt(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError} {
		store, _ := serve(t, func(*asked) (int, string) { return status, `<Error><Code>SlowDown</Code></Error>` })

		_, err := store.List(context.Background(), objectstore.ListRequest{Bucket: "shop"})
		if !objectstore.Temporary(err) {
			t.Errorf("status %d: Temporary(%v) = false, want true", status, err)
		}
	}
}

// A refusal with an empty body is a service behind a proxy that swallowed it. The status still says
// what happened, and losing that would leave the reader with nothing at all.
func TestARefusalWithNoBodyStillSaysWhatHappened(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) { return http.StatusNotFound, "" })

	_, err := store.Head(context.Background(), "shop", "a")
	if !errors.Is(err, domain.ErrUnknownObject) {
		t.Errorf("Head() error = %v, want ErrUnknownObject", err)
	}
}

func TestPresignSignsInTheQueryAndSendsNothing(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, "" })

	link, err := store.Presign(context.Background(), "shop", "a/one two.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("Presign() made %d requests, want none", len(*seen))
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("Presign() gave an unparseable URL: %v", err)
	}
	if !strings.Contains(parsed.EscapedPath(), "one%20two.txt") {
		t.Errorf("path = %q, want the key escaped", parsed.EscapedPath())
	}
	for _, name := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-Expires", "X-Amz-Signature"} {
		if parsed.Query().Get(name) == "" {
			t.Errorf("query is missing %s", name)
		}
	}
	if got := parsed.Query().Get("X-Amz-Expires"); got != "900" {
		t.Errorf("X-Amz-Expires = %q, want 900", got)
	}
}

// The bucket goes in the host or in the path, and it is one flag. Getting it the wrong way round
// against a MinIO in a container is a 404 with nothing in it, which is why it is worth a test.
func TestVirtualHostStyleNamesTheBucketInTheHost(t *testing.T) {
	store, err := Open(Options{
		Profile: domain.StoreProfile{
			Provider: domain.ProviderAWS,
			Endpoint: "s3.eu-west-1.amazonaws.com",
			Region:   "eu-west-1",
		},
		Credentials: sigv4.Credentials{AccessKeyID: "key", SecretAccessKey: "secret"},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	target := store.address("shop", "a/b.txt", nil)
	if target.Host != "shop.s3.eu-west-1.amazonaws.com" {
		t.Errorf("host = %q, want the bucket as a label of it", target.Host)
	}
	if target.Path != "/a/b.txt" {
		t.Errorf("path = %q, want the key alone", target.Path)
	}
}
