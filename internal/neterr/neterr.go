// Package neterr says what a failure that happened before any protocol was spoken means: a
// cancelled read, a deadline, a host that is not there, a certificate this machine will not trust.
//
// It sits beside `internal/tlscert` for the same reason that package exists: **none of those four
// is about a database.** They read identically to an object store, so `internal/driver` does not own
// them and `internal/objectstore` does not copy them.
//
// It lived in `sqlkit` as `TransportError` until an object store needed it. What it does *not* map is
// still the caller's: only an adapter can read its own wire errors, which is why this answers nil
// for anything it does not recognise rather than guessing.
package neterr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Map maps the failures that read the same whatever is at the other end — a cancelled
// context, a deadline, an unreachable host, a certificate that does not verify — onto
// domain sentinels. It returns nil for anything else, which is the adapter's own to
// translate because only it can read its own wire errors.
func Map(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return domain.ErrQueryCancelled
	case isCertVerificationError(err):
		return domain.ErrTLSNotVerified
	case errors.Is(err, context.DeadlineExceeded):
		return domain.ErrConnectionTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return domain.ErrConnectionTimeout
		}
		return domain.ErrHostUnreachable
	}

	return nil
}

func isCertVerificationError(err error) bool {
	var verificationErr *tls.CertificateVerificationError
	if errors.As(err, &verificationErr) {
		return true
	}

	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return true
	}

	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return true
	}

	var invalidErr x509.CertificateInvalidError
	return errors.As(err, &invalidErr)
}
