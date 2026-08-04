package cloudauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// errNoCredentials is what both halves of this package answer with when the machine holds no
// identity to sign in as. It wraps the domain's own sentinel so the app layer maps it without
// knowing which cloud was being reached.
var errNoCredentials = domain.ErrTokenUnavailable

const (
	googleTokenURL = "https://oauth2.googleapis.com/token"
	// googleScope is what a Cloud SQL IAM login is authorised by. `sqlservice.login` is the
	// narrow one — it grants signing in to a database and nothing else about the project — and is
	// what a service account needs on top of the Cloud SQL Instance User role.
	googleScope = "https://www.googleapis.com/auth/sqlservice.login"
	// googleAssertionLife is how long the JWT a service account swaps for a token is good for.
	// It is the assertion's life and not the token's — the token's is Google's to decide.
	googleAssertionLife = 10 * time.Minute
	googleTimeout       = 15 * time.Second
)

// googleCredentials is the shape of an application-default-credentials file. Google writes two
// kinds under one name and the `type` field is what tells them apart: `authorized_user` is the
// reader's own login from `gcloud auth application-default login`, and `service_account` is a key
// file. Both end at the same token endpoint by different routes.
type googleCredentials struct {
	Type string `json:"type"`

	// authorized_user
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`

	// service_account
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// Google answers with an OAuth 2.0 access token, which is what Cloud SQL IAM database
// authentication takes as a password. Nothing about the profile decides it: the identity is the
// machine's, exactly as `gcloud` and every Google client library read it.
func Google(ctx context.Context) (string, error) {
	creds, err := googleADC()
	if err != nil {
		return "", err
	}

	switch creds.Type {
	case "service_account":
		return googleServiceAccountToken(ctx, creds)
	case "authorized_user":
		return googleRefreshedToken(ctx, creds)
	default:
		return "", fmt.Errorf("%w: unsupported credential type %q", errNoCredentials, creds.Type)
	}
}

// googleADC finds the application default credentials the way every Google tool does:
// GOOGLE_APPLICATION_CREDENTIALS if it names a file, and otherwise the well-known path
// `gcloud auth application-default login` writes to.
func googleADC() (googleCredentials, error) {
	path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if path == "" {
		found, err := googleWellKnownPath()
		if err != nil {
			return googleCredentials{}, err
		}
		path = found
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return googleCredentials{}, fmt.Errorf(
			"%w: run `gcloud auth application-default login`, "+
				"or point GOOGLE_APPLICATION_CREDENTIALS at a key file: %w",
			errNoCredentials, err,
		)
	}

	var creds googleCredentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return googleCredentials{}, fmt.Errorf("%w: %s is not a credentials file", errNoCredentials, path)
	}
	return creds, nil
}

func googleWellKnownPath() (string, error) {
	const file = "application_default_credentials.json"

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "gcloud", file), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", errNoCredentials, err)
	}
	return filepath.Join(home, ".config", "gcloud", file), nil
}

// googleRefreshedToken swaps the reader's own refresh token for an access token, which is the
// ordinary OAuth 2.0 refresh exchange and needs no signing.
func googleRefreshedToken(ctx context.Context, creds googleCredentials) (string, error) {
	return googlePost(ctx, googleTokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
		"refresh_token": {creds.RefreshToken},
		// A refresh does not take a scope of its own: the token inherits what the login granted,
		// which for application default credentials already includes signing in to Cloud SQL.
	})
}

// googleServiceAccountToken signs a JWT with the key from the file and swaps it for an access
// token. That exchange is the whole of OAuth 2.0's JWT bearer grant, and it is short enough to
// write out that pulling in a library to do it would be the larger cost.
func googleServiceAccountToken(ctx context.Context, creds googleCredentials) (string, error) {
	endpoint := creds.TokenURI
	if endpoint == "" {
		endpoint = googleTokenURL
	}

	assertion, err := googleAssertion(creds, endpoint)
	if err != nil {
		return "", err
	}

	return googlePost(ctx, endpoint, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	})
}

func googleAssertion(creds googleCredentials, endpoint string) (string, error) {
	key, err := googlePrivateKey(creds.PrivateKey)
	if err != nil {
		return "", err
	}

	now := time.Now()
	header := base64URL(`{"alg":"RS256","typ":"JWT"}`)

	claims, err := json.Marshal(map[string]any{
		"iss":   creds.ClientEmail,
		"scope": googleScope,
		"aud":   endpoint,
		"iat":   now.Unix(),
		"exp":   now.Add(googleAssertionLife).Unix(),
	})
	if err != nil {
		return "", err
	}

	signed := header + "." + base64URL(string(claims))
	digest := sha256.Sum256([]byte(signed))

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("%w: sign the assertion: %w", errNoCredentials, err)
	}

	return signed + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func googlePrivateKey(encoded string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(encoded))
	if block == nil {
		return nil, fmt.Errorf("%w: the key file holds no private key", errNoCredentials)
	}

	// Google writes PKCS#8; the older PKCS#1 form is read too, because a key file is a file and
	// a reader may well have converted one.
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%w: the private key is not RSA", errNoCredentials)
		}
		return key, nil
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read the private key: %w", errNoCredentials, err)
	}
	return key, nil
}

// googlePost makes the token request and reads the access token out of it. A failure carries
// Google's own wording, which names the misconfiguration far better than anything this side
// could guess at — `invalid_grant` for an expired login, `invalid_scope` for a missing role.
func googlePost(ctx context.Context, endpoint string, form url.Values) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, googleTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: cannot reach Google: %w", errNoCredentials, err)
	}
	defer response.Body.Close()

	var body struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("%w: Google answered with something unreadable", errNoCredentials)
	}

	if body.AccessToken == "" {
		return "", fmt.Errorf("%w: %s", errNoCredentials, googleFailure(body.Error, body.ErrorDescription))
	}
	return body.AccessToken, nil
}

// base64URL is JWT's own encoding: URL-safe and unpadded, the two ways it differs from ordinary
// base64 and the two that a token is rejected for getting wrong.
func base64URL(text string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(text))
}

func googleFailure(code, description string) string {
	switch {
	case description != "":
		return description
	case code != "":
		return code
	default:
		return "Google returned no access token"
	}
}

// Unavailable reports whether a failure is this package saying the machine holds no usable
// identity, which is what the app layer maps onto its own code.
func Unavailable(err error) bool {
	return errors.Is(err, domain.ErrTokenUnavailable)
}
