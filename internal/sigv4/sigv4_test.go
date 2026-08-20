package sigv4

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The published example from AWS's own Signature Version 4 documentation. It signs for a service
// neither caller of this package talks to, which is the whole reason SigningKey takes one: the
// derivation is what this checks, and the derivation is what everything else rests on.
func TestSigningKeyMatchesTheDocumentedExample(t *testing.T) {
	const (
		secret = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
		want   = "c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9"
	)

	got := hex.EncodeToString(SigningKey(secret, "20150830", "us-east-1", "iam"))
	if got != want {
		t.Errorf("SigningKey() = %s, want %s", got, want)
	}
}

func TestEmptyPayloadIsTheHashOfNothing(t *testing.T) {
	if got := SHA256Hex(""); got != EmptyPayload {
		t.Errorf("SHA256Hex(\"\") = %s, want the EmptyPayload constant %s", got, EmptyPayload)
	}
}

func testCredentials() Credentials {
	return Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	}
}

func signed(t *testing.T, creds Credentials, method, url string, headers map[string]string) *http.Request {
	t.Helper()

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	SignRequest(request, creds, "eu-west-1", "s3", EmptyPayload, time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC))
	return request
}

func TestSignRequestSignsWhatItSaysItSigns(t *testing.T) {
	request := signed(t, testCredentials(), http.MethodGet, "https://bucket.s3.eu-west-1.amazonaws.com/a/b.txt", nil)

	authorization := request.Header.Get("Authorization")
	for _, part := range []string{
		Algorithm,
		"Credential=AKIAIOSFODNN7EXAMPLE/20260820/eu-west-1/s3/aws4_request",
		"SignedHeaders=host;x-amz-content-sha256;x-amz-date",
		"Signature=",
	} {
		if !strings.Contains(authorization, part) {
			t.Errorf("Authorization = %q, want it to hold %q", authorization, part)
		}
	}

	if got := request.Header.Get("X-Amz-Date"); got != "20260820T090000Z" {
		t.Errorf("X-Amz-Date = %q, want the stamp the signature was made at", got)
	}
	if got := request.Header.Get("X-Amz-Content-Sha256"); got != EmptyPayload {
		t.Errorf("X-Amz-Content-Sha256 = %q, want the payload hash it was signed with", got)
	}
}

// A session token has to be *signed* and not merely sent: a temporary credential whose token was
// added after the signature is a request every service refuses.
func TestSignRequestSignsASessionToken(t *testing.T) {
	creds := testCredentials()
	creds.SessionToken = "FQoGZXIvYXdzEExample"

	request := signed(t, creds, http.MethodGet, "https://bucket.s3.eu-west-1.amazonaws.com/a", nil)

	if got := request.Header.Get("X-Amz-Security-Token"); got != creds.SessionToken {
		t.Errorf("X-Amz-Security-Token = %q, want the token", got)
	}
	if !strings.Contains(request.Header.Get("Authorization"), "x-amz-security-token") {
		t.Error("Authorization does not sign x-amz-security-token")
	}
}

// The two headers a service checks for itself have to be in the signature where they are present,
// and must not appear in it where they are not — a signed header that was never sent is refused
// exactly as loudly as one that was sent unsigned.
func TestSignRequestSignsTheBodyHeadersOnlyWhenThereAreAny(t *testing.T) {
	with := signed(t, testCredentials(), http.MethodPut, "https://bucket.s3.eu-west-1.amazonaws.com/a", map[string]string{
		"Content-Type": "text/csv",
		"Content-MD5":  "1B2M2Y8AsgTpgAmY7PhCfg==",
	})
	if !strings.Contains(with.Header.Get("Authorization"), "SignedHeaders=content-md5;content-type;host;x-amz-content-sha256;x-amz-date") {
		t.Errorf("Authorization = %q, want both body headers signed in order", with.Header.Get("Authorization"))
	}

	without := signed(t, testCredentials(), http.MethodPut, "https://bucket.s3.eu-west-1.amazonaws.com/a", nil)
	if strings.Contains(without.Header.Get("Authorization"), "content-type") {
		t.Errorf("Authorization = %q, want no content-type signed", without.Header.Get("Authorization"))
	}
}

// A signature over a path this app did not escape is a signature over somebody else's path. The
// canonical form is the *escaped* one, and a request to the root of a service signs a slash rather
// than nothing at all.
func TestCanonicalPathIsNeverEmpty(t *testing.T) {
	if got := CanonicalPath(""); got != "/" {
		t.Errorf("CanonicalPath(\"\") = %q, want \"/\"", got)
	}
	if got := CanonicalPath("/a%20b/c"); got != "/a%20b/c" {
		t.Errorf("CanonicalPath() = %q, want the escaped path unchanged", got)
	}
}
