package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

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
	CodeUnknownSource    ErrorCode = "UNKNOWN_DEFINITION"
	CodeUnknownAccount   ErrorCode = "UNKNOWN_ACCOUNT"
	CodeTableNotEditable ErrorCode = "TABLE_NOT_EDITABLE"
	// CodeRowGone is a write whose key named no stored row. It is its own code because the answer is
	// to refresh and look again, not to retry the write — see domain.ErrRowGone.
	CodeRowGone        ErrorCode = "ROW_GONE"
	CodeEmptyStatement ErrorCode = "EMPTY_STATEMENT"
	// CodeEmptySnippet is a library entry saved with no body. It is its own code rather than
	// EMPTY_STATEMENT because nothing was being run — see docs/features/snippets.md.
	CodeEmptySnippet ErrorCode = "EMPTY_SNIPPET"
	CodeSQLError     ErrorCode = "SQL_ERROR"
	// The two ways a manual transaction is asked for something it is not in a state to give. They
	// are their own codes because the reader answers each by looking at the button: one says there
	// is already one open, and the other that there is not — see docs/features/transaction.md.
	CodeTransactionActive ErrorCode = "TRANSACTION_ACTIVE"
	CodeNoTransaction     ErrorCode = "NO_TRANSACTION"
	// CodeFileUnreadable is a file on this machine, not an answer from a server — see
	// domain.ErrFileUnreadable.
	CodeFileUnreadable ErrorCode = "FILE_UNREADABLE"
	// CodeUnsupported is a thing this engine has no concept of rather than a thing that went
	// wrong — a foreign key on MongoDB, a primary key other than `_id`. It is its own code so the
	// UI can say so plainly instead of drawing it as a failure the reader could retry.
	CodeUnsupported      ErrorCode = "UNSUPPORTED"
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

	// The two ways the hop in front of everything else fails. They are their own codes for the
	// reason the tunnel's are: the reader fixes each of them in a different box.
	CodeProxyFailed      ErrorCode = "PROXY_FAILED"
	CodeProxyCredentials ErrorCode = "PROXY_CREDENTIALS"

	// CodeReadOnly is something this app refused to send because the connection was opened
	// read-only. It is its own code so the UI can say so plainly rather than drawing it as a
	// failure the reader could retry — nothing reached the server, and retrying is not the answer.
	CodeReadOnly ErrorCode = "READ_ONLY"

	// The two ways one of an engine's *own* programs fails. Neither is a server saying anything:
	// the first is a program this machine has not got, and the second is that program's own
	// verdict, carried through in its words.
	CodeToolNotFound ErrorCode = "TOOL_NOT_FOUND"
	CodeToolFailed   ErrorCode = "TOOL_FAILED"
	CodeUpdateFailed ErrorCode = "UPDATE_FAILED"

	// The two ways a scheduled job is asked for something it cannot give. They are their own codes
	// because the reader answers each differently: the first is a job that has been deleted, and
	// the second is one they pressed Run on while it was already going — see
	// docs/features/automation.md.
	CodeTaskNotFound ErrorCode = "TASK_NOT_FOUND"
	CodeTaskRunning  ErrorCode = "TASK_RUNNING"

	// The four ways an **object store** answers that it will not do something. They are their own
	// codes because the reader answers each of them somewhere else: a bucket name to correct, a
	// listing to refresh, a *policy* that has nothing to do with the key being right, and the
	// service's own refusal in its own words — see docs/features/object-storage.md.
	CodeUnknownBucket  ErrorCode = "UNKNOWN_BUCKET"
	CodeUnknownObject  ErrorCode = "UNKNOWN_OBJECT"
	CodeForbidden      ErrorCode = "FORBIDDEN"
	CodeServiceRefused ErrorCode = "SERVICE_REFUSED"

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
	{domain.ErrUnknownDefinition, CodeUnknownSource, "That view or routine no longer exists. Refresh the folder it was in."},
	{domain.ErrUnknownAccount, CodeUnknownAccount, "That user no longer exists on this server. Refresh the list."},
	{domain.ErrTableNotEditable, CodeTableNotEditable, "This table has no primary key, so a single row cannot be identified for editing."},
	{domain.ErrRowGone, CodeRowGone, "That row is no longer there. Refresh the table and make the change again."},
	{domain.ErrEmptyStatement, CodeEmptyStatement, "There is no statement to run."},
	{domain.ErrEmptySnippet, CodeEmptySnippet, "A snippet needs a statement in it."},
	{domain.ErrTransactionActive, CodeTransactionActive, "A transaction is already open on this connection. Commit or roll it back first."},
	{domain.ErrNoTransaction, CodeNoTransaction, "No transaction is open on this connection."},
	{domain.ErrQueryCancelled, CodeQueryCancelled, "The query was cancelled."},
	{domain.ErrFileUnreadable, CodeFileUnreadable, "That file could not be read. Check it is still there and that this account can open it."},
	{domain.ErrHostUnreachable, CodeHostUnreachable, "Could not reach the server. Check the host and port."},
	{domain.ErrConnectionTimeout, CodeTimeout, "The server did not respond in time."},
	{domain.ErrTLSUnsupported, CodeTLSUnsupported, "The server does not support TLS. Try a lower TLS mode."},
	{domain.ErrTLSNotVerified, CodeTLSNotVerified, "The server's TLS certificate is not trusted. Use TLS mode Required to encrypt without verifying it."},
	{domain.ErrPasswordNotFound, CodePasswordNotFound, "No saved password. Enter the password to connect."},
	{domain.ErrTunnelFailed, CodeTunnelFailed, "Could not reach the SSH server. Check the tunnel's host and port."},
	{domain.ErrTunnelCredentials, CodeTunnelCredentials, "The SSH server rejected the credentials. Check the user, the key and its pass phrase."},
	{domain.ErrTunnelHostKey, CodeTunnelHostKey, "The SSH server's host key is not in known_hosts, or does not match the one stored there."},
	{domain.ErrTokenUnavailable, CodeTokenUnavailable, "No cloud credentials on this machine to sign in with. The detail says what to set."},
	{domain.ErrProxyFailed, CodeProxyFailed, "Could not reach the far side through the proxy. Check the proxy's type, host and port."},
	{domain.ErrProxyCredentials, CodeProxyCredentials, "The proxy rejected the credentials. Check its user and password."},
	{domain.ErrUnknownBucket, CodeUnknownBucket, "That bucket does not exist on this service, or these credentials cannot see it."},
	{domain.ErrUnknownObject, CodeUnknownObject, "That object is no longer there. Refresh the folder."},
	{domain.ErrForbidden, CodeForbidden, "These credentials may not do that. The key is accepted; it is the permission that is missing."},
	{domain.ErrTaskNotFound, CodeTaskNotFound, "That scheduled job no longer exists. Refresh the list."},
	{domain.ErrTaskRunning, CodeTaskRunning, "That job is already running. Wait for it to finish."},
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	// An error that has already been through here is left exactly as it is. One bound method
	// calling another — which is what a scheduled job does, firing `RunBackup` — would otherwise
	// have the answer it was given re-wrapped as INTERNAL, and the reader would be shown "something
	// went wrong" in place of the server's own words.
	var already wireError
	if errors.As(err, &already) {
		return already
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

	// What an engine has no concept of is named, so the message says which thing rather than only
	// that something was refused.
	var unsupportedErr domain.UnsupportedError
	if errors.As(err, &unsupportedErr) {
		return wireError{
			Code:    CodeUnsupported,
			Message: "This database does not support " + unsupportedErr.What + ".",
		}
	}

	// What a read-only connection held back is named, so the message says which statement or which
	// change was not sent rather than only that something was refused. Nothing reached the server:
	// that is what the wording has to convey, since there is nothing here to retry.
	var readOnlyErr domain.ReadOnlyError
	if errors.As(err, &readOnlyErr) {
		return wireError{
			Code:    CodeReadOnly,
			Message: "This connection is read-only. " + readOnlyErr.What + " was not sent to the server.",
			Detail:  readOnlyErr.What,
		}
	}

	// A program that is not on this machine is named, because the reader's next move is to install
	// that one — or to point the app at where it already is.
	var missingTool domain.ToolNotFoundError
	if errors.As(err, &missingTool) {
		return wireError{
			Code:    CodeToolNotFound,
			Message: missingTool.Tool + " is not on this machine. Install it, or say where it is.",
			Field:   missingTool.Tool,
		}
	}

	// A program that ran and refused answers in its own last words, for the same reason a rejected
	// statement does: mongorestore knows why it stopped and this app does not.
	var failedTool domain.ToolFailedError
	if errors.As(err, &failedTool) {
		// A program that ended badly without saying anything is rare and is still not nothing: its
		// own sentence stands in, so the reader is never shown an empty failure.
		message := failedTool.Message
		if message == "" {
			message = failedTool.Error()
		}
		return wireError{
			Code:    CodeToolFailed,
			Message: message,
			Field:   failedTool.Tool,
			Detail:  failedTool.Tool + " exited " + strconv.Itoa(failedTool.Status),
		}
	}

	// A service's own wording is the only useful message for a request it refused, exactly as an
	// engine's is about a statement it rejected: the service knows why and this app does not.
	var serviceErr domain.ServiceError
	if errors.As(err, &serviceErr) {
		return wireError{
			Code:    CodeServiceRefused,
			Message: serviceErr.Message,
			Detail:  serviceErr.Code,
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
