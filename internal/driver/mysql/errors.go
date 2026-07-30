package mysql

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
)

const (
	errAccessDenied         = 1045
	errUnknownDatabase      = 1049
	errUnknownTable         = 1146
	errAccessDeniedNoPasswd = 1698
)

var errnoToDomain = map[uint16]error{
	errAccessDenied:         domain.ErrBadCredentials,
	errAccessDeniedNoPasswd: domain.ErrBadCredentials,
	errUnknownDatabase:      domain.ErrUnknownDatabase,
	errUnknownTable:         domain.ErrUnknownTable,
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		if mapped, ok := errnoToDomain[mysqlErr.Number]; ok {
			return mapped
		}
		return domain.StatementError{Number: int(mysqlErr.Number), Message: mysqlErr.Message}
	}

	if errors.Is(err, context.Canceled) {
		return domain.ErrQueryCancelled
	}

	if errors.Is(err, mysql.ErrNoTLS) {
		return domain.ErrTLSUnsupported
	}

	if isCertVerificationError(err) {
		return domain.ErrTLSNotVerified
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrConnectionTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return domain.ErrConnectionTimeout
		}
		return domain.ErrHostUnreachable
	}

	return err
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
