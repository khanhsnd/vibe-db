package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

type ErrorCode string

const (
	CodeInvalidProfile   ErrorCode = "INVALID_PROFILE"
	CodeInvalidName      ErrorCode = "INVALID_NAME"
	CodeProfileNotFound  ErrorCode = "PROFILE_NOT_FOUND"
	CodeSessionNotFound  ErrorCode = "SESSION_NOT_FOUND"
	CodeBadCredentials   ErrorCode = "BAD_CREDENTIALS"
	CodeUnknownDatabase  ErrorCode = "UNKNOWN_DATABASE"
	CodeUnknownSchema    ErrorCode = "UNKNOWN_SCHEMA"
	CodeUnknownTable     ErrorCode = "UNKNOWN_TABLE"
	CodeTableNotEditable ErrorCode = "TABLE_NOT_EDITABLE"
	CodeEmptyStatement   ErrorCode = "EMPTY_STATEMENT"
	CodeSQLError         ErrorCode = "SQL_ERROR"
	CodeQueryCancelled   ErrorCode = "QUERY_CANCELLED"
	CodeHostUnreachable  ErrorCode = "HOST_UNREACHABLE"
	CodeTimeout          ErrorCode = "TIMEOUT"
	CodeTLSUnsupported   ErrorCode = "TLS_UNSUPPORTED"
	CodeTLSNotVerified   ErrorCode = "TLS_NOT_VERIFIED"
	CodePasswordNotFound ErrorCode = "PASSWORD_NOT_FOUND"

	CodeTunnelFailed      ErrorCode = "TUNNEL_FAILED"
	CodeTunnelCredentials ErrorCode = "TUNNEL_CREDENTIALS"
	CodeTunnelHostKey     ErrorCode = "TUNNEL_HOST_KEY"
	CodeTokenUnavailable  ErrorCode = "TOKEN_UNAVAILABLE"

	CodeInternal ErrorCode = "INTERNAL"
)

type wireError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

func (e wireError) Error() string {
	encoded, err := json.Marshal(e)
	if err != nil {
		return string(e.Code) + ": " + e.Message
	}
	return string(encoded)
}

var sentinelToWire = []struct {
	target  error
	code    ErrorCode
	message string
}{
	{domain.ErrProfileNotFound, CodeProfileNotFound, "This connection no longer exists."},
	{domain.ErrSessionNotFound, CodeSessionNotFound, "This connection is no longer open."},
	{domain.ErrBadCredentials, CodeBadCredentials, "Access denied. Check the user name and password."},
	{domain.ErrUnknownDatabase, CodeUnknownDatabase, "That database does not exist on the server."},
	{domain.ErrUnknownSchema, CodeUnknownSchema, "That schema no longer exists. Refresh the database."},
	{domain.ErrUnknownTable, CodeUnknownTable, "That table no longer exists. Refresh the table list."},
	{domain.ErrTableNotEditable, CodeTableNotEditable, "This table has no primary key, so a single row cannot be identified for editing."},
	{domain.ErrEmptyStatement, CodeEmptyStatement, "There is no statement to run."},
	{domain.ErrQueryCancelled, CodeQueryCancelled, "The query was cancelled."},
	{domain.ErrHostUnreachable, CodeHostUnreachable, "Could not reach the server. Check the host and port."},
	{domain.ErrConnectionTimeout, CodeTimeout, "The server did not respond in time."},
	{domain.ErrTLSUnsupported, CodeTLSUnsupported, "The server does not support TLS. Try a lower TLS mode."},
	{domain.ErrTLSNotVerified, CodeTLSNotVerified, "The server's TLS certificate is not trusted. Use TLS mode Required to encrypt without verifying it."},
	{domain.ErrPasswordNotFound, CodePasswordNotFound, "No saved password. Enter the password to connect."},
	{domain.ErrTunnelFailed, CodeTunnelFailed, "Could not reach the SSH server. Check the tunnel's host and port."},
	{domain.ErrTunnelCredentials, CodeTunnelCredentials, "The SSH server rejected the credentials. Check the user, the key and its pass phrase."},
	{domain.ErrTunnelHostKey, CodeTunnelHostKey, "The SSH server's host key is not in known_hosts, or does not match the one stored there."},
	{domain.ErrTokenUnavailable, CodeTokenUnavailable, "No cloud credentials on this machine to sign in with. The detail says what to set."},
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	var validationErr domain.ValidationError
	if errors.As(err, &validationErr) {
		return wireError{
			Code:    CodeInvalidProfile,
			Message: fmt.Sprintf("The %s field %s.", validationErr.Field, validationErr.Message),
			Field:   validationErr.Field,
		}
	}

	// A blank identifier is named by what it was for, so the dialog can say which box is
	// empty without reading the message.
	var nameErr domain.NameError
	if errors.As(err, &nameErr) {
		return wireError{
			Code:    CodeInvalidName,
			Message: "A " + nameErr.What + " name is required.",
			Field:   nameErr.What,
		}
	}

	// The server's own wording is the only useful message for a rejected
	// statement, so it reaches the UI verbatim.
	var statementErr domain.StatementError
	if errors.As(err, &statementErr) {
		return wireError{
			Code:    CodeSQLError,
			Message: statementErr.Message,
			Detail:  "error " + statementErr.Code,
		}
	}

	for _, mapping := range sentinelToWire {
		if errors.Is(err, mapping.target) {
			return wireError{Code: mapping.code, Message: mapping.message, Detail: err.Error()}
		}
	}

	return wireError{
		Code:    CodeInternal,
		Message: "Something went wrong while talking to the database.",
		Detail:  err.Error(),
	}
}
