package netproxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// fakeProxy is one HTTP proxy, answering every CONNECT with `status` and then — on a 200 — echoing
// whatever is written to it, which is how a test can tell the socket really is the far side's.
//
// It answers `asked` with the address of the last CONNECT it was given, since carrying the address
// **to the far side** rather than resolving it here is the whole reason a proxy is worth having.
type fakeProxy struct {
	listener net.Listener
	status   string
	asked    chan *http.Request
}

func startFakeProxy(t *testing.T, status string) *fakeProxy {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	proxy := &fakeProxy{listener: listener, status: status, asked: make(chan *http.Request, 1)}
	go proxy.serve()

	t.Cleanup(func() { listener.Close() })
	return proxy
}

func (p *fakeProxy) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.answer(conn)
	}
}

func (p *fakeProxy) answer(conn net.Conn) {
	reader := bufio.NewReader(conn)

	request, err := http.ReadRequest(reader)
	if err != nil {
		conn.Close()
		return
	}
	p.asked <- request

	if _, err := io.WriteString(conn, "HTTP/1.1 "+p.status+"\r\n\r\n"); err != nil {
		conn.Close()
		return
	}
	if !strings.HasPrefix(p.status, "200") {
		conn.Close()
		return
	}

	// The far side, standing in for a database: whatever it is sent, it says back.
	io.Copy(conn, reader)
	conn.Close()
}

func (p *fakeProxy) options(user string) domain.ProxyOptions {
	host, port, _ := net.SplitHostPort(p.listener.Addr().String())
	number, _ := strconv.Atoi(port)

	return domain.ProxyOptions{
		Enabled: true,
		Kind:    domain.ProxyHTTP,
		Host:    host,
		Port:    number,
		User:    user,
	}
}

// TestHTTPProxyCarriesTheConnection is the whole of what an HTTP proxy is for: the address is
// resolved **on the far side**, and what comes back is the socket to it rather than a response.
func TestHTTPProxyCarriesTheConnection(t *testing.T) {
	fake := startFakeProxy(t, "200 Connection Established")

	proxy, err := Open(fake.options(""), Secrets{})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}

	conn, err := proxy.Dial(context.Background(), "tcp", "db.internal:5432")
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	if asked := <-fake.asked; asked.Host != "db.internal:5432" {
		t.Errorf("proxy was asked for %q, want db.internal:5432", asked.Host)
	}

	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatalf("write: %v", err)
	}

	echoed := make([]byte, 4)
	if _, err := io.ReadFull(conn, echoed); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(echoed) != "ping" {
		t.Errorf("read %q back, want ping — the socket is not the far side's", echoed)
	}
}

// TestHTTPProxyCarriesTheCredentials checks the one header that matters, and that a proxy asking for
// nobody is sent nobody: an empty user would otherwise be an authentication nobody wanted.
func TestHTTPProxyCarriesTheCredentials(t *testing.T) {
	fake := startFakeProxy(t, "200 Connection Established")

	proxy, err := Open(fake.options("reader"), Secrets{Secret: "opensesame"})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}

	conn, err := proxy.Dial(context.Background(), "tcp", "db.internal:5432")
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	// `reader:opensesame`, base64.
	const want = "Basic cmVhZGVyOm9wZW5zZXNhbWU="
	if got := (<-fake.asked).Header.Get("Proxy-Authorization"); got != want {
		t.Errorf("Proxy-Authorization = %q, want %q", got, want)
	}
}

func TestHTTPProxyWithNoUserSendsNoHeader(t *testing.T) {
	fake := startFakeProxy(t, "200 Connection Established")

	proxy, _ := Open(fake.options(""), Secrets{})
	conn, err := proxy.Dial(context.Background(), "tcp", "db.internal:5432")
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	if got := (<-fake.asked).Header.Get("Proxy-Authorization"); got != "" {
		t.Errorf("Proxy-Authorization = %q, want it absent", got)
	}
}

// TestHTTPProxyTellsTheTwoFailuresApart is why there are two sentinels: the reader fixes a rejected
// credential in one box and an unreachable far side in another.
func TestHTTPProxyTellsTheTwoFailuresApart(t *testing.T) {
	cases := []struct {
		status string
		want   error
	}{
		{"407 Proxy Authentication Required", domain.ErrProxyCredentials},
		{"502 Bad Gateway", domain.ErrProxyFailed},
		{"403 Forbidden", domain.ErrProxyFailed},
	}

	for _, one := range cases {
		fake := startFakeProxy(t, one.status)

		proxy, _ := Open(fake.options("reader"), Secrets{Secret: "wrong"})
		_, err := proxy.Dial(context.Background(), "tcp", "db.internal:5432")

		if !errors.Is(err, one.want) {
			t.Errorf("%s: Dial() = %v, want %v", one.status, err, one.want)
		}
	}
}

// TestProxyRefusesWhatItCannotCarry is the honest answer to a driver asking for something neither
// protocol carries: saying so beats handing back a connection that is not the one it asked for.
func TestProxyRefusesWhatItCannotCarry(t *testing.T) {
	fake := startFakeProxy(t, "200 Connection Established")

	proxy, _ := Open(fake.options(""), Secrets{})
	if _, err := proxy.Dial(context.Background(), "udp", "db.internal:5432"); !errors.Is(err, domain.ErrUnsupported) {
		t.Errorf("Dial(udp) = %v, want it unsupported", err)
	}
}

// TestOpenChecksTheProfile is the guard in front of every dial: a proxy nobody filled in is refused
// where it is configured rather than where it is used.
func TestOpenChecksTheProfile(t *testing.T) {
	_, err := Open(domain.ProxyOptions{Enabled: true, Kind: "socks4", Host: "10.0.0.9", Port: 1080}, Secrets{})

	var invalid domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Field != "proxyKind" {
		t.Errorf("Open() = %v, want it refused on proxyKind", err)
	}
}

// TestOpenReadsASOCKSAddress checks the one thing building a SOCKS dialler can fail on. Nothing is
// reached: a proxy is spoken to per connection, so a host that is not there fails on the first dial.
func TestOpenReadsASOCKSAddress(t *testing.T) {
	proxy, err := Open(domain.ProxyOptions{
		Enabled: true,
		Kind:    domain.ProxySOCKS5,
		Host:    "10.0.0.9",
		Port:    domain.DefaultSOCKSPort,
	}, Secrets{})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if proxy.socks == nil {
		t.Error("a SOCKS5 proxy was opened with no dialler")
	}
}
