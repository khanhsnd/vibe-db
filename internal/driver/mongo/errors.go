package mongo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	mgo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/neterr"
)

// The server's own numbers for the failures a reader answers differently from a rejected command.
// MongoDB numbers every error, which puts this adapter's error map in the shape MySQL's is in
// rather than PostgreSQL's — see docs/errors.md.
const (
	errUserNotFound       = 11
	errAuthenticationFail = 18
	errNamespaceNotFound  = 26
	// errAtlasError is what a hosted cluster reports a refused *connection* as, credentials
	// included, because it never reaches the wire protocol's own authentication.
	errAtlasError = 8000
)

var codeToDomain = map[int]error{
	errUserNotFound:       domain.ErrBadCredentials,
	errAuthenticationFail: domain.ErrBadCredentials,
	errAtlasError:         domain.ErrBadCredentials,
	errNamespaceNotFound:  domain.ErrUnknownTable,
}

// authWordings are the texts a handshake failure arrives as when it arrives as no code at all: the
// driver wraps one in a connection error whose type carries nothing to switch on. This is the
// *adapter* reading a message, which is the same concession `sshtunnel` makes and for the same
// reason — see docs/errors.md. The UI switches on the code this produces, never on the text.
var authWordings = []string{"AuthenticationFailed", "auth error", "authentication failed"}

// translateError maps the server's own rejections onto domain sentinels. Anything that is a
// rejection of one command rather than of the connection stays a StatementError, so the server's
// wording — the only useful text for a command nobody but its author understands — reaches the
// reader verbatim.
func translateError(err error) error {
	if err == nil {
		return nil
	}

	// A cancelled read is cancelled whatever else the driver says about it, and a deadline is a
	// deadline. Both are answered before any server code is read, because the driver reports both
	// as failures of its own that may also carry one.
	if mapped := neterr.Map(err); mapped != nil {
		return mapped
	}
	if mgo.IsTimeout(err) {
		return domain.ErrConnectionTimeout
	}

	var serverErr mgo.ServerError
	if errors.As(err, &serverErr) {
		for code, sentinel := range codeToDomain {
			if serverErr.HasErrorCode(code) {
				return fmt.Errorf("%s: %w", err.Error(), sentinel)
			}
		}
		return rejected(err, serverErr)
	}

	message := err.Error()
	for _, wording := range authWordings {
		if strings.Contains(message, wording) {
			return fmt.Errorf("%s: %w", message, domain.ErrBadCredentials)
		}
	}

	// Every remaining way a connection fails reads as a server nobody could reach: a host that is
	// not there, a replica set with no primary and a name that does not resolve are all reported
	// as a server the driver could not select.
	if mgo.IsNetworkError(err) || strings.Contains(message, "server selection error") {
		return fmt.Errorf("%s: %w", message, domain.ErrHostUnreachable)
	}
	if errors.Is(err, mgo.ErrClientDisconnected) {
		return fmt.Errorf("%s: %w", message, domain.ErrSessionNotFound)
	}

	return err
}

// rejected is one command's refusal: the server's wording, and the *name* it gave the failure as the
// code — `NamespaceExists` says more to a reader than 48 does, which is why it is preferred to the
// number wherever the server sent one.
func rejected(err error, serverErr mgo.ServerError) error {
	var commandErr mgo.CommandError
	if errors.As(err, &commandErr) {
		code := commandErr.Name
		if code == "" {
			code = strconv.FormatInt(int64(commandErr.Code), 10)
		}
		return domain.StatementError{Code: code, Message: commandErr.Message}
	}

	code := ""
	if codes := serverErr.ErrorCodes(); len(codes) > 0 {
		code = strconv.Itoa(codes[0])
	}
	return domain.StatementError{Code: code, Message: err.Error()}
}
