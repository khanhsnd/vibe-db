package cloudauth

import (
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testCredentials() AWSCredentials {
	return AWSCredentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	}
}

func TestAWSTokenIsTheEndpointAndASignedQuery(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 30, 0, 0, time.UTC)
	token := AWSToken(
		testCredentials(),
		"ap-southeast-1", "shop.abc123.ap-southeast-1.rds.amazonaws.com", 3306, "app_user", at,
	)

	const endpoint = "shop.abc123.ap-southeast-1.rds.amazonaws.com:3306/?"
	if !strings.HasPrefix(token, endpoint) {
		t.Fatalf("token = %q, want it to start with %q", token, endpoint)
	}

	query, err := url.ParseQuery(strings.TrimPrefix(token, endpoint))
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}

	want := map[string]string{
		"Action":              "connect",
		"DBUser":              "app_user",
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Date":          "20260804T093000Z",
		"X-Amz-Expires":       "900",
		"X-Amz-SignedHeaders": "host",
		"X-Amz-Credential":    "AKIAIOSFODNN7EXAMPLE/20260804/ap-southeast-1/rds-db/aws4_request",
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}

	// A signature is 32 bytes of SHA-256 written as hex, and nothing else is acceptable: a
	// truncated or empty one is refused by RDS with the same message a wrong password gets.
	signature := query.Get("X-Amz-Signature")
	if len(signature) != 64 {
		t.Errorf("X-Amz-Signature = %q, want 64 hex characters", signature)
	}
	if _, err := hex.DecodeString(signature); err != nil {
		t.Errorf("X-Amz-Signature = %q, which is not hex", signature)
	}

	// No session token means no `X-Amz-Security-Token`: an empty one would be signed over and
	// rejected, rather than simply absent.
	if query.Has("X-Amz-Security-Token") {
		t.Error("a long-lived key must not carry a security token")
	}
}

func TestAWSTokenCarriesATemporaryCredentialsSessionToken(t *testing.T) {
	creds := testCredentials()
	creds.SessionToken = "FQoGZXIvYXdzEExample//////////"

	token := AWSToken(creds, "eu-west-1", "db.example.com", 5432, "app", time.Now())

	if !strings.Contains(token, "X-Amz-Security-Token=") {
		t.Errorf("token = %q, want it to carry the session token", token)
	}
}

// The same inputs sign to the same token, which is what makes the signature checkable at all —
// and different inputs to a different one, which is what makes it a signature.
func TestAWSTokenSignsItsInputs(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 30, 0, 0, time.UTC)
	base := AWSToken(testCredentials(), "eu-west-1", "db.example.com", 5432, "app", at)

	if again := AWSToken(testCredentials(), "eu-west-1", "db.example.com", 5432, "app", at); again != base {
		t.Error("the same inputs signed to two different tokens")
	}

	changed := map[string]string{
		"user":   AWSToken(testCredentials(), "eu-west-1", "db.example.com", 5432, "other", at),
		"host":   AWSToken(testCredentials(), "eu-west-1", "other.example.com", 5432, "app", at),
		"port":   AWSToken(testCredentials(), "eu-west-1", "db.example.com", 3306, "app", at),
		"region": AWSToken(testCredentials(), "us-east-1", "db.example.com", 5432, "app", at),
		"time": AWSToken(
			testCredentials(), "eu-west-1", "db.example.com", 5432, "app", at.Add(time.Minute),
		),
	}
	for what, token := range changed {
		if token == base {
			t.Errorf("changing the %s did not change the token", what)
		}
	}
}

func TestAWSRegionIsReadOffAnRDSEndpoint(t *testing.T) {
	tests := []struct {
		named string
		host  string
		want  string
	}{
		{"", "shop.abc123.ap-southeast-1.rds.amazonaws.com", "ap-southeast-1"},
		{"", "shop.cluster-abc.us-east-2.rds.amazonaws.com", "us-east-2"},
		// What the profile names wins: a proxy or a CNAME in front of RDS carries no region.
		{"eu-west-3", "shop.abc123.ap-southeast-1.rds.amazonaws.com", "eu-west-3"},
		{"eu-west-3", "db.internal", "eu-west-3"},
	}

	for _, tt := range tests {
		if got := AWSRegion(tt.named, tt.host); got != tt.want {
			t.Errorf("AWSRegion(%q, %q) = %q, want %q", tt.named, tt.host, got, tt.want)
		}
	}
}

func TestAWSRegionFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv("AWS_REGION", "sa-east-1")

	if got := AWSRegion("", "db.internal"); got != "sa-east-1" {
		t.Errorf("AWSRegion() = %q, want the environment's region", got)
	}
}

func TestAWSSaysWhatIsMissing(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	_, err := AWS(t.Context(), "eu-west-1", "db.example.com", 5432, "app")
	if err == nil {
		t.Fatal("AWS() = nil error, want a failure naming the missing credentials")
	}
	if !Unavailable(err) {
		t.Errorf("AWS() error = %v, want it to report an unavailable identity", err)
	}
	if !strings.Contains(err.Error(), "AWS_ACCESS_KEY_ID") {
		t.Errorf("AWS() error = %v, want it to name what to set", err)
	}
}

func TestAWSSaysWhenThereIsNoRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	if _, err := AWS(t.Context(), "", "db.internal", 5432, "app"); err == nil {
		t.Error("AWS() = nil error, want a failure naming the missing region")
	}
}
