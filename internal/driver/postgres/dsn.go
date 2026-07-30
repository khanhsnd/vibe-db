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
	domain.TLSDisabled:  "disable",
	domain.TLSPreferred: "prefer",
	domain.TLSRequired:  "require",
	domain.TLSVerifyCA:  "verify-ca",
}

func BuildDSN(profile domain.ConnectionProfile, creds driver.Credentials) (string, error) {
	sslMode, ok := sslModes[profile.TLSMode]
	if !ok {
		return "", domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	return keywords([][2]string{
		{"host", profile.Host},
		{"port", strconv.Itoa(profile.Port)},
		{"user", profile.User},
		{"password", creds.Password},
		{"dbname", profile.Database},
		{"sslmode", sslMode},
		{"connect_timeout", strconv.Itoa(int(dialTimeout.Seconds()))},
	}), nil
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
