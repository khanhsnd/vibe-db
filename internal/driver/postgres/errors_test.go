package postgres

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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
			"28P01 is bad credentials",
			&pgconn.PgError{Code: "28P01", Message: "password authentication failed"},
			domain.ErrBadCredentials,
		},
		{
			"28000 is bad credentials",
			&pgconn.PgError{Code: "28000", Message: "no pg_hba.conf entry"},
			domain.ErrBadCredentials,
		},
		{
			"3D000 is an unknown database",
			&pgconn.PgError{Code: "3D000", Message: `database "nope" does not exist`},
			domain.ErrUnknownDatabase,
		},
		{
			"3F000 is an unknown schema, which is its own level of the tree",
			&pgconn.PgError{Code: "3F000", Message: `schema "nope" does not exist`},
			domain.ErrUnknownSchema,
		},
		{
			"42P01 is an unknown table",
			&pgconn.PgError{Code: "42P01", Message: `relation "nope" does not exist`},
			domain.ErrUnknownTable,
		},
		{
			"57014 is a cancelled query",
			&pgconn.PgError{Code: "57014", Message: "canceling statement due to user request"},
			domain.ErrQueryCancelled,
		},
		{"a wrapped server error keeps its meaning",
			fmt.Errorf("query: %w", &pgconn.PgError{Code: "42P01", Message: "nope"}),
			domain.ErrUnknownTable,
		},
		{"a cancelled context is a cancelled query", context.Canceled, domain.ErrQueryCancelled},
		{"a deadline is a timeout", context.DeadlineExceeded, domain.ErrConnectionTimeout},
		{
			"a refused TLS connection is an unsupported one",
			errors.New("failed to connect: server refused TLS connection"),
			domain.ErrTLSUnsupported,
		},
		{
			"an untrusted certificate is not verified",
			x509.UnknownAuthorityError{},
			domain.ErrTLSNotVerified,
		},
		{
			"a refused dial is an unreachable host",
			&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			domain.ErrHostUnreachable,
		},
		{"a network timeout is a timeout", timeoutError{}, domain.ErrConnectionTimeout},
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

func TestTranslateErrorKeepsUnknownServerErrorsAsStatementErrors(t *testing.T) {
	original := &pgconn.PgError{Code: "42601", Message: `syntax error at or near "slect"`}

	got := translateError(original)

	if !errors.Is(got, domain.ErrStatementFailed) {
		t.Fatalf("translateError() = %v, want a statement failure", got)
	}

	var statementErr domain.StatementError
	if !errors.As(got, &statementErr) {
		t.Fatalf("translateError() = %v, want a domain.StatementError", got)
	}
	if statementErr.Code != "42601" {
		t.Errorf("code = %q, want the SQLSTATE 42601", statementErr.Code)
	}
	if statementErr.Message != original.Message {
		t.Errorf("message = %q, want the server's wording %q", statementErr.Message, original.Message)
	}
}
