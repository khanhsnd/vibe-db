package redis

import (
	"errors"
	"fmt"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/neterr"
)

// prefixes are the words Redis begins an error with, against the sentinel each of them means. It is
// the shape MySQL's errno map and MongoDB's code map are in, and it is very nearly the *whole* of
// this engine's error contract: a Redis error is a line of text whose first word is its class, which
// is why `domain.StatementError.Code` — a string because MySQL numbers its errors and PostgreSQL
// letters them — carries that word unchanged for everything not listed here.
var prefixes = map[string]error{
	"NOAUTH":    domain.ErrBadCredentials,
	"WRONGPASS": domain.ErrBadCredentials,
	"NOPERM":    domain.ErrBadCredentials,
}

// authWordings are the rejections whose first word does *not* say what they are. A server with
// `requirepass` set answers `ERR Client sent AUTH, but no password is set` or `ERR invalid password`
// depending on its age, so the wording is read. It is the same concession the MongoDB adapter makes
// for a handshake failure and `sshtunnel` makes for a bastion — an *adapter* reading a library's
// text, never the UI — see docs/errors.md.
var authWordings = []string{
	"invalid password",
	"invalid username-password",
	"client sent auth",
	"authentication required",
	"without any password configured",
}

// rangeWordings is a `SELECT` for a database the server was not started with, which is the only way
// this engine says a database does not exist: the set is fixed at start-up, so index 17 of a
// sixteen-database server is not missing so much as impossible.
var rangeWordings = []string{"db index is out of range", "invalid db index"}

// translateError maps go-redis's failures onto domain sentinels. Transport comes first and is
// shared — a cancelled context and an unreachable host read the same whatever the engine — and what
// is left is a line the server sent, which is this engine's whole error language.
//
// `redis.Nil` is deliberately not one of them. It is the *absence* of a value rather than a failure:
// a GET of a key that is not there, or a value read while it expired, and every caller here answers
// it as the fact it is.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, goredis.Nil) {
		return err
	}
	if mapped := neterr.Map(err); mapped != nil {
		return mapped
	}

	message := err.Error()
	code, rest := errorParts(message)
	lower := strings.ToLower(message)

	switch {
	case prefixes[code] != nil:
		return fmt.Errorf("%s: %w", message, prefixes[code])
	case contains(lower, authWordings):
		return fmt.Errorf("%s: %w", message, domain.ErrBadCredentials)
	case contains(lower, rangeWordings):
		return fmt.Errorf("%s: %w", message, domain.ErrUnknownDatabase)
	}

	// Everything else is the server's own rejection of one command, and its wording is the only
	// useful thing to show: `WRONGTYPE Operation against a key holding the wrong kind of value` says
	// more than any classification of it would.
	return domain.StatementError{Code: code, Message: rest}
}

// errorParts splits a Redis error into its class and its wording. The class is the first word when
// it is written the way the server writes one — upper case and nothing else — and `ERR` when the
// failure came from somewhere with no class at all, which is what a client-side one looks like.
func errorParts(message string) (code, rest string) {
	first, tail, found := strings.Cut(message, " ")
	if !found || first == "" || first != strings.ToUpper(first) {
		return "ERR", message
	}
	return first, tail
}

func contains(message string, wordings []string) bool {
	for _, wording := range wordings {
		if strings.Contains(message, wording) {
			return true
		}
	}
	return false
}
