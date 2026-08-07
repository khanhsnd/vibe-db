package oracle

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	go_ora "github.com/sijms/go-ora/v2"
)

const dialTimeout = 5 * time.Second

// buildURL spells the connection the driver's own way. Oracle's is a URL rather than a keyword
// list, and the profile's Database is its **service name** — what the listener is asked for, and
// the one thing on an Oracle profile that is not what the same box means on any other engine.
//
// The certificate files are not in it. Those become a `*tls.Config` handed to the connector, so
// what is here is only the two things Oracle needs told in words: that the connection is to be
// encrypted at all, and whether the driver should check the certificate itself — which it must not,
// the app's own configuration having already decided how far to check.
func buildURL(
	profile domain.ConnectionProfile,
	creds driver.Credentials,
) (string, error) {
	if !profile.TLS.Mode.Valid() {
		return "", domain.ValidationError{
			Field:   "tlsMode",
			Message: "is not a known TLS mode",
		}
	}

	options := map[string]string{
		"CONNECT TIMEOUT": strconv.Itoa(int(dialTimeout.Seconds())),
	}

	// Oracle has no `preferred`: a listener endpoint is either TCP or TCPS and the client has to
	// know which before it dials. So the middle mode is read as plain text, which is what a
	// listener's default endpoint is, and a reader wanting TLS asks for it.
	if profile.TLS.Mode.Encrypts() {
		options["SSL"] = "TRUE"
		options["SSL VERIFY"] = "FALSE"
	}

	return go_ora.BuildUrl(
		profile.Host,
		profile.Port,
		profile.Database,
		profile.User,
		creds.Password,
		options,
	), nil
}

// tunnelDialer is a bastion's dial in the shape the driver takes one.
type tunnelDialer struct {
	dial driver.Dialer
}

func (t tunnelDialer) DialContext(
	ctx context.Context,
	network, address string,
) (net.Conn, error) {
	return t.dial(ctx, network, address)
}
