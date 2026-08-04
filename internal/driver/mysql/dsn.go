package mysql

import (
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const dialTimeout = 5 * time.Second

// BuildDSN spells the connection string. Neither the dialler nor the TLS configuration is in it:
// go-sql-driver takes both by *name*, and `transport` is what registers them and says what those
// names are — see transport.go.
func BuildDSN(
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	t transport,
) (string, error) {
	if !profile.TLS.Mode.Valid() {
		return "", domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	cfg := mysql.NewConfig()
	cfg.Net = t.net
	cfg.Addr = profile.Address()
	cfg.User = profile.User
	cfg.Passwd = creds.Password
	cfg.DBName = profile.Database
	cfg.TLSConfig = t.tls
	cfg.Timeout = dialTimeout
	cfg.ParseTime = false
	cfg.InterpolateParams = true
	cfg.MultiStatements = false

	// An IAM token is a signed request rather than a password, and the server hands it to the
	// AWS authentication plugin, which wants it in the clear over an encrypted connection.
	if profile.Auth == domain.AuthAWSIAM {
		cfg.AllowCleartextPasswords = true
	}

	return cfg.FormatDSN(), nil
}
