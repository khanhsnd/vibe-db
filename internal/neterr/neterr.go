package sqlkit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// TransportError maps the failures that read the same whatever the engine — a cancelled
// context, a deadline, an unreachable host, a certificate that does not verify — onto
// domain sentinels. It returns nil for anything else, which is the adapter's own to
// translate because only it can read the engine's wire errors.
func TransportError(err error) error {
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
