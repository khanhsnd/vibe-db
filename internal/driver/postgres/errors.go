package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// PostgreSQL identifies a failure by its SQLSTATE rather than a number of its own.
const (
	sqlStateInvalidAuthorization = "28000"
	sqlStateInvalidPassword      = "28P01"
	sqlStateInvalidCatalogName   = "3D000"
	sqlStateInvalidSchemaName    = "3F000"
	sqlStateUndefinedTable       = "42P01"
	sqlStateQueryCanceled        = "57014"
)

var sqlStateToDomain = map[string]error{
	sqlStateInvalidAuthorization: domain.ErrBadCredentials,
	sqlStateInvalidPassword:      domain.ErrBadCredentials,
	sqlStateInvalidCatalogName:   domain.ErrUnknownDatabase,
	sqlStateInvalidSchemaName:    domain.ErrUnknownSchema,
	sqlStateUndefinedTable:       domain.ErrUnknownTable,
	sqlStateQueryCanceled:        domain.ErrQueryCancelled,
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// The sentinel decides the code the UI switches on, but the server's own wording is
		// the only thing that says which name it went looking for — PostgreSQL folds an
		// unquoted identifier to lower case, and that is usually the surprise — so it is
		// kept in the chain and reaches the user as the error's detail.
		if mapped, ok := sqlStateToDomain[pgErr.Code]; ok {
			return fmt.Errorf("%s: %w", pgErr.Message, mapped)
		}
		return domain.StatementError{Code: pgErr.Code, Message: pgErr.Message}
	}

	if isTLSRefused(err) {
		return domain.ErrTLSUnsupported
	}

	if mapped := sqlkit.TransportError(err); mapped != nil {
		return mapped
	}

	return err
}

// isTLSRefused reads the one refusal pgx reports as prose rather than as a typed error or a
// SQLSTATE: a server built without TLS answering a mode that demanded it. The match is on
// pgx's own wording and stays inside this adapter, which is the only place that knows it.
func isTLSRefused(err error) bool {
	return strings.Contains(err.Error(), "server refused TLS connection")
}
