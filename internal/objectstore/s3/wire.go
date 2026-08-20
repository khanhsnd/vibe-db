package s3

import (
	"encoding/xml"
	"net/url"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/objectstore"
)

// The answers an S3 service gives, as the documented XML. They are written out here rather than
// generated, because the whole of what this app asks a service is on this page: nine requests and
// six answers. **No SDK is taken**, for the reason `internal/transfer` takes none for a spreadsheet
// and `internal/sigv4` takes none for a signature — a documented format read by hand is a few
// hundred lines, and an SDK is fifteen megabytes of binary and a dependency tree.

// listBucketsResult is `GET /`.
type listBucketsResult struct {
	Buckets struct {
		Bucket []struct {
			Name         string `xml:"Name"`
			CreationDate string `xml:"CreationDate"`
		} `xml:"Bucket"`
	} `xml:"Buckets"`
}

// listObjectsResult is `GET /bucket?list-type=2`, which is ListObjectsV2. The keys come back **URL
// encoded**, because the request asks for `encoding-type=url` — without it a key holding a newline
// or a control character is not representable in the XML at all, and such keys exist.
type listObjectsResult struct {
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
	Contents              []struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	} `xml:"Contents"`
	CommonPrefixes []struct {
		Prefix string `xml:"Prefix"`
	} `xml:"CommonPrefixes"`
}

// deleteResult is what `POST /bucket?delete` answers. The request asks to be quiet, so the only
// thing in here is what the service **would not** remove — which is why a partial failure is not an
// error: it is this list.
type deleteResult struct {
	Error []struct {
		Key     string `xml:"Key"`
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
}

// deleteRequest is the body of that request.
type deleteRequest struct {
	XMLName xml.Name       `xml:"Delete"`
	Quiet   bool           `xml:"Quiet"`
	Object  []deleteObject `xml:"Object"`
}

type deleteObject struct {
	Key string `xml:"Key"`
}

// initiateResult is `POST /bucket/key?uploads`.
type initiateResult struct {
	UploadID string `xml:"UploadId"`
}

// completeRequest is the body of `POST /bucket/key?uploadId=…`, and the parts have to be in order.
type completeRequest struct {
	XMLName xml.Name       `xml:"CompleteMultipartUpload"`
	Part    []completePart `xml:"Part"`
}

type completePart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// createBucketRequest is the body of `PUT /bucket`, which every region but us-east-1 needs and
// which us-east-1 refuses.
type createBucketRequest struct {
	XMLName            xml.Name `xml:"CreateBucketConfiguration"`
	LocationConstraint string   `xml:"LocationConstraint"`
}

// aclResult is `GET /bucket/key?acl`. A grantee is told apart by **which child it carries** rather
// than by its `xsi:type` attribute: the attribute is namespaced, and a service that spells the
// namespace differently would otherwise read as an empty grantee.
type aclResult struct {
	Owner struct {
		ID          string `xml:"ID"`
		DisplayName string `xml:"DisplayName"`
	} `xml:"Owner"`
	Grants []struct {
		Grantee struct {
			ID           string `xml:"ID"`
			DisplayName  string `xml:"DisplayName"`
			URI          string `xml:"URI"`
			EmailAddress string `xml:"EmailAddress"`
		} `xml:"Grantee"`
		Permission string `xml:"Permission"`
	} `xml:"AccessControlList>Grant"`
}

// taggingResult is `GET /bucket/key?tagging`.
type taggingResult struct {
	Tags []struct {
		Key   string `xml:"Key"`
		Value string `xml:"Value"`
	} `xml:"TagSet>Tag"`
}

// versionsResult is `GET /bucket?versions`. A **delete marker** is a second element rather than a
// flag on the first, which is the protocol saying what it is: a version whose whole content is the
// fact that the object was deleted at that moment.
type versionsResult struct {
	Version []struct {
		Key          string `xml:"Key"`
		VersionID    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	} `xml:"Version"`
	DeleteMarker []struct {
		Key          string `xml:"Key"`
		VersionID    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		LastModified string `xml:"LastModified"`
	} `xml:"DeleteMarker"`
}

// serviceError is the body a service puts a refusal in. `Code` is the word this app switches on and
// `Message` is what the reader is shown, for the reason a rejected statement's own message reaches
// the UI verbatim: the service knows why it refused and this app does not.
type serviceError struct {
	Code      string `xml:"Code"`
	Message   string `xml:"Message"`
	Resource  string `xml:"Resource"`
	RequestID string `xml:"RequestId"`
}

// objectsOf turns one listing into the port's own shape. It is the one place a wire answer becomes
// a domain one, so nothing above here has heard of XML.
func objectsOf(result listObjectsResult) objectstore.Listing {
	listing := objectstore.Listing{
		Folders: make([]string, 0, len(result.CommonPrefixes)),
		Objects: make([]objectstore.Object, 0, len(result.Contents)),
	}

	for _, prefix := range result.CommonPrefixes {
		listing.Folders = append(listing.Folders, decodeKey(prefix.Prefix))
	}
	for _, entry := range result.Contents {
		listing.Objects = append(listing.Objects, objectstore.Object{
			Key:          decodeKey(entry.Key),
			Size:         entry.Size,
			Modified:     stamp(entry.LastModified),
			ETag:         strings.Trim(entry.ETag, `"`),
			StorageClass: entry.StorageClass,
		})
	}

	// A truncated listing always carries a token; one that does not is complete, whatever the flag
	// said. Reading the token rather than the flag is what keeps a walk from looping for ever on a
	// service that sets one and not the other.
	if result.IsTruncated {
		listing.Next = result.NextContinuationToken
	}
	return listing
}

// decodeKey undoes the `encoding-type=url` the listing asked for. A key that does not decode is
// handed back as it came: a service that ignored the parameter is answering with plain keys, and
// refusing them would be refusing the whole bucket over a parameter.
func decodeKey(key string) string {
	decoded, err := url.QueryUnescape(key)
	if err != nil {
		return key
	}
	return decoded
}

// stamp reads the two forms a service writes a time in: the ISO 8601 of a listing and the HTTP date
// of a `Last-Modified` header. A time that reads as neither is the zero one, which the grid draws as
// empty rather than as 1970.
func stamp(text string) time.Time {
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123, http1123} {
		if when, err := time.Parse(layout, text); err == nil {
			return when.UTC()
		}
	}
	return time.Time{}
}

// http1123 is RFC 1123 with GMT spelled out, which is the form an HTTP header actually uses and
// which `time.RFC1123` does not parse.
const http1123 = "Mon, 02 Jan 2006 15:04:05 GMT"
