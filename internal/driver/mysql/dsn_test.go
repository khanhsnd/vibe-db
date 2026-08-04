package mysql

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func profile() domain.ConnectionProfile {
	return domain.ConnectionProfile{
		Name:   "local",
		Engine: domain.EngineMySQL,
		Host:   "127.0.0.1",
		Port:   3306,
		User:   "root",
		Auth:   domain.AuthPassword,
		TLS:    domain.TLSOptions{Mode: domain.TLSPreferred},
	}
}

// built is the DSN a profile produces once its transport has been registered, which is what
// every test below is really asking about. Nothing is left registered behind it.
func built(t *testing.T, p domain.ConnectionProfile, creds driver.Credentials) (string, error) {
	t.Helper()

	transport, err := dial(p, nil)
	if err != nil {
		return "", err
	}
	t.Cleanup(transport.release)

	return BuildDSN(p, creds, transport)
}

func TestBuildDSNTLSModes(t *testing.T) {
	tests := []struct {
		mode    domain.TLSMode
		wantTLS string
	}{
		{domain.TLSDisabled, "tls=false"},
		{domain.TLSPreferred, "tls=preferred"},
		{domain.TLSRequired, "tls=skip-verify"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			p := profile()
			p.TLS.Mode = tt.mode

			dsn, err := built(t, p, driver.Credentials{Password: "secret"})
			if err != nil {
				t.Fatalf("BuildDSN() error = %v", err)
			}
			if !strings.Contains(dsn, tt.wantTLS) {
				t.Errorf("dsn = %q, want it to contain %q", dsn, tt.wantTLS)
			}
		})
	}
}

// The two verifying modes are a configuration this side builds, so the DSN names a registration
// of its own rather than one of go-sql-driver's built-ins.
func TestBuildDSNRegistersTheVerifyingModes(t *testing.T) {
	for _, mode := range []domain.TLSMode{domain.TLSVerifyCA, domain.TLSVerifyFull} {
		t.Run(string(mode), func(t *testing.T) {
			p := profile()
			p.TLS.Mode = mode

			dsn, err := built(t, p, driver.Credentials{})
			if err != nil {
				t.Fatalf("BuildDSN() error = %v", err)
			}
			if !strings.Contains(dsn, "tls=vibe-") {
				t.Errorf("dsn = %q, want it to name a registered TLS configuration", dsn)
			}
		})
	}
}

// A tunnel is a registered dialler, and the DSN names it where `tcp` would otherwise be.
func TestBuildDSNNamesATunnelsDialler(t *testing.T) {
	transport, err := dial(profile(), func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("not dialled in this test")
	})
	if err != nil {
		t.Fatalf("dial() error = %v", err)
	}
	t.Cleanup(transport.release)

	dsn, err := BuildDSN(profile(), driver.Credentials{}, transport)
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}
	if strings.Contains(dsn, "tcp(") || !strings.Contains(dsn, "vibe-") {
		t.Errorf("dsn = %q, want it to name the registered dialler rather than tcp", dsn)
	}
}

func TestBuildDSNRejectsUnknownTLSMode(t *testing.T) {
	p := profile()
	p.TLS.Mode = "sometimes"

	_, err := built(t, p, driver.Credentials{})

	var validationErr domain.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "tlsMode" {
		t.Fatalf("BuildDSN() error = %v, want ValidationError on tlsMode", err)
	}
}

func TestBuildDSNCarriesConnectionSettings(t *testing.T) {
	p := profile()
	p.Database = "shop"

	dsn, err := built(t, p, driver.Credentials{Password: "p@ss word/?#"})
	if err != nil {
		t.Fatalf("BuildDSN() error = %v", err)
	}

	for _, want := range []string{
		"root:",
		"tcp(127.0.0.1:3306)",
		"/shop?",
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

	if strings.Contains(dsn, "parseTime=true") {
		t.Error("parseTime must stay off; the grid shows and edits MySQL's own text form")
	}
}

func TestBuildDSNRoundTripsSpecialCharactersInPassword(t *testing.T) {
	p := profile()
	password := "p@ss:word/?#[]"

	dsn, err := built(t, p, driver.Credentials{Password: password})
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
