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

	// ErrFileUnreadable is a file on this machine the app was pointed at and could not read: a
	// path that is no longer there, or one this account may not open. Nothing was sent to a
	// server, and the reader answers it in the file picker rather than in the connection.
	ErrFileUnreadable = errors.New("cannot read the file")

	// ErrUnsupported is a thing this engine has no concept of rather than a thing that went
	// wrong: a foreign key on MongoDB, a primary key other than `_id`, a stored routine. It is
	// its own sentinel because a reader answers it by not asking again, and because the alternative
	// — letting the server reject it — would report a design decision as a server failure.
	ErrUnsupported = errors.New("this engine does not support that")

	// The three ways a connection through a bastion fails before the database is ever reached.
	// They are separate sentinels because a reader answers each of them differently: one is a
	// host that is not there, one is a key or a password the bastion refused, and one is the
	// bastion not being the bastion — which is the only one of the three that may mean something
	// is wrong rather than mistyped.
	ErrTunnelFailed      = errors.New("cannot reach the SSH server")
	ErrTunnelCredentials = errors.New("SSH server rejected the credentials")
	ErrTunnelHostKey     = errors.New("SSH host key is not known or does not match")

	// ErrTokenUnavailable is a cloud identity this machine could not produce a token from. It is
	// not a rejection by the database — nothing was sent to one — and the reader fixes it where
	// their cloud credentials live rather than in the profile.
	ErrTokenUnavailable = errors.New("no cloud credentials to sign in with")
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

// FileError is a file on this machine that could not be opened, read or written, carrying the
// reason the operating system gave. It unwraps to ErrFileUnreadable, which is what the UI switches
// on: *which* file and *why* is detail, and the reader answers all of it in the same picker.
type FileError struct {
	Err error
}

func (e FileError) Error() string {
	return ErrFileUnreadable.Error() + ": " + e.Err.Error()
}

func (e FileError) Unwrap() error {
	return ErrFileUnreadable
}

// UnsupportedError names the thing this engine has no concept of, so the reader is told which
// one rather than only that something was refused.
type UnsupportedError struct {
	What string
}

func (e UnsupportedError) Error() string {
	return e.What + ": not supported by this engine"
}

func (e UnsupportedError) Unwrap() error {
	return ErrUnsupported
}

// Unsupported refuses a request this engine has no concept of: `Unsupported("a foreign key")`.
func Unsupported(what string) error {
	return UnsupportedError{What: what}
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
