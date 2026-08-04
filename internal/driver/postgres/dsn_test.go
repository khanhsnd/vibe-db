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
		Auth:     domain.AuthPassword,
		TLS:      domain.TLSOptions{Mode: domain.TLSPreferred},
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
		{domain.TLSVerifyFull, "sslmode='verify-full'"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			p := profile()
			p.TLS.Mode = tt.mode

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

// libpq takes the certificate files as keywords of its own, so they are in the string rather
// than registered the way MySQL's are — and a file nobody named is left out of it entirely,
// because `sslrootcert=”` is a file called nothing and fails on it.
func TestBuildDSNCarriesCertificateFilesOnlyWhenNamed(t *testing.T) {
	bare, err := BuildDSN(profile(), driver.Credentials{})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}
	for _, absent := range []string{"sslrootcert", "sslcert", "sslkey"} {
		if strings.Contains(bare, absent) {
			t.Errorf("dsn = %q, want no %s when none is named", bare, absent)
		}
	}

	p := profile()
	p.TLS = domain.TLSOptions{
		Mode:       domain.TLSVerifyFull,
		RootCert:   `C:\certs\ca.pem`,
		ClientCert: `C:\certs\client.pem`,
		ClientKey:  `C:\certs\client.key`,
	}

	dsn, err := BuildDSN(p, driver.Credentials{})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}
	// The backslashes of a Windows path have to survive the quoting, or libpq goes looking for
	// a file with half a name.
	for _, want := range []string{
		`sslrootcert='C:\\certs\\ca.pem'`,
		`sslcert='C:\\certs\\client.pem'`,
		`sslkey='C:\\certs\\client.key'`,
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn = %q, want it to contain %q", dsn, want)
		}
	}
}

func TestBuildDSNRejectsUnknownTLSMode(t *testing.T) {
	p := profile()
	p.TLS.Mode = "sometimes"

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
