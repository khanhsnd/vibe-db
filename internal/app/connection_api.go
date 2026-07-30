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
)

const (
	connectTimeout = 8 * time.Second
	queryTimeout   = 15 * time.Second
)

type ConnectionAPI struct {
	apiContext

	profiles profile.Repository
	secrets  secret.Store
	sessions *session.Manager
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
	}
}

func (a *ConnectionAPI) ListProfiles() ([]Profile, error) {
	stored, err := a.profiles.List()
	if err != nil {
		return nil, wrapError(err)
	}

	out := make([]Profile, 0, len(stored))
	for _, p := range stored {
		out = append(out, toProfileDTO(p, a.hasStoredPassword(p)))
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

	return toProfileDTO(target, a.hasStoredPassword(target)), nil
}

func (a *ConnectionAPI) DeleteProfile(id string) error {
	if err := a.profiles.Delete(id); err != nil {
		return wrapError(err)
	}
	if err := a.secrets.Delete(id); err != nil {
		return wrapError(err)
	}
	return nil
}

func (a *ConnectionAPI) TestConnection(input ProfileInput) (TestResult, error) {
	target := input.toDomain()
	if err := target.Validate(); err != nil {
		return TestResult{}, wrapError(err)
	}

	creds, err := a.resolveCredentials(target, input.Password)
	if err != nil {
		return TestResult{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(connectTimeout)
	defer cancel()

	started := time.Now()
	opened, err := a.sessions.Open(ctx, target, creds)
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

func (a *ConnectionAPI) Connect(profileID, password string) (Session, error) {
	stored, err := a.profiles.Get(profileID)
	if err != nil {
		return Session{}, wrapError(err)
	}

	creds, err := a.resolveCredentials(stored, password)
	if err != nil {
		return Session{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(connectTimeout)
	defer cancel()

	opened, err := a.sessions.Open(ctx, stored, creds)
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

func (a *ConnectionAPI) ListTables(sessionID string, ns Namespace) ([]string, error) {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	names, err := a.sessions.Tables(ctx, sessionID, ns.toDriver())
	if err != nil {
		return nil, wrapError(err)
	}
	return names, nil
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

func (a *ConnectionAPI) LoadTableRows(sessionID string, page TablePageInput) (TableRows, error) {
	target := page.toDriver()

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	rows, err := a.sessions.TableRows(ctx, sessionID, target)
	if err != nil {
		return TableRows{}, wrapError(err)
	}
	return toTableRowsDTO(rows, target), nil
}

func (a *ConnectionAPI) UpdateCell(sessionID string, edit CellEditInput) error {
	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	return wrapError(a.sessions.UpdateCell(ctx, sessionID, edit.toDriver()))
}

func (a *ConnectionAPI) resolveCredentials(
	target domain.ConnectionProfile,
	password string,
) (driver.Credentials, error) {
	if password != "" {
		return driver.Credentials{Password: password}, nil
	}
	if !target.SavePassword || target.ID == "" {
		return driver.Credentials{}, nil
	}

	stored, err := a.secrets.Get(target.ID)
	if errors.Is(err, domain.ErrPasswordNotFound) {
		return driver.Credentials{}, nil
	}
	if err != nil {
		return driver.Credentials{}, err
	}
	return driver.Credentials{Password: stored}, nil
}

func (a *ConnectionAPI) persistPassword(target domain.ConnectionProfile, password string) error {
	if !target.SavePassword {
		return a.secrets.Delete(target.ID)
	}
	if password == "" {
		return nil
	}
	return a.secrets.Set(target.ID, password)
}

func (a *ConnectionAPI) hasStoredPassword(target domain.ConnectionProfile) bool {
	if !target.SavePassword {
		return false
	}
	_, err := a.secrets.Get(target.ID)
	return err == nil
}

func newProfileID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
