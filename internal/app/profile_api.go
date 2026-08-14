package app

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
)

// What a server already ran slowly, and what it is recording, as four bound methods. Nothing here
// knows where an engine keeps its slow statements or how it spells a switch: the reads and the
// statements are the driver's, the hazard is a `domain` rule, and this layer's whole job is to find
// the session, refuse a change that arrived as something this app does not do, and wrap what comes
// back.
//
// See docs/features/profiler.md.

// LoadSlowQueries is what the server recorded, in the columns its own catalogue keeps.
//
// It takes the ordinary read deadline, exactly as a process list does, and for the same reason: this
// is one catalogue read on every engine, and a server too busy to answer one inside the deadline is a
// server the reader wants told about rather than waited on.
//
// The namespace is carried because two engines keep the recording per **database** — a MongoDB
// profile is a collection of one, and a PostgreSQL extension is installed into one — so a pane read
// against the wrong database would be a pane that says a server records nothing when it does.
func (a *ConnectionAPI) LoadSlowQueries(
	sessionID string,
	ns Namespace,
	limit int,
) (SlowQueries, error) {
	profiler, _, err := a.sessions.Profiler(sessionID)
	if err != nil {
		return SlowQueries{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	list, err := profiler.SlowQueries(ctx, ns.toDriver(), limit)
	if err != nil {
		return SlowQueries{}, wrapError(err)
	}
	return toSlowQueriesDTO(list), nil
}

// LoadProfileConfig is how the recording is currently set, read from the server. It is a read of its
// own rather than a field of the list above because the two answer different questions, and the one
// that matters most is asked when the list is **empty**: nothing recorded and nothing recording are
// not the same answer, and only this one can tell them apart.
func (a *ConnectionAPI) LoadProfileConfig(sessionID string, ns Namespace) (ProfileConfig, error) {
	profiler, _, err := a.sessions.Profiler(sessionID)
	if err != nil {
		return ProfileConfig{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	config, err := profiler.ProfileConfig(ctx, ns.toDriver())
	if err != nil {
		return ProfileConfig{}, wrapError(err)
	}
	return toProfileConfigDTO(config), nil
}

// PreviewProfileChanges is exactly what ApplyProfileChanges will run, so the confirmation shows the
// statements rather than a description of them — the arrangement PreviewProcessKills and
// PreviewAccountChanges have.
//
// It answers with the hazard beside them, and whether the reader has to confirm: that is
// `domain.ProfileHazard` paired with the connection's environment, decided here and never re-derived
// in a component.
func (a *ConnectionAPI) PreviewProfileChanges(
	sessionID string,
	changes []ProfileChangeInput,
) (ProfilePreview, error) {
	profiler, environment, err := a.sessions.Profiler(sessionID)
	if err != nil {
		return ProfilePreview{}, wrapError(err)
	}

	planned, err := toDriverProfileChanges(changes)
	if err != nil {
		return ProfilePreview{}, wrapError(err)
	}

	statements := profiler.ProfileStatements(planned)
	if statements == nil {
		statements = []string{}
	}

	hazard := profileHazard(planned)
	return ProfilePreview{
		Statements: statements,
		Hazard:     string(hazard),
		Confirm:    environment.NeedsConfirmation(hazard),
	}, nil
}

// ApplyProfileChanges runs what PreviewProfileChanges showed, in that order, stopping at the first
// failure. A list with nothing in it is refused rather than being a run that does nothing quietly.
func (a *ConnectionAPI) ApplyProfileChanges(
	sessionID string,
	ns Namespace,
	changes []ProfileChangeInput,
) error {
	profiler, _, err := a.sessions.Profiler(sessionID)
	if err != nil {
		return wrapError(err)
	}
	if len(changes) == 0 {
		return wrapError(domain.ErrEmptyStatement)
	}

	planned, err := toDriverProfileChanges(changes)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(profiler.ApplyProfileChanges(ctx, ns.toDriver(), planned))
}
