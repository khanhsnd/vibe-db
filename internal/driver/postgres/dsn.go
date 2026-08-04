package postgres

import (
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const dialTimeout = 5 * time.Second

// sslModes maps the TLS modes the UI offers onto libpq's own names, which pgx parses.
// `verify-ca` keeps libpq's meaning — the chain is checked but the host name is not — so
// the label in the dialog says what the server is actually held to.
var sslModes = map[domain.TLSMode]string{
	domain.TLSDisabled:   "disable",
	domain.TLSPreferred:  "prefer",
	domain.TLSRequired:   "require",
	domain.TLSVerifyCA:   "verify-ca",
	domain.TLSVerifyFull: "verify-full",
}

// BuildDSN renders the connection string. Unlike MySQL, the certificate files go *in* it:
// `sslrootcert`, `sslcert` and `sslkey` are libpq's own keywords and pgx reads them, so there is
// nothing for this side to build or register — see mysql/transport.go for the other half of that
// difference.
func BuildDSN(profile domain.ConnectionProfile, creds driver.Credentials) (string, error) {
	sslMode, ok := sslModes[profile.TLS.Mode]
	if !ok {
		return "", domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	fields := [][2]string{
		{"host", profile.Host},
		{"port", strconv.Itoa(profile.Port)},
		{"user", profile.User},
		{"password", creds.Password},
		{"dbname", profile.Database},
		{"sslmode", sslMode},
		{"connect_timeout", strconv.Itoa(int(dialTimeout.Seconds()))},
	}

	// A file left blank is left out rather than sent empty: libpq reads `sslrootcert=''` as a
	// file called nothing and fails on it, where an absent keyword falls back to its own default.
	// The reader's *ServerName* is not here at all — it is applied to the parsed configuration in
	// driver.go, because libpq has no keyword for "check the certificate against another name".
	for _, field := range [][2]string{
		{"sslrootcert", profile.TLS.RootCert},
		{"sslcert", profile.TLS.ClientCert},
		{"sslkey", profile.TLS.ClientKey},
	} {
		if field[1] != "" {
			fields = append(fields, field)
		}
	}

	return keywords(fields), nil
}

// keywords renders a libpq keyword/value connection string. Every value is quoted, because
// a password is arbitrary text and an unquoted space in one would end the field early.
func keywords(fields [][2]string) string {
	rendered := make([]string, len(fields))
	for i, field := range fields {
		rendered[i] = field[0] + "=" + quoteValue(field[1])
	}
	return strings.Join(rendered, " ")
}

func quoteValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value)
	return "'" + escaped + "'"
}
