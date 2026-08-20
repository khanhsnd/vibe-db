package sqlserver

import (
	"context"
	"net"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/tlscert"
	"github.com/microsoft/go-mssqldb/msdsn"
)

const dialTimeout = 5 * time.Second

// buildConfig fills the driver's own configuration rather than spelling a connection string for it
// to parse back, which is what the MongoDB adapter does and for the same reason: a `*tls.Config` and
// a dialler have no text form, and half the settings would then be in a string and half beside it.
//
// The encryption modes are the profile's read into TDS's four. `preferred` is `EncryptionOff`, which
// is TDS's own default and is not "off" at all — the login packet is always encrypted and the rest
// of the session is encrypted where the server asks for it, which is exactly what preferred means.
func buildConfig(
	profile domain.ConnectionProfile,
	creds driver.Credentials,
) (msdsn.Config, error) {
	if !profile.TLS.Mode.Valid() {
		return msdsn.Config{}, domain.ValidationError{
			Field:   "tlsMode",
			Message: "is not a known TLS mode",
		}
	}

	config := msdsn.Config{
		Host:        profile.Host,
		Port:        uint64(profile.Port),
		Database:    profile.Database,
		User:        profile.User,
		Password:    creds.Password,
		DialTimeout: dialTimeout,
		// Only TCP. A named pipe and shared memory are local transports this app has no profile
		// for, and leaving them in the list would have a failed dial try two more ways to reach a
		// server that is not there.
		Protocols:  []string{"tcp"},
		Encryption: msdsn.EncryptionOff,
	}

	switch profile.TLS.Mode {
	case domain.TLSDisabled:
		config.Encryption = msdsn.EncryptionDisabled
	case domain.TLSPreferred:
		config.Encryption = msdsn.EncryptionOff
	default:
		config.Encryption = msdsn.EncryptionRequired
	}

	tlsConfig, err := tlscert.Config(profile.TLS, profile.Host)
	if err != nil {
		return msdsn.Config{}, domain.FileError{Err: err}
	}
	if tlsConfig != nil {
		config.TLSConfig = tlsConfig
		// The name the certificate is checked against is already on that configuration — the
		// profile's own where it named one, and the host dialled otherwise. Saying it was provided
		// is what stops the driver overwriting it with the host, which is the difference between a
		// tunnelled connection verifying and not.
		config.HostInCertificateProvided = true
	}

	return config, nil
}

// tunnelDialer is a bastion's dial in the shape the driver takes one. It is a HostDialer as well as
// a Dialer, which is what tells the driver to resolve the server's name **on the far side**: a name
// that only exists inside the network the tunnel reaches would otherwise be looked up here.
type tunnelDialer struct {
	dial driver.Dialer
	host string
}

func (t tunnelDialer) DialContext(
	ctx context.Context,
	network, address string,
) (net.Conn, error) {
	return t.dial(ctx, network, address)
}

func (t tunnelDialer) HostName() string {
	return t.host
}
