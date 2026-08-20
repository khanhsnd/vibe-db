package sqlite

import (
	"errors"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/neterr"
	"modernc.org/sqlite"
)

// SQLite answers with a small set of numbered classes, and the primary code is the low byte of
// whatever it reports — an extended code carries a reason in the bits above it.
const (
	codeError    = 1
	codeReadOnly = 8
	codeCantOpen = 14
	codeNotADB   = 26
)

// missingTable is the one failure SQLite reports as prose rather than as a class of its own: every
// name that is not there comes back as the generic error. It is read here, and only here, for the
// reason the SSH package reads a message — there is no code to switch on, and the alternative is
// telling a reader that a mistyped table name is an internal failure.
const missingTable = "no such table"

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		message := sqliteErr.Error()

		switch primary(sqliteErr.Code()) {
		case codeCantOpen, codeNotADB:
			// Neither is a rejection by a server, there being none: the file is not there, is not
			// this account's to open, or is not a database. The reader answers all three in the
			// picker, which is what ErrFileUnreadable is for.
			return domain.FileError{Err: err}
		case codeError:
			if strings.Contains(message, missingTable) {
				return domain.ErrUnknownTable
			}
		case codeReadOnly:
			return domain.StatementError{
				Code:    strconv.Itoa(codeReadOnly),
				Message: "the database file is read-only",
			}
		}

		return domain.StatementError{
			Code:    strconv.Itoa(sqliteErr.Code()),
			Message: message,
		}
	}

	// A cancelled read still comes through the context, which is the one thing about a file that
	// reads exactly as it does about a server.
	if mapped := neterr.Map(err); mapped != nil {
		return mapped
	}

	return err
}

// primary is the class of a result code with the reason above it masked off, so an extended code
// is read as the failure it belongs to rather than as one nothing here knows.
func primary(code int) int {
	return code & 0xff
}
