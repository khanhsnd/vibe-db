package mongo

import (
	"context"
	"net"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	appName = "vibe-db"

	connectTimeout = 5 * time.Second
	// selectTimeout is how long the driver looks for a server to talk to before saying it cannot
	// find one. The driver's own default is thirty seconds, which turns a mistyped host into half a
	// minute of a spinner; five is long enough for a replica set to elect and short enough that a
	// wrong port is a wrong port.
	selectTimeout = 5 * time.Second

	maxPoolSize = 8
	minPoolSize = 1
	poolIdle    = 5 * time.Minute
)

// clientOptions is this engine's whole connection string, spelled as the driver's own options
// rather than as a URI: a URI would mean percent-encoding a password and a host that a reader typed,
// and there is nothing to gain from writing text only to parse it back.
//
// Neither the dialler nor the TLS configuration is named globally the way MySQL's has to be — the
// driver takes both as values here — which is why this adapter has no `transport.go` and nothing to
// release when a session closes.
func clientOptions(
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (*options.ClientOptions, error) {
	if profile.Auth.Minted() {
		// A cloud identity is minted above this layer as an ordinary password, and no MongoDB
		// authentication mechanism takes one: passing it as a password would be a login that failed
		// for a reason the reader could not see. Say so instead.
		return nil, domain.Unsupported("cloud identity authentication on MongoDB")
	}
	if !profile.TLS.Mode.Valid() {
		return nil, domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	opts := options.Client().
		SetAppName(appName).
		SetHosts([]string{profile.Address()}).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(selectTimeout).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetMaxConnIdleTime(poolIdle)

	// A server started without access control takes no credentials at all, which is why a blank
	// user is a connection rather than a validation failure — see domain.ConnectionProfile.Validate.
	if profile.User != "" {
		opts.SetAuth(options.Credential{
			Username:    profile.User,
			Password:    creds.Password,
			PasswordSet: true,
		})
	}

	// A tunnelled connection is held to the one server the profile names. Topology discovery would
	// otherwise hand back the *other* members' own addresses, which are resolvable on the far side
	// of the bastion and nowhere on this machine — a replica set that connected and then could not
	// be read.
	if tunnel != nil {
		opts.SetDialer(dialer(tunnel)).SetDirect(true)
	}

	config, err := sqlkit.TLSConfig(profile.TLS, profile.Host)
	if err != nil {
		return nil, err
	}
	if config != nil {
		opts.SetTLSConfig(config)
	}

	return opts, nil
}

// dialer is the tunnel as this driver takes it. `driver.Dialer` is already the right shape; only the
// name of the method differs, which is the whole of what this exists for.
type dialer func(ctx context.Context, network, address string) (net.Conn, error)

func (d dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d(ctx, network, address)
}
