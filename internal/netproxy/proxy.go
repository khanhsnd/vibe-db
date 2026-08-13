// Package netproxy carries a database connection through a SOCKS5 or an HTTP proxy. It knows
// nothing about databases: it hands back a function that opens connections *through* the proxy,
// which is exactly the shape `driver.Dialer` has — the arrangement `sshtunnel` has, and for the same
// reason. That is why no engine adapter has ever heard of a proxy either.
//
// The address it is asked for is resolved on the far side, so a host that only exists inside the
// network the proxy sits on is named in the profile as that network knows it. Nothing is listened on
// locally: there is no forwarded port to collide with anything.
//
// **A proxy is the outer hop.** It is how this machine reaches the network at all, so a profile that
// has both dials the *bastion* through the proxy and then the database through the bastion — see
// `session.Manager.Open`, which is where the two are put in that order.
package netproxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/net/proxy"
)

// dialTimeout bounds reaching the proxy itself. What happens *after* it — the far side answering —
// is the caller's deadline, which arrives on the context.
const dialTimeout = 10 * time.Second

// Secrets is what the credential store holds for a proxy: the password of the user it authenticates
// as. One field, because a proxy takes at most one — the arrangement `sshtunnel.Secrets` has.
type Secrets struct {
	Secret string
}

// Proxy is one configured proxy. It holds no connection of its own: SOCKS5 and HTTP CONNECT are both
// negotiated per connection, so there is nothing to keep alive and nothing to close — which is the
// one way this differs from a tunnel.
type Proxy struct {
	config domain.ProxyOptions
	secret string
	// socks is the dialler for a SOCKS5 proxy, built once because building it parses the address.
	// It is nil for an HTTP proxy, whose whole handshake is connectHTTP below.
	socks proxy.ContextDialer
}

// Open reads the profile's proxy and answers with something that can dial through it. It reaches
// nothing: a proxy is spoken to per connection, so the first time this one is contacted is the first
// Dial — which is when the reader finds out it is not there.
func Open(config domain.ProxyOptions, secrets Secrets) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	p := &Proxy{config: config, secret: secrets.Secret}
	if config.Kind != domain.ProxySOCKS5 {
		return p, nil
	}

	forward := &net.Dialer{Timeout: dialTimeout}
	dialer, err := proxy.SOCKS5("tcp", config.Address(), p.socksAuth(), forward)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrProxyFailed, config.Address(), err)
	}

	contextual, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("%w: %s: no context-aware dialler", domain.ErrProxyFailed, config.Address())
	}

	p.socks = contextual
	return p, nil
}

// socksAuth is nil for a proxy that asks for nobody, which is most of them on a private network:
// offering an empty user name instead would make the proxy negotiate an authentication it never
// wanted.
func (p *Proxy) socksAuth() *proxy.Auth {
	if p.config.User == "" {
		return nil
	}
	return &proxy.Auth{User: p.config.User, Password: p.secret}
}

// Dial opens a connection to `address` **through the proxy**. It is `driver.Dialer` in all but name,
// and is what the engine adapter — or the tunnel in front of it — is handed.
func (p *Proxy) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	// Both protocols carry a stream and nothing else. A driver asking for anything but TCP is asking
	// for something no proxy of either kind can carry, and saying so is better than handing back a
	// connection that is not the one it asked for.
	if !strings.HasPrefix(network, "tcp") {
		return nil, domain.Unsupported("carrying " + network + " through a proxy")
	}

	if p.socks != nil {
		conn, err := p.socks.DialContext(ctx, network, address)
		if err != nil {
			return nil, socksError(p.config, address, err)
		}
		return conn, nil
	}
	return p.connectHTTP(ctx, address)
}

// connectHTTP is the whole of an HTTP proxy: `CONNECT host:port`, and on a 200 the same socket is
// the connection to the far side. It is written out here rather than taken from `net/http`, which
// only ever does this on the way to a request of its own — what is wanted is the socket.
func (p *Proxy) connectHTTP(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.config.Address())
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrProxyFailed, p.config.Address(), err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	request := &http.Request{
		Method: http.MethodConnect,
		// A CONNECT request line carries an authority and not a path, which is what an opaque URL
		// with a Host beside it writes.
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	if p.config.User != "" {
		request.Header.Set("Proxy-Authorization", basicAuth(p.config.User, p.secret))
	}

	if err := request.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrProxyFailed, p.config.Address(), err)
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrProxyFailed, p.config.Address(), err)
	}
	response.Body.Close()

	if err := connectStatus(p.config, response); err != nil {
		conn.Close()
		return nil, err
	}

	// Anything the reader buffered past the headers would be bytes of the *far side's* first
	// answer, and they are inside a buffer the caller is never given: the socket goes back naked.
	// No proxy sends any, because nothing on the far side speaks first until this side has — so
	// this is refused rather than papered over with a wrapper that could hide a proxy doing
	// something else entirely.
	if reader.Buffered() > 0 {
		conn.Close()
		return nil, fmt.Errorf("%w: %s: answered before the connection was made",
			domain.ErrProxyFailed, p.config.Address())
	}

	// The handshake's deadline must not outlive the handshake: every read the session makes
	// afterwards goes over this same connection, and a deadline left on it would kill the first
	// query that took longer than the dial was allowed. It is the same rule the tunnel follows.
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// connectStatus tells the two failures apart that a reader answers differently: a proxy that wants
// somebody it has not been given, and a proxy that would not open this connection at all.
func connectStatus(config domain.ProxyOptions, response *http.Response) error {
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusProxyAuthRequired:
		return fmt.Errorf("%w: %s: %s", domain.ErrProxyCredentials, config.Address(), response.Status)
	default:
		return fmt.Errorf("%w: %s: %s", domain.ErrProxyFailed, config.Address(), response.Status)
	}
}

func basicAuth(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

// socksError tells a rejected credential from a proxy that never answered. `x/net/proxy` gives no
// code to switch on, only its own wording, so this reads a message — for the reason
// `sshtunnel.isAuthFailure` does, and with the same bargain: it is the library's own text and not
// anything a reader typed, and what the UI switches on is still the code this produces.
func socksError(config domain.ProxyOptions, address string, err error) error {
	message := err.Error()
	if strings.Contains(message, "authentication failed") ||
		strings.Contains(message, "no acceptable authentication") {
		return fmt.Errorf("%w: %s: %w", domain.ErrProxyCredentials, config.Address(), err)
	}
	return fmt.Errorf("%w: %s via %s: %w", domain.ErrProxyFailed, address, config.Address(), err)
}
