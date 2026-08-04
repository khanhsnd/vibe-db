package sshtunnel

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// writeKnownHosts puts one line per key on disk for `host`, and answers with the file's path.
func writeKnownHosts(t *testing.T, host string, keys ...ssh.PublicKey) string {
	t.Helper()

	lines := ""
	for _, key := range keys {
		lines += knownhosts.Line([]string{host}, key) + "\n"
	}

	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func ed25519Key(t *testing.T) ssh.PublicKey {
	t.Helper()

	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}
	return key
}

func ecdsaKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	key, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}
	return key
}

func rsaKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	key, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}
	return key
}

func callbackFor(t *testing.T, path string) ssh.HostKeyCallback {
	t.Helper()

	callback, err := hostKeyCallback(domain.SSHTunnel{StrictHostKey: true, KnownHosts: path})
	if err != nil {
		t.Fatalf("hostKeyCallback() error = %v", err)
	}
	return callback
}

// The failure this exists for: a server offers RSA, ECDSA *and* Ed25519, known_hosts holds only
// the Ed25519 one, and the client must therefore ask for Ed25519. Left to its own preference,
// `x/crypto/ssh` asks for RSA, is handed a key known_hosts has never seen, and reports a key
// mismatch for a server whose key has not changed at all.
func TestHostKeyAlgorithmsAsksForTheKeyKnownHostsHolds(t *testing.T) {
	const address = "bastion.example.com:22"
	path := writeKnownHosts(t, address, ed25519Key(t))

	got := hostKeyAlgorithms(callbackFor(t, path), address)

	if !slices.Equal(got, []string{ssh.KeyAlgoED25519}) {
		t.Errorf("hostKeyAlgorithms() = %v, want only %q", got, ssh.KeyAlgoED25519)
	}
}

// An `ssh-rsa` entry is a key, not a signature algorithm. Asking for `ssh-rsa` alone asks for
// SHA-1, which OpenSSH 8.8 and later refuse outright.
func TestHostKeyAlgorithmsOffersSHA2ForAnRSAEntry(t *testing.T) {
	const address = "bastion.example.com:22"
	path := writeKnownHosts(t, address, rsaKey(t))

	got := hostKeyAlgorithms(callbackFor(t, path), address)

	want := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	if !slices.Equal(got, want) {
		t.Errorf("hostKeyAlgorithms() = %v, want %v", got, want)
	}
}

func TestHostKeyAlgorithmsOffersEveryEntryItHolds(t *testing.T) {
	const address = "bastion.example.com:22"
	path := writeKnownHosts(t, address, ed25519Key(t), ecdsaKey(t))

	got := hostKeyAlgorithms(callbackFor(t, path), address)

	for _, want := range []string{ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256} {
		if !slices.Contains(got, want) {
			t.Errorf("hostKeyAlgorithms() = %v, want it to offer %q", got, want)
		}
	}
}

// A host that is not in the file yields nothing, and the handshake then runs on the defaults and
// fails as "not known" — which is the truth of it, and a different message to the reader.
func TestHostKeyAlgorithmsIsEmptyForAnUnknownHost(t *testing.T) {
	path := writeKnownHosts(t, "other.example.com:22", ed25519Key(t))

	if got := hostKeyAlgorithms(callbackFor(t, path), "bastion.example.com:22"); len(got) != 0 {
		t.Errorf("hostKeyAlgorithms() = %v, want nothing", got)
	}
}

// With the host-key check turned off there is no file to read, and the handshake keeps the
// library's own preference.
func TestHostKeyAlgorithmsIsEmptyWhenNothingIsChecked(t *testing.T) {
	callback, err := hostKeyCallback(domain.SSHTunnel{StrictHostKey: false})
	if err != nil {
		t.Fatalf("hostKeyCallback() error = %v", err)
	}

	if got := hostKeyAlgorithms(callback, "bastion.example.com:22"); len(got) != 0 {
		t.Errorf("hostKeyAlgorithms() = %v, want nothing", got)
	}
}

// The probe must never be mistaken for a real check: it says which algorithms are known and
// nothing about whether a key is acceptable.
func TestTheProbeKeyMatchesNothing(t *testing.T) {
	const address = "bastion.example.com:22"
	real := ed25519Key(t)
	path := writeKnownHosts(t, address, real)

	callback := callbackFor(t, path)
	if err := callback(address, probeAddr(address), real); err != nil {
		t.Errorf("the stored key was rejected: %v", err)
	}

	probe, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}
	if callback(address, probeAddr(address), probe) == nil {
		t.Error("the probe key was accepted, which would make every algorithm list empty")
	}
}
