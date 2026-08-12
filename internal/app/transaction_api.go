package app

// The manual transaction, as four bound methods. There is no DTO: what a transaction is addressed
// by is the session and the namespace the reader is looking at, and what it answers with is whether
// one is now open — which is what the button draws itself from, so every one of them returns it and
// the frontend never has to ask a second time.
//
// It is `internal/session`'s to hold and the driver's to run; nothing here knows how an engine
// spells BEGIN, and an engine that has no such notion answers `domain.Unsupported` from its own
// adapter. See docs/features/transaction.md.

// BeginTransaction opens one. It carries the namespace so that an unqualified name inside the
// transaction resolves where the reader is looking, exactly as a definition does.
func (a *ConnectionAPI) BeginTransaction(sessionID string, ns Namespace) (bool, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	if err := a.sessions.Begin(ctx, sessionID, ns.toDriver()); err != nil {
		return a.sessions.InTransaction(sessionID), wrapError(err)
	}
	return a.sessions.InTransaction(sessionID), nil
}

// CommitTransaction makes what was done inside it true for everybody.
func (a *ConnectionAPI) CommitTransaction(sessionID string) (bool, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	if err := a.sessions.Commit(ctx, sessionID); err != nil {
		return a.sessions.InTransaction(sessionID), wrapError(err)
	}
	return a.sessions.InTransaction(sessionID), nil
}

// RollbackTransaction throws it away. The transaction is over either way — see driver.Transaction —
// so the answer beside a failure is the truth about the session and not a guess.
func (a *ConnectionAPI) RollbackTransaction(sessionID string) (bool, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	if err := a.sessions.Rollback(ctx, sessionID); err != nil {
		return a.sessions.InTransaction(sessionID), wrapError(err)
	}
	return a.sessions.InTransaction(sessionID), nil
}

// InTransaction is what a reader returning to a tab is drawn from, and it asks no server anything.
func (a *ConnectionAPI) InTransaction(sessionID string) bool {
	return a.sessions.InTransaction(sessionID)
}
