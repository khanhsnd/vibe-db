package domain

import "errors"

var (
	ErrInvalidProfile    = errors.New("connection profile is invalid")
	ErrProfileNotFound   = errors.New("connection profile not found")
	ErrSessionNotFound   = errors.New("session not found")
	ErrBadCredentials    = errors.New("access denied for user")
	ErrUnknownDatabase   = errors.New("database does not exist")
	ErrUnknownSchema     = errors.New("schema does not exist")
	ErrHostUnreachable   = errors.New("cannot reach database server")
	ErrConnectionTimeout = errors.New("connection timed out")
	ErrTLSUnsupported    = errors.New("server does not support TLS")
	ErrTLSNotVerified    = errors.New("server TLS certificate could not be verified")
	ErrPasswordNotFound  = errors.New("no stored password for profile")
	ErrUnknownTable      = errors.New("table does not exist")
	ErrTableNotEditable  = errors.New("table has no primary key")
	ErrEmptyStatement    = errors.New("no statement to run")
	ErrStatementFailed   = errors.New("statement rejected by the server")
	ErrQueryCancelled    = errors.New("query cancelled")
)

// StatementError carries an engine's own rejection of a statement. Its message is
// the only useful text for the user, so it survives the trip to the UI verbatim.
// Code is the engine's own identifier for the failure and is a string rather than a
// number, because MySQL numbers its errors and PostgreSQL gives a SQLSTATE like `42P01`.
type StatementError struct {
	Code    string
	Message string
}

func (e StatementError) Error() string {
	return e.Message
}

func (e StatementError) Unwrap() error {
	return ErrStatementFailed
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (e ValidationError) Unwrap() error {
	return ErrInvalidProfile
}
