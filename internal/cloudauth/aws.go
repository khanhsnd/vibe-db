// Package cloudauth mints the password for a connection that authenticates by a cloud identity
// rather than by something the reader knows. Both kinds it produces are short-lived tokens, which
// is why nothing here is ever stored: a credential good for a quarter of an hour is not a secret
// worth keeping, and one read back out of a keychain an hour later would only fail.
package cloudauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// awsService is the service an RDS IAM token is signed for. It is not `rds`: the token
	// authenticates a *database* connection, and the control plane and the data plane are signed
	// for separately.
	awsService = "rds-db"
	awsAction  = "connect"
	// awsTokenLife is how long a signed token is accepted for. Fifteen minutes is the longest
	// RDS allows, and it bounds only the *start* of a connection — a session already open is
	// unaffected when its token expires.
	awsTokenLife = 15 * time.Minute
)

// AWSCredentials is an access key as the machine holds it. A session token is present for
// temporary credentials — an assumed role, an SSO login, an instance profile — and empty for a
// long-lived key.
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// AWSEnvironment reads the credentials the AWS tools all agree on. It is the environment alone:
// resolving a named profile, an SSO cache or an instance metadata endpoint is the SDK's job, and
// pulling that in to read three variables would be a great deal of dependency for very little.
// A reader who has run `aws sso login` or set a profile exports them with `aws configure export-
// credentials`, and that is what the dialog says.
func AWSEnvironment() (AWSCredentials, bool) {
	creds := AWSCredentials{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
	}
	return creds, creds.AccessKeyID != "" && creds.SecretAccessKey != ""
}

// AWSRegion is the region to sign for: the profile's own, or the one the endpoint already carries.
// An RDS endpoint is `name.hash.region.rds.amazonaws.com`, so the region is the third-from-last
// label before `amazonaws.com` — and where it is not an RDS endpoint at all, the environment's.
func AWSRegion(region, host string) string {
	if region != "" {
		return region
	}

	labels := strings.Split(host, ".")
	if len(labels) >= 5 && strings.HasSuffix(host, ".rds.amazonaws.com") {
		return labels[len(labels)-4]
	}
	if fromEnv := os.Getenv("AWS_REGION"); fromEnv != "" {
		return fromEnv
	}
	return os.Getenv("AWS_DEFAULT_REGION")
}

// AWSToken is the password an RDS instance with IAM authentication takes. It is not a secret the
// server holds a copy of: it is a signed request to connect as `user`, which RDS verifies against
// IAM, so the whole of what makes it work is the signature — and an unencrypted connection would
// hand a valid one to anybody listening. `now` is a parameter so the signature is testable; every
// caller passes time.Now().
//
// The shape is AWS Signature Version 4 in its presigned-URL form, and the token is that URL with
// its scheme removed.
func AWSToken(
	creds AWSCredentials,
	region, host string,
	port int,
	user string,
	now time.Time,
) string {
	stamp := now.UTC().Format("20060102T150405Z")
	day := stamp[:8]
	scope := strings.Join([]string{day, region, awsService, "aws4_request"}, "/")

	endpoint := host + ":" + strconv.Itoa(port)

	// The query is signed in sorted order, which is what url.Values.Encode already does.
	query := url.Values{
		"Action":              {awsAction},
		"DBUser":              {user},
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {creds.AccessKeyID + "/" + scope},
		"X-Amz-Date":          {stamp},
		"X-Amz-Expires":       {strconv.Itoa(int(awsTokenLife.Seconds()))},
		"X-Amz-SignedHeaders": {"host"},
	}
	if creds.SessionToken != "" {
		query.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	canonical := strings.Join([]string{
		"GET",
		"/",
		query.Encode(),
		"host:" + endpoint + "\n",
		"host",
		sha256Hex(""),
	}, "\n")

	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		stamp,
		scope,
		sha256Hex(canonical),
	}, "\n")

	key := signingKey(creds.SecretAccessKey, day, region, awsService)
	query.Set("X-Amz-Signature", hex.EncodeToString(hmacSHA256(key, toSign)))

	return endpoint + "/?" + query.Encode()
}

// signingKey is SigV4's chain of four HMACs: the secret is never used to sign the request
// directly, only to derive a key that is good for one day, one region and one service. The
// service is a parameter rather than the constant above so the chain can be checked against
// AWS's own published example, which signs for a different one.
func signingKey(secret, day, region, service string) []byte {
	key := hmacSHA256([]byte("AWS4"+secret), day)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	return hmacSHA256(key, "aws4_request")
}

func hmacSHA256(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// AWS answers with the token for this profile, or says why it cannot. The context is unused and
// present for the reason every other minting function takes one: nothing here goes over the
// network, and a caller should not have to know which of the two does.
func AWS(_ context.Context, region, host string, port int, user string) (string, error) {
	creds, ok := AWSEnvironment()
	if !ok {
		return "", fmt.Errorf(
			"%w: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, "+
				"or run `aws configure export-credentials --format env`",
			errNoCredentials,
		)
	}

	signFor := AWSRegion(region, host)
	if signFor == "" {
		return "", fmt.Errorf(
			"%w: no AWS region — name one on the connection, or set AWS_REGION",
			errNoCredentials,
		)
	}

	return AWSToken(creds, signFor, host, port, user, time.Now()), nil
}
