package app

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
)

// User and privilege management, as five bound methods. Nothing here knows how an engine spells a
// `GRANT`: the statements are the driver's, the hazard is a `domain` rule, and this layer's whole
// job is to find the session, refuse a change that arrived as something this app does not make, and
// wrap what comes back.
//
// See docs/features/accounts.md.

// ListAccounts is who this server knows, in the columns its own catalogue keeps.
func (a *ConnectionAPI) ListAccounts(sessionID string) (AccountList, error) {
	security, _, err := a.sessions.Security(sessionID)
	if err != nil {
		return AccountList{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	list, err := security.Accounts(ctx)
	if err != nil {
		return AccountList{}, wrapError(err)
	}
	return toAccountListDTO(list), nil
}

// LoadAccount reads one whole: its attributes, what it has been granted, the roles it holds, and
// the server's own words about it.
func (a *ConnectionAPI) LoadAccount(sessionID string, ref AccountRef) (AccountDetail, error) {
	security, _, err := a.sessions.Security(sessionID)
	if err != nil {
		return AccountDetail{}, wrapError(err)
	}
	if err := domain.RequireName("user", ref.Name); err != nil {
		return AccountDetail{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	detail, err := security.Account(ctx, ref.toDriver())
	if err != nil {
		return AccountDetail{}, wrapError(err)
	}
	return toAccountDetailDTO(detail), nil
}

// LoadAccountOptions reads the lists the editor draws its boxes and its privilege columns from —
// which attributes this engine's account carries, what may be granted at each level, and which
// roles exist.
func (a *ConnectionAPI) LoadAccountOptions(sessionID string) (AccountOptions, error) {
	security, _, err := a.sessions.Security(sessionID)
	if err != nil {
		return AccountOptions{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	options, err := security.AccountOptions(ctx)
	if err != nil {
		return AccountOptions{}, wrapError(err)
	}
	return toAccountOptionsDTO(options), nil
}

// PreviewAccountChanges is exactly what ApplyAccountChanges will run, so the editor's Changes pane
// shows the statements rather than a guess at them — the arrangement PreviewTableChanges and
// PreviewSource have.
//
// **The password is not in it.** This is what goes on the screen, into the History Log and into a
// saved history, and the driver spells a placeholder where the literal will go. It is the one thing
// about a preview here that is not the statement character for character, and it is deliberate.
//
// It answers with the hazard beside the statements, and whether the reader has to confirm: that is
// `domain.GrantHazard` paired with the connection's environment, decided here and never re-derived
// in a component.
func (a *ConnectionAPI) PreviewAccountChanges(
	sessionID string,
	changes []AccountChangeInput,
) (AccountPreview, error) {
	security, environment, err := a.sessions.Security(sessionID)
	if err != nil {
		return AccountPreview{}, wrapError(err)
	}

	planned, err := toDriverAccountChanges(changes)
	if err != nil {
		return AccountPreview{}, wrapError(err)
	}

	statements := security.AccountStatements(planned)
	if statements == nil {
		statements = []string{}
	}

	hazard := accountHazard(planned)
	return AccountPreview{
		Statements: statements,
		Hazard:     string(hazard),
		Confirm:    environment.NeedsConfirmation(hazard),
	}, nil
}

// ApplyAccountChanges runs what PreviewAccountChanges showed, in that order, stopping at the first
// failure. A list with nothing in it is refused rather than being a run that does nothing quietly.
//
// It takes the same deadline a schema change does: a grant across a large catalogue is not always
// instant, and neither is dropping an account that owns things.
func (a *ConnectionAPI) ApplyAccountChanges(
	sessionID string,
	changes []AccountChangeInput,
) error {
	security, _, err := a.sessions.Security(sessionID)
	if err != nil {
		return wrapError(err)
	}
	if len(changes) == 0 {
		return wrapError(domain.ErrEmptyStatement)
	}

	planned, err := toDriverAccountChanges(changes)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(schemaTimeout)
	defer cancel()

	return wrapError(security.ApplyAccountChanges(ctx, planned))
}
