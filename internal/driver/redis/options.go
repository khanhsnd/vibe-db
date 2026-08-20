package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/tlscert"
)

const (
	dialTimeout = 5 * time.Second
	// poolSize is small on purpose. A key-space read is a scan and a pipeline rather than a
	// fan-out, so nothing here asks for more than a couple of connections at once, and a client
	// per database means the count is multiplied by however many the reader opens.
	poolSize = 4
	poolIdle = 5 * time.Minute
)

// clientOptions is this engine's whole connection, spelled as the driver's own options rather than
// as a URL, for the reason the MongoDB adapter spells its own: a `redis://` URL would mean
// percent-encoding a password a reader typed only to parse it straight back.
//
// The database is deliberately *not* set here. Every client this session opens is built from these
// options with one index written into it — see session.client — because `SELECT` is connection state
// and a pool that switched databases underneath a read would answer from whichever one it happened
// to be on.
func clientOptions(
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (*goredis.Options, error) {
	if profile.Auth.Minted() {
		// A cloud identity is minted above this layer as an ordinary password, and no Redis
		// authentication takes one: it would be a login that failed for a reason the reader could
		// not see. Say so instead.
		return nil, domain.Unsupported("cloud identity authentication on Redis")
	}
	if !profile.TLS.Mode.Valid() {
		return nil, domain.ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	config, err := tlscert.Config(profile.TLS, profile.Host)
	if err != nil {
		return nil, err
	}

	return &goredis.Options{
		Addr: profile.Address(),
		// A server with no ACL takes no user name at all, and one with an ACL takes the user beside
		// the password. Both are what the profile says, blank included — see domain.Engine.NeedsUser.
		Username:        profile.User,
		Password:        creds.Password,
		DialTimeout:     dialTimeout,
		PoolSize:        poolSize,
		ConnMaxIdleTime: poolIdle,
		TLSConfig:       config,
		// The tunnel is already the shape this driver wants, which is the whole reason neither
		// adapter has ever heard of SSH.
		Dialer: tunnel,
	}, nil
}
