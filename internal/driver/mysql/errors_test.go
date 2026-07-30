package mysql

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestTranslateError(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil stays nil", nil, nil},
		{
			"errno 1045 is bad credentials",
			&gomysql.MySQLError{Number: 1045, Message: "Access denied"},
			domain.ErrBadCredentials,
		},
		{
			"errno 1698 is bad credentials",
			&gomysql.MySQLError{Number: 1698, Message: "Access denied"},
			domain.ErrBadCredentials,
		},
		{
			"errno 1049 is unknown database",
			&gomysql.MySQLError{Number: 1049, Message: "Unknown database"},
			domain.ErrUnknownDatabase,
		},
		{"no TLS support", gomysql.ErrNoTLS, domain.ErrTLSUnsupported},
		{
			"untrusted certificate authority",
			&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}},
			domain.ErrTLSNotVerified,
		},
		{
			"certificate hostname mismatch",
			fmt.Errorf("handshake: %w", x509.HostnameError{Host: "db.internal"}),
			domain.ErrTLSNotVerified,
		},
		{"deadline exceeded", context.DeadlineExceeded, domain.ErrConnectionTimeout},
		{
			"wrapped deadline exceeded",
			fmt.Errorf("dial: %w", context.DeadlineExceeded),
			domain.ErrConnectionTimeout,
		},
		{"net timeout", timeoutError{}, domain.ErrConnectionTimeout},
		{
			"connection refused is unreachable",
			&net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
			domain.ErrHostUnreachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateError(tt.in)

			if tt.want == nil {
				if got != nil {
					t.Fatalf("translateError() = %v, want nil", got)
				}
				return
			}
			if !errors.Is(got, tt.want) {
				t.Errorf("translateError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTranslateErrorPassesThroughUnknownServerErrors(t *testing.T) {
	original := &gomysql.MySQLError{Number: 1064, Message: "You have an error in your SQL syntax"}

	got := translateError(original)

	if !errors.Is(got, original) {
		t.Errorf("translateError() = %v, want the original error preserved", got)
	}
}
