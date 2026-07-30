package mysql

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

const (
	errAccessDenied         = 1045
	errUnknownDatabase      = 1049
	errUnknownTable         = 1146
	errAccessDeniedNoPasswd = 1698
)

var errnoToDomain = map[uint16]error{
	errAccessDenied:         domain.ErrBadCredentials,
	errAccessDeniedNoPasswd: domain.ErrBadCredentials,
	errUnknownDatabase:      domain.ErrUnknownDatabase,
	errUnknownTable:         domain.ErrUnknownTable,
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		// The sentinel decides the code the UI switches on; the server's own wording names
		// the object it could not find, so it stays in the chain and reaches the user as
		// the error's detail.
		if mapped, ok := errnoToDomain[mysqlErr.Number]; ok {
			return fmt.Errorf("%s: %w", mysqlErr.Message, mapped)
		}
		return domain.StatementError{
			Code:    strconv.FormatUint(uint64(mysqlErr.Number), 10),
			Message: mysqlErr.Message,
		}
	}

	if errors.Is(err, mysql.ErrNoTLS) {
		return domain.ErrTLSUnsupported
	}

	if mapped := sqlkit.TransportError(err); mapped != nil {
		return mapped
	}

	return err
}
