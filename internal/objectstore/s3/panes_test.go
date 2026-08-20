package s3

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
)

// The four reads the object panel is drawn from. They are tested here rather than only in the live
// run because what each of them *is* is a shape to parse: an XML document with a namespaced
// attribute, a header map with a prefix in it, and two lists that have to be put back into one
// order. A live service proves the signature; these prove the reading.

const aclBody = `<?xml version="1.0"?>
<AccessControlPolicy>
  <Owner><ID>abc123</ID><DisplayName>khanh</DisplayName></Owner>
  <AccessControlList>
    <Grant>
      <Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="CanonicalUser">
        <ID>abc123</ID><DisplayName>khanh</DisplayName>
      </Grantee>
      <Permission>FULL_CONTROL</Permission>
    </Grant>
    <Grant>
      <Grantee xsi:type="Group"><URI>http://acs.amazonaws.com/groups/global/AllUsers</URI></Grantee>
      <Permission>READ</Permission>
    </Grant>
  </AccessControlList>
</AccessControlPolicy>`

// A grantee is told apart by **which child it carries** and never by its `xsi:type` attribute: the
// attribute is namespaced, and a service that spells the namespace differently would otherwise read
// as an empty grantee — which is what the second grant here has.
func TestPermissionsReadTheGranteeOffItsChild(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, aclBody })

	held, err := store.ObjectPermissions(context.Background(), "shop", "a.txt")
	if err != nil {
		t.Fatalf("ObjectPermissions() error = %v", err)
	}

	if held.Owner != "khanh" {
		t.Errorf("Owner = %q, want the display name in preference to the id", held.Owner)
	}
	if len(held.Grants) != 2 {
		t.Fatalf("Grants = %v, want two", held.Grants)
	}
	if held.Grants[0].Kind != "CanonicalUser" || held.Grants[0].Grantee != "khanh" {
		t.Errorf("the first grant = %+v, want a named canonical user", held.Grants[0])
	}
	// The one that matters: a URI grantee is a **group**, and it is named by its last segment rather
	// than shown as a URL nobody reads.
	if held.Grants[1].Kind != "Group" || held.Grants[1].Grantee != "AllUsers" {
		t.Errorf("the second grant = %+v, want the group named AllUsers", held.Grants[1])
	}

	if !(*seen)[0].Query.Has("acl") {
		t.Errorf("query = %v, want a bare ?acl", (*seen)[0].Query)
	}
}

// A service that has not got ACLs at all answers its own refusal, and it has to reach the reader as
// one: an empty table would read as "nobody may do anything", which is a different and wrong
// sentence.
func TestPermissionsPassTheServicesRefusalOn(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) {
		return http.StatusNotImplemented, `<Error><Code>NotImplemented</Code></Error>`
	})

	_, err := store.ObjectPermissions(context.Background(), "shop", "a.txt")
	if !errors.Is(err, domain.ErrUnsupported) {
		t.Errorf("ObjectPermissions() error = %v, want ErrUnsupported naming it", err)
	}
}

func TestTagsAreReadAsPairs(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) {
		return http.StatusOK, `<Tagging><TagSet>
			<Tag><Key>env</Key><Value>production</Value></Tag>
			<Tag><Key>owner</Key><Value>platform</Value></Tag>
		</TagSet></Tagging>`
	})

	tags, err := store.ObjectTags(context.Background(), "shop", "a.txt")
	if err != nil {
		t.Fatalf("ObjectTags() error = %v", err)
	}
	if len(tags) != 2 || tags[0].Name != "env" || tags[0].Value != "production" {
		t.Errorf("ObjectTags() = %v, want the two pairs", tags)
	}
	if !(*seen)[0].Query.Has("tagging") {
		t.Errorf("query = %v, want a bare ?tagging", (*seen)[0].Query)
	}
}

const versionsBody = `<?xml version="1.0"?>
<ListVersionsResult>
  <Version>
    <Key>report.csv</Key><VersionId>v2</VersionId><IsLatest>true</IsLatest>
    <LastModified>2026-08-20T10:00:00.000Z</LastModified>
    <ETag>&quot;aaa&quot;</ETag><Size>200</Size><StorageClass>STANDARD</StorageClass>
  </Version>
  <Version>
    <Key>report.csv</Key><VersionId>v1</VersionId><IsLatest>false</IsLatest>
    <LastModified>2026-08-18T10:00:00.000Z</LastModified>
    <ETag>&quot;bbb&quot;</ETag><Size>100</Size><StorageClass>STANDARD</StorageClass>
  </Version>
  <Version>
    <Key>report.csv.bak</Key><VersionId>x1</VersionId><IsLatest>true</IsLatest>
    <LastModified>2026-08-19T10:00:00.000Z</LastModified>
    <ETag>&quot;ccc&quot;</ETag><Size>50</Size>
  </Version>
  <DeleteMarker>
    <Key>report.csv</Key><VersionId>v3</VersionId><IsLatest>false</IsLatest>
    <LastModified>2026-08-21T10:00:00.000Z</LastModified>
  </DeleteMarker>
</ListVersionsResult>`

// Two things at once, and both are the shape of the protocol rather than a choice: a versions listing
// answers about a **prefix**, so a neighbouring key has to be dropped; and a delete marker arrives in
// a second list, so one order has to be put back over both.
func TestVersionsKeepOneKeyAndOneOrder(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, versionsBody })

	versions, err := store.ObjectVersions(context.Background(), "shop", "report.csv", 10)
	if err != nil {
		t.Fatalf("ObjectVersions() error = %v", err)
	}

	if len(versions) != 3 {
		t.Fatalf("ObjectVersions() = %d, want the three that are report.csv and not report.csv.bak", len(versions))
	}

	// Newest first, across both lists — the delete marker is the newest thing here.
	if !versions[0].Deleted || versions[0].VersionID != "v3" {
		t.Errorf("the first = %+v, want the delete marker, being the newest", versions[0])
	}
	if versions[1].VersionID != "v2" || !versions[1].Latest {
		t.Errorf("the second = %+v, want v2, the latest version with bytes", versions[1])
	}
	if versions[2].VersionID != "v1" || versions[2].ETag != "bbb" {
		t.Errorf("the third = %+v, want v1 with its tag unquoted", versions[2])
	}

	request := (*seen)[0]
	if !request.Query.Has("versions") || request.Query.Get("prefix") != "report.csv" {
		t.Errorf("query = %v, want ?versions with the key as the prefix", request.Query)
	}
}

// The head is what **two** panes are drawn from — Properties and Header — so it has to hand back both
// readings of one answer: the whole of what the service said, and the uploader's own facts with the
// protocol's prefix taken off.
func TestHeadCarriesTheWholeAnswerAndTheUploadersOwnFacts(t *testing.T) {
	store, _ := serve(t, func(record *asked) (int, string) {
		record.Header.Set("ignored", "the recorder's own copy")
		return http.StatusOK, ""
	})

	// The fake service answers with whatever headers the handler set, so they are set here.
	store.client.Transport = &stubTransport{header: http.Header{
		"Content-Type":           {"text/csv"},
		"Content-Length":         {"23"},
		"Etag":                   {`"5d41402abc4b2a76b9719d911017c592"`},
		"X-Amz-Meta-Uploaded-By": {"nightly-job"},
		"X-Amz-Meta-Batch":       {"2026-08-20"},
		"Cache-Control":          {"max-age=60"},
	}}

	object, err := store.Head(context.Background(), "shop", "report.csv")
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}

	if object.ContentType != "text/csv" {
		t.Errorf("ContentType = %q", object.ContentType)
	}
	if object.Metadata["Uploaded-By"] != "nightly-job" || object.Metadata["Batch"] != "2026-08-20" {
		t.Errorf("Metadata = %v, want the two with the x-amz-meta- prefix taken off", object.Metadata)
	}

	// Sorted by name, because the transport hands them over as a map and the order they were sent in
	// is gone — and *every* header is there, including the ones nothing parses.
	names := make([]string, 0, len(object.Headers))
	for _, header := range object.Headers {
		names = append(names, header.Name)
	}
	for at := 1; at < len(names); at++ {
		if names[at-1] > names[at] {
			t.Fatalf("Headers are not sorted: %v", names)
		}
	}
	if !holds(names, "Cache-Control") || !holds(names, "X-Amz-Meta-Batch") {
		t.Errorf("Headers = %v, want the whole answer including what nothing parses", names)
	}
}

func holds(values []string, want string) bool {
	for _, one := range values {
		if one == want {
			return true
		}
	}
	return false
}

// stubTransport answers every request with one set of headers and no body, which is what a `HEAD` is:
// `httptest` cannot set a response header from the handler *and* be read as a HEAD by the client, so
// the transport is the honest place to put them.
type stubTransport struct {
	header http.Header
}

func (t *stubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	header := http.Header{}
	for name, values := range t.header {
		header[name] = values
	}

	length, _ := strconv.ParseInt(header.Get("Content-Length"), 10, 64)

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        header,
		Body:          http.NoBody,
		ContentLength: length,
		Request:       request,
	}, nil
}

// Changing a header is a **copy of the object onto itself**, and every part of that request is easy
// to get wrong in a way no reader would notice until a header quietly vanished. So all of it is
// checked: the directive, the source, which headers are sent, which are dropped, and the class that
// has to be carried or the object moves to the service's default.
func TestWriteMetadataIsACopyOntoItself(t *testing.T) {
	store, seen := serve(t, func(*asked) (int, string) { return http.StatusOK, `<CopyObjectResult/>` })

	err := store.WriteMetadata(context.Background(), "shop", "a/report.csv", objectstore.Metadata{
		System: map[string]string{
			"Content-Type":  "text/csv",
			"Cache-Control": "max-age=60",
			// Not one of the six: there is no request that writes it, so it must not be sent.
			"ETag": "pretend",
		},
		User:         map[string]string{"Uploaded-By": "nightly-job"},
		StorageClass: "STANDARD_IA",
	})
	if err != nil {
		t.Fatalf("WriteMetadata() error = %v", err)
	}

	request := (*seen)[0]
	if request.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", request.Method)
	}
	if got := request.Header.Get("X-Amz-Copy-Source"); got != "/shop/a/report.csv" {
		t.Errorf("X-Amz-Copy-Source = %q, want the object itself", got)
	}
	if got := request.Header.Get("X-Amz-Metadata-Directive"); got != "REPLACE" {
		t.Errorf("X-Amz-Metadata-Directive = %q, want REPLACE — without it the copy keeps the old set", got)
	}
	if got := request.Header.Get("Content-Type"); got != "text/csv" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := request.Header.Get("Cache-Control"); got != "max-age=60" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := request.Header.Get("X-Amz-Meta-Uploaded-By"); got != "nightly-job" {
		t.Errorf("X-Amz-Meta-Uploaded-By = %q, want the pair with the prefix put back on", got)
	}
	// The class is carried, or the copy silently moves the object to the default one.
	if got := request.Header.Get("X-Amz-Storage-Class"); got != "STANDARD_IA" {
		t.Errorf("X-Amz-Storage-Class = %q, want the class carried through", got)
	}
	// A header nothing can write is dropped rather than sent and ignored.
	if got := request.Header.Get("ETag"); got != "" {
		t.Errorf("ETag = %q, want it dropped: no request writes it", got)
	}
}

// A copy answers 200 and puts the failure in the body — the service is holding the connection open
// while it moves the bytes. Reading the status alone would report a header that never changed as
// changed, and the pane would then re-read and show the old value with no explanation.
func TestWriteMetadataReadsAFailureOutOfASuccessfulResponse(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) {
		return http.StatusOK, `<Error><Code>InvalidRequest</Code><Message>metadata too large</Message></Error>`
	})

	err := store.WriteMetadata(context.Background(), "shop", "a", objectstore.Metadata{})
	if !errors.Is(err, domain.ErrServiceRefused) {
		t.Fatalf("WriteMetadata() error = %v, want ErrServiceRefused", err)
	}
	if !strings.Contains(err.Error(), "metadata too large") {
		t.Errorf("WriteMetadata() error = %q, want the service's own words", err)
	}
}

// The three panes are reads, so a read-only connection still answers them. It is asserted here
// because the guard is a wrapper that *embeds* the store: a method added to the port and forgotten in
// the wrapper would silently become a write nobody refused, and the opposite mistake — refusing a
// read — would take a pane away for no reason.
func TestThePanesAreReads(t *testing.T) {
	store, _ := serve(t, func(*asked) (int, string) { return http.StatusOK, `<Tagging><TagSet/></Tagging>` })

	var port objectstore.Store = store
	if _, err := port.ObjectTags(context.Background(), "shop", "a"); err != nil {
		t.Errorf("ObjectTags() through the port error = %v", err)
	}
}
