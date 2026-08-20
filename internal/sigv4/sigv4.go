// Package sigv4 is AWS Signature Version 4, and nothing else. It knows about no service, no
// database and no bucket: it is a documented algorithm over a canonical request, which is why
// **no SDK is taken for it** — the reason `internal/cloudauth` takes none for a token exchange and
// `internal/transfer` takes none for a spreadsheet.
//
// Two callers sign with it and they sign different things. `internal/cloudauth` signs a *URL* whose
// query carries the signature, because an RDS token is a presigned request handed over as a
// password. `internal/objectstore/s3` signs a *request*, because every call to an S3 service carries
// an `Authorization` header. Both go through the same key chain here, so a signature that works in
// one place cannot be spelled differently in the other.
package sigv4

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	Algorithm = "AWS4-HMAC-SHA256"
	// Terminator ends the credential scope. It is a constant of the algorithm and not a version of
	// anything this app has.
	Terminator = "aws4_request"
	// Unsigned is the payload hash a request uses when its body is not hashed. It is what makes an
	// upload of a twenty-gigabyte file possible at all: a signed payload would mean reading the
	// whole file once to hash it and again to send it, and the file may not even fit on the disk
	// twice. The bytes are still protected — by TLS, which is why the endpoint defaults to https.
	Unsigned = "UNSIGNED-PAYLOAD"
	// EmptyPayload is the hash of no bytes at all, which every GET, HEAD and query-signed request
	// carries. It is written out rather than computed so a reader can recognise it in a log.
	EmptyPayload = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// Credentials is an access key as the machine holds it. A session token is present for temporary
// credentials — an assumed role, an SSO login, an instance profile — and empty for a long-lived key.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// Stamp is the two forms of the moment a signature is made at: the full timestamp that goes in the
// request and the day that goes in the scope. They are one function because a signature whose two
// dates disagree is refused with a message about neither.
func Stamp(now time.Time) (stamp, day string) {
	stamp = now.UTC().Format("20060102T150405Z")
	return stamp, stamp[:8]
}

// Scope is what a signature is good for: one day, one region, one service.
func Scope(day, region, service string) string {
	return strings.Join([]string{day, region, service, Terminator}, "/")
}

// Signature is the hex signature of one canonical request. Everything above it — which headers are
// signed, how the path is encoded — is the caller's, because that is where the two callers of this
// package genuinely differ.
func Signature(secret, day, region, service, stamp, canonical string) string {
	toSign := strings.Join([]string{
		Algorithm,
		stamp,
		Scope(day, region, service),
		SHA256Hex(canonical),
	}, "\n")

	return hex.EncodeToString(HMAC(SigningKey(secret, day, region, service), toSign))
}

// SigningKey is SigV4's chain of four HMACs: the secret never signs a request directly, only a key
// that is good for one day, one region and one service. The service is a parameter rather than a
// constant so the chain can be checked against AWS's own published example, which signs for one
// this app never talks to.
func SigningKey(secret, day, region, service string) []byte {
	key := HMAC([]byte("AWS4"+secret), day)
	key = HMAC(key, region)
	key = HMAC(key, service)
	return HMAC(key, Terminator)
}

func HMAC(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func SHA256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// SignRequest puts the `Authorization` header on a request, along with the two headers a signature
// is made over: the moment it was made, and what its body hashes to.
//
// `payload` is the hex hash of the body, `EmptyPayload` for none, or `Unsigned` where the body is
// too large to hash — see Unsigned. It is the caller's because only the caller knows which.
//
// The headers signed are the ones that **have** to be: host, the two above, the session token where
// there is one, and `Content-MD5` and `Content-Type` where the request carries them. Signing every
// header a client happens to add is how a signature comes to depend on a proxy nobody controls.
func SignRequest(
	request *http.Request,
	creds Credentials,
	region, service, payload string,
	now time.Time,
) {
	stamp, day := Stamp(now)

	request.Header.Set("X-Amz-Date", stamp)
	request.Header.Set("X-Amz-Content-Sha256", payload)
	if creds.SessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	signed := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if creds.SessionToken != "" {
		signed = append(signed, "x-amz-security-token")
	}
	for _, header := range []string{"content-md5", "content-type"} {
		if request.Header.Get(header) != "" {
			signed = append(signed, header)
		}
	}
	sort.Strings(signed)

	var headers strings.Builder
	for _, name := range signed {
		value := request.Header.Get(name)
		if name == "host" {
			value = request.Host
			if value == "" {
				value = request.URL.Host
			}
		}
		headers.WriteString(name)
		headers.WriteString(":")
		// A header value is trimmed and its inner runs of spaces collapsed, which is the one piece
		// of canonicalisation a value needs.
		headers.WriteString(strings.Join(strings.Fields(value), " "))
		headers.WriteString("\n")
	}
	names := strings.Join(signed, ";")

	canonical := strings.Join([]string{
		request.Method,
		CanonicalPath(request.URL.EscapedPath()),
		request.URL.RawQuery,
		headers.String(),
		names,
		payload,
	}, "\n")

	signature := Signature(creds.SecretAccessKey, day, region, service, stamp, canonical)

	request.Header.Set("Authorization", Algorithm+
		" Credential="+creds.AccessKeyID+"/"+Scope(day, region, service)+
		", SignedHeaders="+names+
		", Signature="+signature)
}

// CanonicalPath is the path as a signature spells it: already escaped, and never empty. A request
// to the root of a service signs `/`, which is not what an empty path would say.
func CanonicalPath(escaped string) string {
	if escaped == "" {
		return "/"
	}
	return escaped
}

// unreserved is every character RFC 3986 says needs no escaping, and it is the **whole** list. Go's
// own escapers are looser — `net/url` leaves `+`, `:`, `@`, `$` and `,` alone in a path and writes a
// space as `+` in a query — and a signature is computed over the exact characters, so either
// looseness produces a signature the service works out differently and refuses. That is why this
// package escapes for itself rather than calling `url.PathEscape` or `Values.Encode`.
const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"

// EscapePath escapes a path, keeping the separators. Every S3 key goes through it, so a key holding
// a plus, a hash, a space or a newline reaches the service as the key it is.
func EscapePath(path string) string {
	var out strings.Builder
	for at := 0; at < len(path); at++ {
		character := path[at]
		switch {
		case strings.IndexByte(unreserved, character) >= 0:
			out.WriteByte(character)
		case character == '/':
			out.WriteByte('/')
		default:
			out.WriteString(percent(character))
		}
	}
	return out.String()
}

// EscapeAll escapes a whole string, separators included. It is what a query value and a header
// carrying a key — `x-amz-copy-source` — take.
func EscapeAll(value string) string {
	var out strings.Builder
	for at := 0; at < len(value); at++ {
		character := value[at]
		if strings.IndexByte(unreserved, character) >= 0 {
			out.WriteByte(character)
			continue
		}
		out.WriteString(percent(character))
	}
	return out.String()
}

func percent(character byte) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{'%', digits[character>>4], digits[character&0xf]})
}

// EncodeQuery is the canonical query string: every name and value escaped as above, sorted by name,
// joined with `&`. A parameter with no value still carries its `=`, which is what the algorithm
// says and what a service with a strict signer insists on.
func EncodeQuery(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, EscapeAll(name)+"="+EscapeAll(values[name]))
	}
	return strings.Join(parts, "&")
}
