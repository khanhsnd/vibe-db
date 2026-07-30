package mysql

import (
	"errors"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func profile() domain.ConnectionProfile {
	return domain.ConnectionProfile{
		Name:    "local",
		Engine:  domain.EngineMySQL,
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		TLSMode: domain.TLSPreferred,
	}
}

func TestBuildDSNTLSModes(t *testing.T) {
	tests := []struct {
		mode    domain.TLSMode
		wantTLS string
	}{
		{domain.TLSDisabled, "tls=false"},
		{domain.TLSPreferred, "tls=preferred"},
		{domain.TLSRequired, "tls=skip-verify"},
		{domain.TLSVerifyCA, "tls=true"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			p := profile()
			p.TLSMode = tt.mode

			dsn, err := BuildDSN(p, driver.Credentials{Password: "secret"})
			if err != nil {
				t.Fatalf("BuildDSN() error = %v", err)
			}
			if !strings.Contains(dsn, tt.wantTLS) {
				t.Errorf("dsn = %q, want it to contain %q", dsn, tt.wantTLS)
			}
		})
	}
}

func TestBuildDSNRejectsUnknownTLSMode(t *testing.T) {
	p := profile()
	p.TLSMode = "sometimes"

	_, err := BuildDSN(p, driver.Credentials{})

	var validationErr domain.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "tlsMode" {
		t.Fatalf("BuildDSN() error = %v, want ValidationError on tlsMode", err)
	}
}

func TestBuildDSNCarriesConnectionSettings(t *testing.T) {
	p := profile()
	p.Database = "shop"

	dsn, err := BuildDSN(p, driver.Credentials{Password: "p@ss word/?#"})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}

	for _, want := range []string{
		"root:",
		"tcp(127.0.0.1:3306)",
		"/shop?",
		"parseTime=true",
		"interpolateParams=true",
		"timeout=5s",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn = %q, want it to contain %q", dsn, want)
		}
	}

	if strings.Contains(dsn, "multiStatements=true") {
		t.Error("multiStatements must stay off; statements are split client-side")
	}
}

func TestBuildDSNRoundTripsSpecialCharactersInPassword(t *testing.T) {
	p := profile()
	password := "p@ss:word/?#[]"

	dsn, err := BuildDSN(p, driver.Credentials{Password: password})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}

	parsed, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q) error = %v", dsn, err)
	}
	if parsed.Passwd != password {
		t.Errorf("password round-trip = %q, want %q", parsed.Passwd, password)
	}
}
