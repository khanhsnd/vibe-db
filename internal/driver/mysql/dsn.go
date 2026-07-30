package mysql

import (
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const dialTimeout = 5 * time.Second

var tlsConfigNames = map[domain.TLSMode]string{
	domain.TLSDisabled:  "false",
	domain.TLSPreferred: "preferred",
	domain.TLSRequired:  "skip-verify",
	domain.TLSVerifyCA:  "true",
}

func BuildDSN(profile domain.ConnectionProfile, creds driver.Credentials) (string, error) {
	tlsConfig, ok := tlsConfigNames[profile.TLSMode]
	if !ok {
		return "", domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = profile.Address()
	cfg.User = profile.User
	cfg.Passwd = creds.Password
	cfg.DBName = profile.Database
	cfg.TLSConfig = tlsConfig
	cfg.Timeout = dialTimeout
	cfg.ParseTime = false
	cfg.InterpolateParams = true
	cfg.MultiStatements = false

	return cfg.FormatDSN(), nil
}
