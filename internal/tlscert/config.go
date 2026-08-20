// Package tlscert builds a `*tls.Config` from the certificate files a profile names.
//
// It sits beside `internal/sshtunnel`, `internal/netproxy` and `internal/neterr`, and is there for
// the same reason those are: **it is not about a database.** A root certificate, a client
// certificate and a server name mean exactly the same thing to an object store, so
// `internal/driver` does not own them and `internal/objectstore` does not copy them — both are
// handed the answer.
//
// It lived in `sqlkit` until an object store needed it, which is the moment "what does not vary by
// engine" turned out to have a wider answer than "what does not vary by SQL engine".
package tlscert

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Config builds the configuration a profile's TLS options describe, or nil where the mode asks
// for none. It is here rather than in an adapter because a root certificate and a client
// certificate are files on this machine and mean the same thing to every engine; what differs is
// only how an engine is *told* about them, which is each adapter's own business — PostgreSQL takes
// the paths in its connection string and MySQL takes a configuration registered under a name.
//
// `host` is the server the certificate is checked against, and the profile's own ServerName
// overrides it. That override is what makes a tunnel workable: the connection goes to a local
// port and the certificate is still the database server's.
func Config(options domain.TLSOptions, host string) (*tls.Config, error) {
	if !options.Mode.Encrypts() {
		return nil, nil
	}

	name := options.ServerName
	if name == "" {
		name = host
	}

	config := &tls.Config{ServerName: name, MinVersion: tls.VersionTLS12}

	if options.ClientCert != "" {
		pair, err := tls.LoadX509KeyPair(options.ClientCert, options.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("read client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{pair}
	}

	roots, err := rootPool(options.RootCert)
	if err != nil {
		return nil, err
	}
	config.RootCAs = roots

	// Go verifies the chain *and* the host name together or not at all, so the two modes that
	// stop short of both have to do their own checking. `required` checks nothing — it is there
	// to encrypt — and `verify-ca` checks the chain and not the name, which is libpq's own
	// meaning and the only one that works against a server addressed by an address.
	switch options.Mode {
	case domain.TLSRequired:
		config.InsecureSkipVerify = true
	case domain.TLSVerifyCA:
		config.InsecureSkipVerify = true
		config.VerifyPeerCertificate = verifyChain(roots)
	}

	return config, nil
}

// rootPool is the authority a chain is checked against: the file the profile names, or nil for
// the machine's own store, which is what `crypto/tls` reads a nil RootCAs as.
func rootPool(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}

	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read root certificate: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("root certificate file holds no certificate")
	}
	return pool, nil
}

// verifyChain checks the certificate the server presented against the roots and nothing else. It
// is what `verify-ca` means: the server proved it was issued by an authority this machine trusts,
// and did not prove it is the host that was dialled.
func verifyChain(roots *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("server presented no certificate")
		}

		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, raw := range rawCerts {
			parsed, err := x509.ParseCertificate(raw)
			if err != nil {
				return err
			}
			certs = append(certs, parsed)
		}

		intermediates := x509.NewCertPool()
		for _, cert := range certs[1:] {
			intermediates.AddCert(cert)
		}

		_, err := certs[0].Verify(x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
		})
		return err
	}
}
