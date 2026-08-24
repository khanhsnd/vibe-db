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
	// ErrRowGone is a write whose key named no stored row. It is its own sentinel because the table
	// is there and the *row* is not, and it exists because a write that matched nothing is otherwise
	// indistinguishable from one that worked: a document store reports no error for it, so the edit
	// would be discarded, the page read again, and the reader shown their change quietly undone.
	ErrRowGone         = errors.New("row is no longer there")
	ErrEmptyStatement  = errors.New("no statement to run")
	ErrStatementFailed = errors.New("statement rejected by the server")
	ErrQueryCancelled  = errors.New("query cancelled")
	// ErrRunDeadline is **this app's own clock** running out on something the reader asked for, and
	// it is its own sentinel because it is not what ErrConnectionTimeout says. A timeout is a server
	// that did not answer; this is a server that is still working and a client that stopped waiting —
	// the statement may well be running on the far side, and nothing here can say whether it
	// finished. Reporting it as the server's silence sends a reader to check a server that is fine.
	//
	// It is also what a *cancellation* means when nobody pressed Stop: PostgreSQL answers a cut-off
	// statement with `57014`, which is the same code its own `statement_timeout` uses, so a clock
	// this app set would otherwise be reported as the reader's own doing.
	ErrRunDeadline = errors.New("this app stopped waiting for the run")

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

	// The two ways a connection through a proxy fails before the far side is ever reached. They are
	// separate sentinels for the reason the tunnel's are: one is a proxy that is not there or would
	// not open the connection, and one is a proxy that refused the reader — and the second is fixed
	// in a different box.
	ErrProxyFailed      = errors.New("cannot reach the proxy")
	ErrProxyCredentials = errors.New("proxy rejected the credentials")

	// ErrReadOnly is something this app refused to send because the connection was opened
	// read-only. Nothing reached the server, which is the whole point: it is not a server's
	// rejection and there is nothing to retry — see Access.
	ErrReadOnly = errors.New("this connection is read-only")

	// ErrTokenUnavailable is a cloud identity this machine could not produce a token from. It is
	// not a rejection by the database — nothing was sent to one — and the reader fixes it where
	// their cloud credentials live rather than in the profile.
	ErrTokenUnavailable = errors.New("no cloud credentials to sign in with")

	// The two ways a scheduled job is asked for something it cannot give. They are separate
	// sentinels because a reader answers them differently: the first is a task that has been
	// deleted — in another window, or since the list was drawn — and the second is a task the
	// reader pressed Run on while it was already going, which is a wait rather than a mistake.
	ErrTaskNotFound = errors.New("scheduled task not found")
	ErrTaskRunning  = errors.New("that task is already running")

	// The two ways running one of an engine's own programs fails — see Toolkit. They are separate
	// sentinels because a reader answers them in different places: the first is a program that is
	// not on this machine, which is fixed by installing it or pointing the app at it, and the
	// second is the program having run and refused, which is answered in whatever it printed.
	ErrToolNotFound = errors.New("that program is not on this machine")
	ErrToolFailed   = errors.New("the program refused")

	// The three ways an object store refuses a read — see StoreProfile. They are separate sentinels
	// because a reader answers each of them somewhere else: a bucket that is not there is a name to
	// correct, an object that is not there is a listing to refresh, and a request the credentials
	// may not make is a *policy* — the key is good and this is not its bucket, which is a different
	// sentence from "check the user name and password" and belongs in a different place.
	ErrUnknownBucket = errors.New("bucket does not exist")
	ErrUnknownObject = errors.New("object does not exist")
	ErrForbidden     = errors.New("these credentials may not do that")

	// ErrServiceRefused is a service having read the request and said no, and it is StatementError's
	// counterpart for something that is not a statement: the service's own code and its own words
	// are the only useful ones, exactly as an engine's are about a `SELECT` it rejected.
	ErrServiceRefused = errors.New("the service refused the request")
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

// ServiceError carries an object store's own rejection of a request, and it is StatementError's
// counterpart: the same arrangement for something that is not a statement. `Code` is the service's
// own word for the failure — `NoSuchUpload`, `EntityTooLarge`, `SlowDown` — and `Message` is the
// sentence it wrote, which reaches the reader verbatim because the service knows why it refused.
type ServiceError struct {
	Code    string
	Message string
}

func (e ServiceError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Message
}

func (e ServiceError) Unwrap() error {
	return ErrServiceRefused
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

// ReadOnlyError names what was refused, so the reader is told which thing was not sent rather than
// only that something was held back. What it names is the reader's own word for it — the statement's
// keyword, or the change the tree asked for.
type ReadOnlyError struct {
	What string
}

func (e ReadOnlyError) Error() string {
	return e.What + ": " + ErrReadOnly.Error()
}

func (e ReadOnlyError) Unwrap() error {
	return ErrReadOnly
}

// ReadOnly refuses a request that would write on a read-only connection: `ReadOnly("DROP TABLE")`.
func ReadOnly(what string) error {
	return ReadOnlyError{What: what}
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
