package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// What a server is doing and what it was started with, as six bound methods. Nothing here knows how
// an engine spells a kill or where it keeps its counters: the statements are the driver's, the hazard
// is a `domain` rule, and this layer's whole job is to find the session, refuse an action that arrived
// as something this app does not do, and wrap what comes back.
//
// See docs/features/monitor.md.

// ListProcesses is what the server is doing right now, in the columns its own catalogue keeps.
//
// It takes the ordinary read deadline. A process list is one catalogue read on every engine, and a
// server too busy to answer one inside it is a server the reader wants told about rather than waited
// on — the pane is re-read on a clock.
func (a *ConnectionAPI) ListProcesses(sessionID string) (ProcessList, error) {
	monitor, _, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return ProcessList{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	list, err := monitor.Processes(ctx)
	if err != nil {
		return ProcessList{}, wrapError(err)
	}
	return toProcessListDTO(list), nil
}

// PreviewProcessKills is exactly what KillProcesses will run, so the confirmation shows the statements
// rather than a count of rows — the arrangement PreviewAccountChanges and PreviewTableChanges have.
//
// It answers with the hazard beside them, and whether the reader has to confirm: that is
// `domain.ProcessHazard` paired with the connection's environment, decided here and never re-derived
// in a component.
func (a *ConnectionAPI) PreviewProcessKills(
	sessionID string,
	kills []ProcessKillInput,
) (ProcessPreview, error) {
	monitor, environment, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return ProcessPreview{}, wrapError(err)
	}

	planned, err := toDriverProcessKills(kills)
	if err != nil {
		return ProcessPreview{}, wrapError(err)
	}

	statements := monitor.ProcessStatements(planned)
	if statements == nil {
		statements = []string{}
	}

	hazard := processHazard(planned)
	return ProcessPreview{
		Statements: statements,
		Hazard:     string(hazard),
		Confirm:    environment.NeedsConfirmation(hazard),
	}, nil
}

// KillProcesses runs what PreviewProcessKills showed, in that order, stopping at the first failure. A
// list with nothing in it is refused rather than being a run that does nothing quietly.
func (a *ConnectionAPI) KillProcesses(sessionID string, kills []ProcessKillInput) error {
	monitor, _, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return wrapError(err)
	}
	if len(kills) == 0 {
		return wrapError(domain.ErrEmptyStatement)
	}

	planned, err := toDriverProcessKills(kills)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(monitor.KillProcesses(ctx, planned))
}

// LoadServerStatus is what the server has counted since it started.
func (a *ConnectionAPI) LoadServerStatus(sessionID string) (ServerFacts, error) {
	monitor, _, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return ServerFacts{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	facts, err := monitor.ServerStatus(ctx)
	if err != nil {
		return ServerFacts{}, wrapError(err)
	}
	return toServerFactsDTO(facts), nil
}

// LoadServerVariables is what it was started with. It is a read: changing one is a different statement
// on every engine — and on two of them a file rewritten or a restart — so a variable is changed in a
// query tab, where the reader can see which server they are on.
func (a *ConnectionAPI) LoadServerVariables(sessionID string) (ServerFacts, error) {
	monitor, _, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return ServerFacts{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	facts, err := monitor.ServerVariables(ctx)
	if err != nil {
		return ServerFacts{}, wrapError(err)
	}
	return toServerFactsDTO(facts), nil
}

// LoadServerMetrics is the handful of numbers behind those lists that are worth watching move, in the
// four panels a chart is drawn per — see docs/features/metrics.md.
//
// It is one bound method rather than four because it is one read: a session count from one moment
// drawn beside a rate from another would be two servers on one screen. The moment it came back is
// stamped on the answer, that being what a counter is divided by to become a rate.
func (a *ConnectionAPI) LoadServerMetrics(sessionID string) (ServerMetrics, error) {
	monitor, _, err := a.sessions.Monitor(sessionID)
	if err != nil {
		return ServerMetrics{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	metrics, err := monitor.ServerMetrics(ctx)
	if err != nil {
		return ServerMetrics{}, wrapError(err)
	}
	return toServerMetricsDTO(metrics, time.Now()), nil
}
