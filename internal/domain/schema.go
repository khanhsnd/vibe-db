package domain

import (
	"errors"
	"strings"
)

// ErrInvalidName is raised for a schema change that named nothing. Every identifier is quoted
// on its way into a statement, so having one at all is the only rule left to enforce; what a
// server will accept beyond that is the server's own business.
var ErrInvalidName = errors.New("name is required")

// NameError says which part of a schema change came through blank.
type NameError struct {
	What string
}

func (e NameError) Error() string {
	return e.What + " name is required"
}

func (e NameError) Unwrap() error {
	return ErrInvalidName
}

// RequireName refuses a blank identifier: `RequireName("table", name)`.
func RequireName(what, name string) error {
	if strings.TrimSpace(name) == "" {
		return NameError{What: what}
	}
	return nil
}

// RequireNames refuses the first blank of several, so a caller can check a whole request in
// one line: `RequireNames("table", table, "column", column)`.
func RequireNames(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if err := RequireName(pairs[i], pairs[i+1]); err != nil {
			return err
		}
	}
	return nil
}
