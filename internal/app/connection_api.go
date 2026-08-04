package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/profile"
	"github.com/khanhsnd/vibe-db/internal/secret"
	"github.com/khanhsnd/vibe-db/internal/session"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	connectTimeout = 8 * time.Second
	// tunnelTimeout is what a connection through a bastion is given *on top of* the above: the
	// dial and the handshake are a hop of their own before the database has been reached at all.
	tunnelTimeout = 12 * time.Second
	queryTimeout  = 15 * time.Second

	// noDeadline leaves a read bounded by StopTableRows rather than by a clock — see rowsTimeout.
	noDeadline = 0
)

type ConnectionAPI struct {
	apiContext

	profiles profile.Repository
	secrets  secret.Store
	sessions *session.Manager
	runs     *runRegistry
}

func NewConnectionAPI(
	profiles profile.Repository,
	secrets secret.Store,
	sessions *session.Manager,
) *ConnectionAPI {
	return &ConnectionAPI{
		apiContext: newAPIContext(),
		profiles:   profiles,
		secrets:    secrets,
		sessions:   sessions,
		runs:       newRunRegistry(),
	}
}

func (a *ConnectionAPI) ListProfiles() ([]Profile, error) {
	stored, err := a.profiles.List()
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]Profile, 0, len(stored))
	for _, p := range stored {
		out = append(out, a.profileDTO(p))
	}
	return out, nil
}

func (a *ConnectionAPI) SaveProfile(input ProfileInput) (Profile, error) {
	target := input.toDomain()

	if target.ID == "" {
		id, err := newProfileID()
		if err != nil {
			return Profile{}, wrapError(err)
		}
		target.ID = id
	}

	if err := target.Validate(); err != nil {
		return Profile{}, wrapError(err)
	}

	if err := a.profiles.Save(target); err != nil {
		return Profile{}, wrapError(err)
	}

	if err := a.persistPassword(target, input.Password); err != nil {
		return Profile{}, wrapError(err)
	}
	if err := a.persistSSHSecret(target, input.SSHSecret); err != nil {
		return Profile{}, wrapError(err)
	}

	return a.profileDTO(target), nil
}

func (a *ConnectionAPI) DeleteProfile(id string) error {
	if err := a.profiles.Delete(id); err != nil {
		return wrapError(err)
	}
	if err := a.secrets.Delete(id); err != nil {
		return wrapError(err)
	}
	if err := a.secrets.Delete(sshSecretKey(id)); err != nil {
		return wrapError(err)
	}
	return nil
}

func (a *ConnectionAPI) TestConnection(input ProfileInput) (TestResult, error) {
	target := input.toDomain()
	if err := target.Validate(); err != nil {
		return TestResult{}, wrapError(err)
	}

	secrets, err := a.resolveSecrets(target, input.Password, input.SSHSecret)
	if err != nil {
		return TestResult{}, wrapError(err)
	}

	// A tunnel is dialled before the database is, so a profile that goes through one is given
	// room for both hops rather than being cut off part way through the first.
	ctx, cancel := a.withTimeout(openTimeout(target))
	defer cancel()

	started := time.Now()
	opened, err := a.sessions.Open(ctx, target, secrets)
	if err != nil {
		return TestResult{}, wrapError(err)
	}
	latency := time.Since(started)

	if err := a.sessions.Close(opened.ID); err != nil {
		return TestResult{}, wrapError(err)
	}

	return TestResult{
		ServerVersion: opened.ServerVersion,
		LatencyMs:     latency.Milliseconds(),
	}, nil
}

// Connect dials a stored profile. The password is the one the reader was prompted for, or empty
// to use whatever is stored — and a profile that authenticates by a cloud identity needs neither,
// since its password is minted at the moment of connecting.
func (a *ConnectionAPI) Connect(profileID, password string) (Session, error) {
	stored, err := a.profiles.Get(profileID)
	if err != nil {
		return Session{}, wrapError(err)
	}

	secrets, err := a.resolveSecrets(stored, password, "")
	if err != nil {
		return Session{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(openTimeout(stored))
	defer cancel()

	opened, err := a.sessions.Open(ctx, stored, secrets)
	if err != nil {
		return Session{}, wrapError(err)
	}

	return Session{
		ID:            opened.ID,
		ProfileID:     stored.ID,
		Target:        stored.Target(),
		ServerVersion: opened.ServerVersion,
	}, nil
}

func (a *ConnectionAPI) Disconnect(sessionID string) error {
	return wrapError(a.sessions.Close(sessionID))
}

func (a *ConnectionAPI) ListDatabases(sessionID string) ([]string, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	names, err := a.sessions.Databases(ctx, sessionID)
	if err != nil {
		return nil, wrapError(err)
	}
	return names, nil
}

// ListSchemas answers with nothing for an engine whose database is its schema, which is how
// the tree knows not to draw a level there.
func (a *ConnectionAPI) ListSchemas(sessionID, database string) ([]string, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	names, err := a.sessions.Schemas(ctx, sessionID, database)
	if err != nil {
		return nil, wrapError(err)
	}
	return names, nil
}

// LoadObjects answers with everything the tree draws a folder for under a namespace, in one
// call, because opening any one of those folders needs every folder's count.
func (a *ConnectionAPI) LoadObjects(sessionID string, ns Namespace) (Objects, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	objects, err := a.sessions.Objects(ctx, sessionID, ns.toDriver())
	if err != nil {
		return Objects{}, wrapError(err)
	}
	return toObjectsDTO(objects), nil
}

// LoadTableList answers with what the engine's catalogue keeps about the tables of a namespace,
// which is what the Tables folder opens. The columns are the engine's own, so this side neither
// names them nor formats a single value of them.
func (a *ConnectionAPI) LoadTableList(sessionID string, ns Namespace) (TableList, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	list, err := a.sessions.TableList(ctx, sessionID, ns.toDriver())
	if err != nil {
		return TableList{}, wrapError(err)
	}
	return toTableListDTO(list), nil
}

func (a *ConnectionAPI) LoadTableSchemas(sessionID string, ns Namespace) ([]TableSchema, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	tables, err := a.sessions.TableSchemas(ctx, sessionID, ns.toDriver())
	if err != nil {
		return nil, wrapError(err)
	}
	return toTableSchemaDTOs(tables), nil
}

// LoadTableRows reads one window onto a table's rows.
//
// A bounded page is answered whole. A page that asked for **every** row and minted a run id is
// streamed instead: its rows are emitted as EventRows batches and the answer carries the columns
// and the total alone, because a million rows cannot cross the bridge as one JavaScript string.
// Such a read is given no deadline either — how long every row of a table takes is not knowable,
// and a clock that cuts it off is the very failure All exists to avoid — so StopTableRows is what
// bounds it, and the batches are what make stopping a real choice rather than a guess.
func (a *ConnectionAPI) LoadTableRows(sessionID string, page TablePageInput) (TableRows, error) {
	target := page.toDriver()

	streamed := target.Limit <= 0 && page.RunID != ""
	if streamed {
		target.Stream = a.streamRows(page.RunID)
	}

	ctx, release := a.runs.begin(a.ctx, page.RunID, rowsTimeout(streamed))
	defer release()

	rows, err := a.sessions.TableRows(ctx, sessionID, target)
	if err != nil {
		return TableRows{}, wrapError(err)
	}
	return toTableRowsDTO(rows, target), nil
}

// LoadColumnValues lists the values one column actually holds, which is what the filter builder
// offers to pick from rather than making the reader type one. NULL is not among them: the
// `is NULL` operator is how a reader reaches it, and a list cannot say the difference.
func (a *ConnectionAPI) LoadColumnValues(sessionID string, in ColumnValuesInput) ([]string, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	values, err := a.sessions.ColumnValues(ctx, sessionID, driver.ColumnPick{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Column:    in.Column,
		Limit:     ColumnValueLimit,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return values, nil
}

// StopTableRows stops the read with this id. An unknown id is ignored: the read may have
// finished between the click and this call, the same contract QueryAPI.Cancel has.
func (a *ConnectionAPI) StopTableRows(runID string) {
	a.runs.cancel(runID)
}

// rowsTimeout is the ordinary read deadline for a bounded page and none at all for a streamed
// one. An unbounded read that did *not* ask to be streamed keeps the deadline, because nothing
// on the other side could stop it.
func rowsTimeout(streamed bool) time.Duration {
	if streamed {
		return noDeadline
	}
	return queryTimeout
}

// streamRows pushes a read to the frontend as it goes: its rows batch by batch, and its count on
// its own clock. Emitting is all either half does — both run on the goroutine doing the scan, so
// anything slower here is time the read spends waiting.
func (a *ConnectionAPI) streamRows(runID string) *driver.RowStream {
	return &driver.RowStream{
		Rows: func(batch [][]*string) {
			wailsruntime.EventsEmit(a.ctx, EventRows, RowsBatch{RunID: runID, Rows: batch})
		},
		Counted: func(fetched int) {
			wailsruntime.EventsEmit(a.ctx, EventRowsCounted, RowsCounted{
				RunID:   runID,
				Fetched: fetched,
			})
		},
	}
}

func (a *ConnectionAPI) UpdateCell(sessionID string, edit CellEditInput) error {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(a.sessions.UpdateCell(ctx, sessionID, edit.toDriver()))
}

func (a *ConnectionAPI) InsertRow(sessionID string, insert RowInsertInput) error {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(a.sessions.InsertRow(ctx, sessionID, insert.toDriver()))
}

func (a *ConnectionAPI) DeleteRow(sessionID string, removal RowDeleteInput) error {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(a.sessions.DeleteRow(ctx, sessionID, removal.toDriver()))
}

// openTimeout is how long a connection is given to come up. A tunnelled one has two hops to make
// — the bastion, then the database through it — and cutting the second one off because the first
// took its time would report a working server as unreachable.
func openTimeout(target domain.ConnectionProfile) time.Duration {
	if target.SSH.Enabled {
		return connectTimeout + tunnelTimeout
	}
	return connectTimeout
}

// sshSecretKey is where a bastion's own secret lives in the credential store. The store is keyed
// by string and a profile has two secrets rather than one, so the connection's password keeps the
// profile's id and the bastion's takes a suffix — one profile, two entries, both deleted with it.
func sshSecretKey(profileID string) string {
	return profileID + ":ssh"
}

// resolveSecrets is what a connection is opened with: whatever was typed just now, falling back to
// what is stored. A blank one is not an error — a server with no password is ordinary, and so is a
// profile whose password was never saved, which is what the prompt is for.
func (a *ConnectionAPI) resolveSecrets(
	target domain.ConnectionProfile,
	password, sshSecret string,
) (session.Secrets, error) {
	// A minted password is signed for at the moment of connecting, so nothing typed or stored is
	// read for one: passing a stale token along would fail in a way that looked like a bad
	// password rather than like an expired identity.
	if target.Auth.Minted() {
		password = ""
	} else if password == "" {
		stored, err := a.storedSecret(target, target.ID)
		if err != nil {
			return session.Secrets{}, err
		}
		password = stored
	}

	if sshSecret == "" && target.SSH.Enabled {
		stored, err := a.storedSecret(target, sshSecretKey(target.ID))
		if err != nil {
			return session.Secrets{}, err
		}
		sshSecret = stored
	}

	return session.Secrets{Password: password, SSH: sshSecret}, nil
}

// storedSecret reads one entry of the credential store, answering blank where the profile does not
// keep its secrets or nothing has been kept yet.
func (a *ConnectionAPI) storedSecret(target domain.ConnectionProfile, key string) (string, error) {
	if !target.SavePassword || target.ID == "" {
		return "", nil
	}

	stored, err := a.secrets.Get(key)
	if errors.Is(err, domain.ErrPasswordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return stored, nil
}

func (a *ConnectionAPI) persistPassword(target domain.ConnectionProfile, password string) error {
	return a.persistSecret(target, target.ID, password)
}

func (a *ConnectionAPI) persistSSHSecret(target domain.ConnectionProfile, secret string) error {
	// A tunnel that has been turned off keeps no pass phrase, the same rule the password follows
	// when a profile stops saving one.
	if !target.SSH.Enabled {
		return a.secrets.Delete(sshSecretKey(target.ID))
	}
	return a.persistSecret(target, sshSecretKey(target.ID), secret)
}

// persistSecret writes one entry, and forgets it where the profile has stopped keeping secrets.
// A blank secret leaves what is stored alone, which is what "leave it blank to keep the saved
// password" means in the dialog.
func (a *ConnectionAPI) persistSecret(
	target domain.ConnectionProfile,
	key, secret string,
) error {
	if !target.SavePassword {
		return a.secrets.Delete(key)
	}
	if secret == "" {
		return nil
	}
	return a.secrets.Set(key, secret)
}

// profileDTO answers the two questions the dialog asks about secrets it must never be shown: is
// there a password stored, and is there a pass phrase for the bastion.
func (a *ConnectionAPI) profileDTO(target domain.ConnectionProfile) Profile {
	return toProfileDTO(target, a.hasSecret(target, target.ID),
		target.SSH.Enabled && a.hasSecret(target, sshSecretKey(target.ID)))
}

func (a *ConnectionAPI) hasSecret(target domain.ConnectionProfile, key string) bool {
	if !target.SavePassword {
		return false
	}
	_, err := a.secrets.Get(key)
	return err == nil
}

func newProfileID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
