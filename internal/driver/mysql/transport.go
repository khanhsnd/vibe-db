package mysql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"

	"github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// builtInTLS are the names go-sql-driver already answers to, and they cover every mode that needs
// no file of ours: off, opportunistic, and encrypt-without-checking. `skip-verify` is what
// `required` means here — the connection is encrypted and the server is not held to anything —
// and anything above it is a configuration this side has to build and register.
var builtInTLS = map[domain.TLSMode]string{
	domain.TLSDisabled:  "false",
	domain.TLSPreferred: "preferred",
	domain.TLSRequired:  "skip-verify",
}

// transport is what a DSN has to be able to *name*. go-sql-driver takes neither a dialler nor a
// TLS configuration in its connection string: both are registered globally under a name, and the
// DSN refers to them by it. Both registrations therefore belong to one session and are forgotten
// when it closes, which is what `release` is for.
type transport struct {
	net string
	tls string

	release func()
}

// dial registers whatever this profile needs and answers with the names its DSN will carry. A
// profile going straight out over TCP with a built-in TLS mode registers nothing at all, which is
// every profile that existed before this file.
func dial(profile domain.ConnectionProfile, tunnel driver.Dialer) (transport, error) {
	t := transport{net: "tcp", release: func() {}}
	released := []func(){}

	fail := func(err error) (transport, error) {
		for _, undo := range released {
			undo()
		}
		return transport{}, err
	}

	key, err := registrationKey()
	if err != nil {
		return fail(err)
	}

	if tunnel != nil {
		name := "vibe-" + key
		mysql.RegisterDialContext(name, func(ctx context.Context, address string) (net.Conn, error) {
			return tunnel(ctx, "tcp", address)
		})
		released = append(released, func() { mysql.DeregisterDialContext(name) })
		t.net = name
	}

	if name, ok := builtInTLS[profile.TLS.Mode]; ok {
		t.tls = name
	} else {
		config, err := sqlkit.TLSConfig(profile.TLS, profile.Host)
		if err != nil {
			return fail(err)
		}
		name := "vibe-" + key
		if err := mysql.RegisterTLSConfig(name, config); err != nil {
			return fail(err)
		}
		released = append(released, func() { mysql.DeregisterTLSConfig(name) })
		t.tls = name
	}

	t.release = func() {
		for _, undo := range released {
			undo()
		}
	}
	return t, nil
}

// registrationKey is what keeps two sessions of one profile from sharing a registration and
// deregistering it out from under each other.
func registrationKey() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
