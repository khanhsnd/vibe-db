package domain

import "errors"

var (
	ErrInvalidProfile    = errors.New("connection profile is invalid")
	ErrProfileNotFound   = errors.New("connection profile not found")
	ErrSessionNotFound   = errors.New("session not found")
	ErrBadCredentials    = errors.New("access denied for user")
	ErrUnknownDatabase   = errors.New("database does not exist")
	ErrHostUnreachable   = errors.New("cannot reach database server")
	ErrConnectionTimeout = errors.New("connection timed out")
	ErrTLSUnsupported    = errors.New("server does not support TLS")
	ErrTLSNotVerified    = errors.New("server TLS certificate could not be verified")
	ErrPasswordNotFound  = errors.New("no stored password for profile")
)

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
