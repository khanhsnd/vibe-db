// Package sshtunnel carries a database connection over SSH. It knows nothing about databases: it
// dials a bastion, and hands back a function that opens connections *from* that bastion, which is
// exactly the shape `driver.Dialer` has. That is why neither engine adapter has ever heard of SSH.
//
// The address it is asked for is resolved on the far side, so a server bound to loopback on the
// bastion is reached by naming `127.0.0.1` in the profile — the loopback that matters is the
// bastion's, not this machine's. Nothing is listened on locally: there is no forwarded port to
// collide with anything, and nothing else on this machine can reach the tunnel.
package sshtunnel

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	dialTimeout  = 10 * time.Second
	keepAlive    = 30 * time.Second
	clientBanner = "SSH-2.0-vibe-db"
)

// Secrets is what the credential store holds for a bastion: the password for password
// authentication, or the pass phrase of the private key for key authentication. One field,
// because a tunnel only ever needs one of the two and which one is the profile's to say.
type Secrets struct {
	Secret string
}

// Dial is the hop that carries this tunnel's own connection to the bastion. It is nil for a bastion
// this machine reaches directly, and a proxy's dial for one it does not — which is what makes a
// proxy the *outer* hop, and is the same shape this package hands out at the other end.
type Dial func(ctx context.Context, network, address string) (net.Conn, error)

// Tunnel is one live SSH connection. Its Dial is handed to a driver and used for the whole life
// of the session, so the tunnel is closed when the session is and not before.
type Tunnel struct {
	client *ssh.Client
	// done stops the keep-alive. A tunnel outlives every request made over it, so the goroutine
	// watching it has to be told when there is nothing left to watch.
	done chan struct{}
}

// Open dials the bastion and authenticates. It does *not* reach the database — that happens on
// the first Dial, which is when the engine asks for it.
//
// `under` is what carries the connection to the bastion, and is nil for a bastion this machine
// reaches directly.
func Open(ctx context.Context, config domain.SSHTunnel, secrets Secrets, under Dial) (*Tunnel, error) {
	// An agent signs during the handshake, so its socket outlives this call by exactly the
	// handshake and not a moment longer.
	auth, held, err := authMethod(config, secrets)
	if err != nil {
		return nil, err
	}
	if held != nil {
		defer held.Close()
	}

	hostKey, err := hostKeyCallback(config)
	if err != nil {
		return nil, err
	}

	conn, err := dialBastion(ctx, config, under)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, chans, reqs, err := ssh.NewClientConn(conn, config.Address(), &ssh.ClientConfig{
		User:            config.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKey,
		// Which host key to ask the server for is decided by what is already in known_hosts —
		// see hostKeyAlgorithms, and read it before removing this line.
		HostKeyAlgorithms: hostKeyAlgorithms(hostKey, config.Address()),
		ClientVersion:     clientBanner,
		Timeout:           dialTimeout,
	})
	if err != nil {
		conn.Close()
		return nil, handshakeError(config, err)
	}
	// The handshake's deadline must not outlive the handshake: every read the session makes
	// afterwards goes over this same connection, and a deadline left on it would kill the first
	// query that took longer than the dial was allowed.
	_ = conn.SetDeadline(time.Time{})

	tunnel := &Tunnel{client: ssh.NewClient(client, chans, reqs), done: make(chan struct{})}
	go tunnel.keepAlive()
	return tunnel, nil
}

// dialBastion opens the connection the handshake then runs over. x/crypto/ssh has no context-aware
// dial of its own, so the deadline is applied to that connection instead.
//
// A failure on the outer hop is handed back **as it is**: it is already a proxy's own failure, and
// re-wrapping it as a tunnel's would send the reader to the wrong box in the dialog.
func dialBastion(ctx context.Context, config domain.SSHTunnel, under Dial) (net.Conn, error) {
	if under != nil {
		return under(ctx, "tcp", config.Address())
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", config.Address())
	if err != nil {
		return nil, fmt.Errorf("%w: ssh %s: %w", domain.ErrTunnelFailed, config.Address(), err)
	}
	return conn, nil
}

// keepAlive says the tunnel is still wanted. An idle SSH connection is dropped by plenty of
// bastions and by nearly every NAT in between, and the session carried over it then fails on the
// *next query* rather than on the idleness that actually caused it — which reads to the person at
// the keyboard as the database having gone away. A request nobody answers is not an error here:
// finding out is the connection's own business, on its next read.
func (t *Tunnel) keepAlive() {
	ticker := time.NewTicker(keepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			_, _, _ = t.client.SendRequest("keepalive@openssh.com", true, nil)
		}
	}
}

// Dial opens a connection to `address` **from the bastion**. It is `driver.Dialer` in all but
// name, and is what the engine adapter is handed.
func (t *Tunnel) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := t.client.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrTunnelFailed, address, err)
	}
	return conn, nil
}

func (t *Tunnel) Close() error {
	close(t.done)
	return t.client.Close()
}

// authMethod answers with how to authenticate, and with whatever has to stay open while it is
// used — which is only ever the agent's socket, and is nil for the other two.
func authMethod(config domain.SSHTunnel, secrets Secrets) (ssh.AuthMethod, io.Closer, error) {
	switch config.Auth {
	case domain.SSHPassword:
		return ssh.Password(secrets.Secret), nil, nil

	case domain.SSHAgent:
		return agentAuth()

	default:
		pem, err := os.ReadFile(config.KeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: read private key: %w", domain.ErrTunnelFailed, err)
		}

		signer, err := parseKey(pem, secrets.Secret)
		if err != nil {
			return nil, nil, err
		}
		return ssh.PublicKeys(signer), nil, nil
	}
}

// parseKey reads the private key, with its pass phrase where it has one. A key that needs a pass
// phrase and was given none is reported as a *credentials* failure rather than a file failure,
// because that is what the reader has to do something about.
func parseKey(pem []byte, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(pem, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", domain.ErrTunnelCredentials, err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		var needsPassphrase *ssh.PassphraseMissingError
		if errors.As(err, &needsPassphrase) {
			return nil, fmt.Errorf("%w: the private key needs a pass phrase", domain.ErrTunnelCredentials)
		}
		return nil, fmt.Errorf("%w: read private key: %w", domain.ErrTunnelFailed, err)
	}
	return signer, nil
}

// hostKeyCallback is how the bastion proves it is the bastion. Strict is the default and refuses a
// host that is not in `known_hosts`: a tunnel whose far end is not identified protects the
// credentials it carries from nobody, which is the one thing it exists to do.
func hostKeyCallback(config domain.SSHTunnel) (ssh.HostKeyCallback, error) {
	if !config.StrictHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	path := config.KnownHosts
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("%w: find known_hosts: %w", domain.ErrTunnelHostKey, err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", domain.ErrTunnelHostKey, path, err)
	}
	return callback, nil
}

// hostKeyAlgorithms asks known_hosts which host-key algorithms it already holds a key for, and
// answers in that order. **Without it a server offering several host keys is a coin toss.**
//
// A server presents one host key, and which one is negotiated from the client's preference list.
// `x/crypto/ssh` orders that list by its *own* judgement — RSA and ECDSA above Ed25519 — while
// `known_hosts` may hold only one of the three, so the server hands over an RSA key, the check
// finds only an Ed25519 entry, and it reports a **key mismatch** for a server whose key has not
// changed at all. OpenSSH does not have this problem because it puts the algorithms it already
// knows first, and neither does `ssh` on the command line, nor any client built on libssh — which
// is why the same host works everywhere else and fails here.
//
// It is asked by **probing** the callback with a key no server has, because the package exposes
// the entries it holds no other way: the failure it answers with names every algorithm it knows
// for this host. A host that is not in the file at all yields nothing, and the handshake then runs
// on the defaults and fails as "not known", which is the honest outcome.
func hostKeyAlgorithms(callback ssh.HostKeyCallback, address string) []string {
	// An all-zero Ed25519 key. Nothing generates one, so it can never match an entry, and it
	// needs no randomness to make.
	probe, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		return nil
	}

	var mismatch *knownhosts.KeyError
	if !errors.As(callback(address, probeAddr(address), probe), &mismatch) {
		return nil
	}

	algorithms := make([]string, 0, len(mismatch.Want))
	for _, known := range mismatch.Want {
		// A `ssh-rsa` entry is a *key*, and the same key signs under three algorithms. Offering
		// only the entry's own name would ask for SHA-1, which OpenSSH 8.8 and later refuse — so
		// the stronger two go first and the weak one stays for a server too old for them.
		if known.Key.Type() == ssh.KeyAlgoRSA {
			algorithms = append(algorithms, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256)
		}
		algorithms = append(algorithms, known.Key.Type())
	}
	return algorithms
}

// probeAddr is the remote end as the callback insists on being told it, which is a `net.Addr` even
// when the address it actually checks against is the string beside it.
type probeAddr string

func (probeAddr) Network() string  { return "tcp" }
func (a probeAddr) String() string { return string(a) }

// handshakeError tells the two failures apart that a reader answers differently: a host key that
// did not match or was not known is one thing to fix, and a rejected password or key is another.
func handshakeError(config domain.SSHTunnel, err error) error {
	var mismatch *knownhosts.KeyError
	if errors.As(err, &mismatch) {
		return fmt.Errorf("%w: %s: %w", domain.ErrTunnelHostKey, config.Address(), err)
	}
	var revoked *knownhosts.RevokedError
	if errors.As(err, &revoked) {
		return fmt.Errorf("%w: %s: %w", domain.ErrTunnelHostKey, config.Address(), err)
	}
	if isAuthFailure(err) {
		return fmt.Errorf("%w: %s: %w", domain.ErrTunnelCredentials, config.User, err)
	}
	return fmt.Errorf("%w: ssh %s: %w", domain.ErrTunnelFailed, config.Address(), err)
}

// isAuthFailure tells a rejected credential from a tunnel that never came up. x/crypto/ssh gives
// no code to switch on, only its own wording, so this is the one place in the app where a message
// is read — for the same reason an engine adapter reads an errno. It is the library's own text and
// not anything a reader typed, and what the UI switches on is still the code this produces.
func isAuthFailure(err error) bool {
	message := err.Error()
	return strings.Contains(message, "unable to authenticate") ||
		strings.Contains(message, "no supported methods remain")
}
