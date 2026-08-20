package s3

import (
	"encoding/xml"
	"io"
	"net/http"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/neterr"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
)

// errorBody is how much of a refusal is read. A service answers a refusal with a small XML
// document, so this is generous; it is a bound rather than a whole read because a misconfigured
// endpoint answers a refusal with somebody's HTML error page, and that page can be a megabyte.
const errorBody = 64 << 10

// refusal turns a response the service refused with into the error the app switches on. It is this
// adapter's `errors.go` and it does the same job every driver adapter's does: the service's own
// word for the failure becomes a `domain` sentinel, and everything it does not recognise keeps the
// service's own sentence.
//
// The **status** decides first and the code refines it. That order matters: a service that answers
// 404 with an empty body is still telling the reader the object is not there, and a code this app
// has never heard of must not lose that.
func refusal(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, errorBody))
	response.Body.Close()

	var failure serviceError
	if err := xml.Unmarshal(body, &failure); err != nil {
		failure = serviceError{}
	}

	// A service asking to be slowed down, or one that is briefly unwell, is worth asking again —
	// which is the whole of what a large transfer needs to survive a busy afternoon. It wraps
	// whatever the failure turns out to be, so the reason is not lost by being retried.
	retryable := response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= http.StatusInternalServerError ||
		failure.Code == "SlowDown" ||
		failure.Code == "RequestTimeout"

	err := byStatus(response.StatusCode, failure)
	if retryable {
		return objectstore.Retryable{Err: err}
	}
	return err
}

func byStatus(status int, failure serviceError) error {
	switch status {
	case http.StatusNotFound:
		switch failure.Code {
		case "NoSuchBucket":
			return domain.ErrUnknownBucket
		case "NoSuchUpload":
			// An upload whose parts the service has forgotten. It keeps its own words: the reader's
			// answer is to start it again, and no sentinel of ours says that better.
			return serviceRefusal(failure, "the multipart upload is no longer there")
		default:
			return domain.ErrUnknownObject
		}

	case http.StatusForbidden:
		switch failure.Code {
		case "SignatureDoesNotMatch", "InvalidAccessKeyId", "InvalidSecurity", "TokenRefreshRequired":
			// The key itself is wrong, which is the same sentence a database says about a password.
			return domain.ErrBadCredentials
		case "RequestTimeTooSkewed":
			// The signature was made too long ago or too far in the future, which is nearly always
			// this machine's clock. It keeps the service's own words because they say so.
			return serviceRefusal(failure, "this machine's clock is too far from the service's")
		default:
			// A key that is good and a bucket that is not its business. It is deliberately not
			// BAD_CREDENTIALS: telling a reader to check their password when their password is fine
			// sends them to the wrong box.
			return domain.ErrForbidden
		}

	case http.StatusUnauthorized:
		return domain.ErrBadCredentials

	case http.StatusNotImplemented:
		// The honest answer for a service that speaks most of the protocol: something this app asked
		// for is not part of what it implements.
		return domain.Unsupported(what(failure, "that request"))

	default:
		return serviceRefusal(failure, "the service refused the request")
	}
}

// serviceRefusal is the service's own code and sentence, with a fallback for the service that
// refuses with nothing in the body at all — which a misconfigured proxy in front of one does.
func serviceRefusal(failure serviceError, fallback string) error {
	message := failure.Message
	if message == "" {
		message = fallback
	}
	code := failure.Code
	if code == "" {
		code = "unknown"
	}
	return domain.ServiceError{Code: code, Message: message}
}

func what(failure serviceError, fallback string) string {
	if failure.Code != "" {
		return failure.Code
	}
	return fallback
}

// wire is the failure of a request that never got an answer: a host that is not there, a deadline, a
// certificate this machine will not trust. It is `neterr.Map` and a fallback, exactly as every
// driver adapter's error map opens with — and what it does not recognise is `ErrHostUnreachable`,
// because a request to a service that answered nothing is a service that was not reached.
func wire(err error) error {
	if mapped := neterr.Map(err); mapped != nil {
		return mapped
	}
	// A connection that dropped part way through is worth another attempt; the reader's own Stop is
	// not, and `neterr.Map` has already claimed that one.
	return objectstore.Retryable{Err: err}
}
