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
	// ErrUnknownDefinition is a view or a routine that is no longer there. It is its own sentinel
	// rather than ErrUnknownTable because a reader is answered by *which* thing is missing, and
	// "that table no longer exists" is the wrong sentence about a procedure. It is also what an
	// adapter answers instead of an empty definition, since a save would then have spelled the
	// drop with nothing after it.
	ErrUnknownDefinition = errors.New("view or routine does not exist")
	// ErrUnknownAccount is a user or a role the server does not know. It is its own sentinel for
	// the reason ErrUnknownDefinition is: a reader is answered by *which* thing is missing, and it
	// is what an adapter answers instead of an empty detail — a save built on one would spell a
	// revoke of everything the account never had.
	ErrUnknownAccount   = errors.New("user or role does not exist")
	ErrTableNotEditable = errors.New("table has no primary key")
	ErrEmptyStatement   = errors.New("no statement to run")
	ErrStatementFailed  = errors.New("statement rejected by the server")
	ErrQueryCancelled   = errors.New("query cancelled")

	// ErrFileUnreadable is a file on this machine the app was pointed at and could not read: a
	// path that is no longer there, or one this account may not open. Nothing was sent to a
	// server, and the reader answers it in the file picker rather than in the connection.
	ErrFileUnreadable = errors.New("cannot read the file")

	// The two ways a manual transaction is asked for something it is not in a state to give. They
	// are refusals rather than no-ops because the reader is being told what is *open*: a Begin that
	// quietly joined the transaction already running would hide a second one they thought they had,
	// and a Commit with nothing open would report having written what was never held.
	ErrTransactionActive = errors.New("a transaction is already open on this connection")
	ErrNoTransaction     = errors.New("no transaction is open on this connection")

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

	// The two ways running one of an engine's own programs fails — see Toolkit. They are separate
	// sentinels because a reader answers them in different places: the first is a program that is
	// not on this machine, which is fixed by installing it or pointing the app at it, and the
	// second is the program having run and refused, which is answered in whatever it printed.
	ErrToolNotFound = errors.New("that program is not on this machine")
	ErrToolFailed   = errors.New("the program refused")
)

// ToolNotFoundError names the program that could not be found, so the reader is told which of the
// two to install rather than only that something is missing.
type ToolNotFoundError struct {
	Tool string
}

func (e ToolNotFoundError) Error() string {
	return e.Tool + ": " + ErrToolNotFound.Error()
}

func (e ToolNotFoundError) Unwrap() error {
	return ErrToolNotFound
}

// ToolFailedError is one of an engine's own programs having run and ended badly. It carries the
// program's *own* last words, which are the only useful ones: mongorestore knows why it refused a
// document and this app does not.
type ToolFailedError struct {
	Tool string
	// Status is what the program exited with, and 0 is a failure that was not an exit status at
	// all — a cancelled run, or a program that could not be started.
	Status  int
	Message string
}

func (e ToolFailedError) Error() string {
	if e.Message == "" {
		return e.Tool + ": " + ErrToolFailed.Error()
	}
	return e.Tool + ": " + e.Message
}

func (e ToolFailedError) Unwrap() error {
	return ErrToolFailed
}

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
