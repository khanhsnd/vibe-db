package sqlkit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// writePair puts a self-signed certificate and its key on disk and answers with both paths. It is
// the only way to test the file-reading half of TLSConfig honestly: a path that does not exist
// proves nothing about a certificate that does.
func writePair(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "vibe-db test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	marshalled, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error = %v", err)
	}

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	write := func(path, blockType string, bytes []byte) {
		encoded := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: bytes})
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
	write(certPath, "CERTIFICATE", der)
	write(keyPath, "EC PRIVATE KEY", marshalled)

	return certPath, keyPath
}

func TestTLSConfigIsNothingForAModeThatDoesNotEncrypt(t *testing.T) {
	for _, mode := range []domain.TLSMode{domain.TLSDisabled, domain.TLSPreferred} {
		config, err := TLSConfig(domain.TLSOptions{Mode: mode}, "db.example.com")
		if err != nil {
			t.Fatalf("TLSConfig(%q) error = %v", mode, err)
		}
		if config != nil {
			t.Errorf("TLSConfig(%q) = %+v, want nil", mode, config)
		}
	}
}

// `required` encrypts and holds the server to nothing, which is what both engines' middle mode
// has always meant; `verify-full` is the only one that checks the name as well as the chain.
func TestTLSConfigVerifiesByMode(t *testing.T) {
	tests := []struct {
		mode        domain.TLSMode
		skipVerify  bool
		checksChain bool
	}{
		{domain.TLSRequired, true, false},
		{domain.TLSVerifyCA, true, true},
		{domain.TLSVerifyFull, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			config, err := TLSConfig(domain.TLSOptions{Mode: tt.mode}, "db.example.com")
			if err != nil {
				t.Fatalf("TLSConfig() error = %v", err)
			}
			if config.InsecureSkipVerify != tt.skipVerify {
				t.Errorf("InsecureSkipVerify = %v, want %v", config.InsecureSkipVerify, tt.skipVerify)
			}
			if (config.VerifyPeerCertificate != nil) != tt.checksChain {
				t.Errorf("a chain check %v, want %v", config.VerifyPeerCertificate != nil, tt.checksChain)
			}
			if config.ServerName != "db.example.com" {
				t.Errorf("ServerName = %q, want the host", config.ServerName)
			}
		})
	}
}

func TestTLSConfigTakesTheServerNameOverride(t *testing.T) {
	config, err := TLSConfig(domain.TLSOptions{
		Mode:       domain.TLSVerifyFull,
		ServerName: "shop.internal",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("TLSConfig() error = %v", err)
	}
	if config.ServerName != "shop.internal" {
		t.Errorf("ServerName = %q, want the override", config.ServerName)
	}
}

func TestTLSConfigReadsTheFiles(t *testing.T) {
	certPath, keyPath := writePair(t)

	config, err := TLSConfig(domain.TLSOptions{
		Mode:       domain.TLSVerifyFull,
		RootCert:   certPath,
		ClientCert: certPath,
		ClientKey:  keyPath,
	}, "db.example.com")
	if err != nil {
		t.Fatalf("TLSConfig() error = %v", err)
	}
	if config.RootCAs == nil {
		t.Error("RootCAs = nil, want the certificate that was named")
	}
	if len(config.Certificates) != 1 {
		t.Errorf("Certificates = %d, want the pair that was named", len(config.Certificates))
	}
}

func TestTLSConfigSaysWhichFileItCouldNotRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.pem")

	if _, err := TLSConfig(domain.TLSOptions{
		Mode:     domain.TLSVerifyFull,
		RootCert: missing,
	}, "db.example.com"); err == nil {
		t.Error("TLSConfig() = nil error, want a failure naming the root certificate")
	}

	// A file that is there and holds no certificate is the other half of the same mistake.
	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := TLSConfig(domain.TLSOptions{
		Mode:     domain.TLSVerifyFull,
		RootCert: empty,
	}, "db.example.com"); err == nil {
		t.Error("TLSConfig() = nil error, want a failure on a file holding no certificate")
	}
}
