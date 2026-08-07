package sqlserver

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
	mssql "github.com/microsoft/go-mssqldb"
)

// SQL Server identifies a failure by a number of its own, the way MySQL does.
const (
	errUnknownObject   = 208
	errUnknownDatabase = 911
	errLoginFailed     = 18456
	errNoDatabaseRight = 4060
	errUnknownSchema   = 2759
	errCancelled       = 3617
)

var numberToDomain = map[int32]error{
	errUnknownObject: domain.ErrUnknownTable,
	// 4060 is the login succeeding and the *database* being refused, which reads to a person as
	// the database not being there — and is answered in the same box.
	errUnknownDatabase: domain.ErrUnknownDatabase,
	errNoDatabaseRight: domain.ErrUnknownDatabase,
	errLoginFailed:     domain.ErrBadCredentials,
	errUnknownSchema:   domain.ErrUnknownSchema,
	errCancelled:       domain.ErrQueryCancelled,
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var mssqlErr mssql.Error
	if errors.As(err, &mssqlErr) {
		// The sentinel decides the code the UI switches on; the server's own wording names the
		// object it could not find, so it stays in the chain and reaches the user as the detail.
		if mapped, ok := numberToDomain[mssqlErr.Number]; ok {
			return fmt.Errorf("%s: %w", mssqlErr.Message, mapped)
		}
		return domain.StatementError{
			Code:    strconv.FormatInt(int64(mssqlErr.Number), 10),
			Message: mssqlErr.Message,
		}
	}

	if mapped := sqlkit.TransportError(err); mapped != nil {
		return mapped
	}

	return err
}
