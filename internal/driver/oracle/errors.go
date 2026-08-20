package oracle

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/neterr"
	"github.com/sijms/go-ora/v2/network"
)

// Oracle identifies a failure by an `ORA-` number, which is what the code the UI switches on is
// read from.
const (
	errBadCredentials = 1017
	errNoSuchTable    = 942
	errNoSuchUser     = 1918
	errNoSuchService  = 12514
	errAccountLocked  = 28000
	errCancelled      = 1013
)

var numberToDomain = map[int]error{
	errBadCredentials: domain.ErrBadCredentials,
	errAccountLocked:  domain.ErrBadCredentials,
	errNoSuchTable:    domain.ErrUnknownTable,
	// A schema is what stands where a database stands here, so a schema that is not there is
	// answered in the box that names one — see the note at the top of driver.go.
	errNoSuchUser: domain.ErrUnknownDatabase,
	// The listener refusing the service is the same mistake one step earlier: the name in the
	// Database box is not a service this listener knows.
	errNoSuchService: domain.ErrUnknownDatabase,
	errCancelled:     domain.ErrQueryCancelled,
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var oracleErr *network.OracleError
	if errors.As(err, &oracleErr) {
		// The sentinel decides the code the UI switches on; the server's own wording names the
		// object it could not find — Oracle folds an unquoted name to upper case, and that is
		// usually the surprise — so it stays in the chain and reaches the user as the detail.
		if mapped, ok := numberToDomain[oracleErr.ErrCode]; ok {
			return fmt.Errorf("%s: %w", oracleErr.Error(), mapped)
		}
		return domain.StatementError{
			Code:    "ORA-" + strconv.Itoa(oracleErr.ErrCode),
			Message: oracleErr.Error(),
		}
	}

	if mapped := neterr.Map(err); mapped != nil {
		return mapped
	}

	return err
}
