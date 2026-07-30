package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func profile() domain.ConnectionProfile {
	return domain.ConnectionProfile{
		Name:     "local",
		Engine:   domain.EnginePostgres,
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Database: "shop",
		TLSMode:  domain.TLSPreferred,
	}
}

func TestBuildDSNTLSModes(t *testing.T) {
	tests := []struct {
		mode    domain.TLSMode
		wantTLS string
	}{
		{domain.TLSDisabled, "sslmode='disable'"},
		{domain.TLSPreferred, "sslmode='prefer'"},
		{domain.TLSRequired, "sslmode='require'"},
		{domain.TLSVerifyCA, "sslmode='verify-ca'"},
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
	dsn, err := BuildDSN(profile(), driver.Credentials{Password: "secret"})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}

	for _, want := range []string{
		"host='127.0.0.1'",
		"port='5432'",
		"user='postgres'",
		"dbname='shop'",
		"connect_timeout='5'",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn = %q, want it to contain %q", dsn, want)
		}
	}
}

func TestBuildDSNRoundTripsSpecialCharactersInPassword(t *testing.T) {
	password := `p@ss word'\=%`

	dsn, err := BuildDSN(profile(), driver.Credentials{Password: password})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}

	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig(%q) error = %v", dsn, err)
	}
	if parsed.Password != password {
		t.Errorf("password round-trip = %q, want %q", parsed.Password, password)
	}
	if parsed.Database != "shop" {
		t.Errorf("database = %q, want shop: quoting must not swallow the fields after it", parsed.Database)
	}
}
