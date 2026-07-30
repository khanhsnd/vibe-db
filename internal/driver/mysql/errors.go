package mysql

import (
	"errors"
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
		if mapped, ok := errnoToDomain[mysqlErr.Number]; ok {
			return mapped
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
