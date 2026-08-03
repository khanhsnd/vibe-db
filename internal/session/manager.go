package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

type Session struct {
	ID            string
	Profile       domain.ConnectionProfile
	ServerVersion string

	driverSession driver.Session
}

type Manager struct {
	registry *driver.Registry

	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager(registry *driver.Registry) *Manager {
	return &Manager{
		registry: registry,
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
) (*Session, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	engineDriver, err := m.registry.For(profile.Engine)
	if err != nil {
		return nil, err
	}

	driverSession, err := engineDriver.Open(ctx, profile, creds)
	if err != nil {
		return nil, err
	}

	info, err := driverSession.ServerInfo(ctx)
	if err != nil {
		driverSession.Close()
		return nil, err
	}

	id, err := newID()
	if err != nil {
		driverSession.Close()
		return nil, err
	}

	s := &Session{
		ID:            id,
		Profile:       profile,
		ServerVersion: info.Version,
		driverSession: driverSession,
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return s, nil
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()

	if !ok {
		return domain.ErrSessionNotFound
	}
	return s.driverSession.Close()
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, s := range sessions {
		s.driverSession.Close()
	}
}

func (m *Manager) Databases(ctx context.Context, id string) ([]string, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession.Databases(ctx)
}

func (m *Manager) Schemas(ctx context.Context, id, database string) ([]string, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession.Schemas(ctx, database)
}

func (m *Manager) Tables(
	ctx context.Context,
	id string,
	ns driver.Namespace,
) ([]string, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession.Tables(ctx, ns)
}

func (m *Manager) TableSchemas(
	ctx context.Context,
	id string,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession.TableSchemas(ctx, ns)
}

// Schema hands out the DDL half of a session, so a schema change is one lookup here and
// then the driver's own method rather than a pass-through per statement.
func (m *Manager) Schema(id string) (driver.Schema, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession, nil
}

func (m *Manager) TableRows(
	ctx context.Context,
	id string,
	page driver.TablePage,
) (driver.TableRows, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.TableRows{}, err
	}
	return s.driverSession.TableRows(ctx, page)
}

func (m *Manager) Query(
	ctx context.Context,
	id string,
	query driver.Query,
) (driver.QueryResult, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.QueryResult{}, err
	}
	return s.driverSession.Query(ctx, query)
}

func (m *Manager) UpdateCell(ctx context.Context, id string, edit driver.CellEdit) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.UpdateCell(ctx, edit)
}

func (m *Manager) InsertRow(ctx context.Context, id string, insert driver.RowInsert) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.InsertRow(ctx, insert)
}

func (m *Manager) DeleteRow(ctx context.Context, id string, removal driver.RowDelete) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.DeleteRow(ctx, removal)
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
