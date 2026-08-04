package app

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func decode(t *testing.T, err error) wireError {
	t.Helper()

	var decoded wireError
	if jsonErr := json.Unmarshal([]byte(err.Error()), &decoded); jsonErr != nil {
		t.Fatalf("wrapped error is not JSON: %v (%q)", jsonErr, err.Error())
	}
	return decoded
}

func TestWrapErrorNilStaysNil(t *testing.T) {
	if got := wrapError(nil); got != nil {
		t.Errorf("wrapError(nil) = %v, want nil", got)
	}
}

func TestWrapErrorMapsSentinels(t *testing.T) {
	tests := []struct {
		in   error
		want ErrorCode
	}{
		{domain.ErrProfileNotFound, CodeProfileNotFound},
		{domain.ErrSessionNotFound, CodeSessionNotFound},
		{domain.ErrBadCredentials, CodeBadCredentials},
		{domain.ErrUnknownDatabase, CodeUnknownDatabase},
		{domain.ErrUnknownTable, CodeUnknownTable},
		{domain.ErrTableNotEditable, CodeTableNotEditable},
		{domain.ErrHostUnreachable, CodeHostUnreachable},
		{domain.ErrConnectionTimeout, CodeTimeout},
		{domain.ErrTLSUnsupported, CodeTLSUnsupported},
		{domain.ErrPasswordNotFound, CodePasswordNotFound},
		{domain.ErrEmptyStatement, CodeEmptyStatement},
		{domain.ErrQueryCancelled, CodeQueryCancelled},
		{domain.ErrTunnelFailed, CodeTunnelFailed},
		{domain.ErrTunnelCredentials, CodeTunnelCredentials},
		{domain.ErrTunnelHostKey, CodeTunnelHostKey},
		{domain.ErrTokenUnavailable, CodeTokenUnavailable},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			got := decode(t, wrapError(fmt.Errorf("context: %w", tt.in)))

			if got.Code != tt.want {
				t.Errorf("code = %q, want %q", got.Code, tt.want)
			}
			if got.Message == "" {
				t.Error("message must not be empty; the UI shows it verbatim")
			}
		})
	}
}

func TestWrapErrorCarriesValidationField(t *testing.T) {
	err := domain.ValidationError{Field: "port", Message: "must be between 1 and 65535"}

	got := decode(t, wrapError(err))

	if got.Code != CodeInvalidProfile {
		t.Errorf("code = %q, want %q", got.Code, CodeInvalidProfile)
	}
	if got.Field != "port" {
		t.Errorf("field = %q, want %q", got.Field, "port")
	}
}

func TestWrapErrorKeepsTheServerWordingOfAStatementError(t *testing.T) {
	err := domain.StatementError{Code: "1054", Message: "Unknown column 'nope' in 'field list'"}

	got := decode(t, wrapError(fmt.Errorf("run: %w", err)))

	if got.Code != CodeSQLError {
		t.Errorf("code = %q, want %q", got.Code, CodeSQLError)
	}
	if got.Message != err.Message {
		t.Errorf("message = %q, want the server's wording %q", got.Message, err.Message)
	}
	if got.Detail != "error 1054" {
		t.Errorf("detail = %q, want the engine's own error code", got.Detail)
	}
}

func TestWrapErrorFallsBackToInternal(t *testing.T) {
	got := decode(t, wrapError(fmt.Errorf("some driver explosion")))

	if got.Code != CodeInternal {
		t.Errorf("code = %q, want %q", got.Code, CodeInternal)
	}
	if got.Detail != "some driver explosion" {
		t.Errorf("detail = %q, want the original message preserved", got.Detail)
	}
}

// A thing an engine has no concept of is not a failure the reader can retry, so it has a code of
// its own and a message naming which thing.
func TestWrapErrorNamesWhatAnEngineDoesNotSupport(t *testing.T) {
	got := decode(t, wrapError(fmt.Errorf("add: %w", domain.Unsupported("a foreign key"))))

	if got.Code != CodeUnsupported {
		t.Errorf("code = %q, want %q", got.Code, CodeUnsupported)
	}
	if got.Message != "This database does not support a foreign key." {
		t.Errorf("message = %q, want it to name the thing", got.Message)
	}
}
