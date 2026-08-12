package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/cloudauth"
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/sshtunnel"
)

// Secrets is everything a connection is opened with that the profile does not hold: the password
// the *database* takes, and — for a profile that reaches it through a bastion — what the *bastion*
// takes, which is a password or the pass phrase of a private key.
//
// A profile authenticating by a cloud identity leaves Password empty and it is minted here; see
// Open.
type Secrets struct {
	Password string
	SSH      string
}

type Session struct {
	ID            string
	Profile       domain.ConnectionProfile
	ServerVersion string

	driverSession driver.Session
	// tunnel is the bastion this session is carried over, and is nil for a direct connection.
	// It lives exactly as long as the session: closing one closes the other.
	tunnel *sshtunnel.Tunnel
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

// Open dials a server, doing in order the three things that have to happen before an engine
// adapter is given anything: the profile is checked, the bastion is stood up, and the password is
// minted where the profile says it is not known but signed for. All three are here rather than in
// an adapter because none of them varies by engine — see the dependency rule in
// docs/architecture.md.
func (m *Manager) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	secrets Secrets,
) (*Session, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	engineDriver, err := m.registry.For(profile.Engine)
	if err != nil {
		return nil, err
	}

	tunnel, err := openTunnel(ctx, profile, secrets)
	if err != nil {
		return nil, err
	}

	// Anything that fails from here on has a tunnel to take down with it.
	fail := func(err error) (*Session, error) {
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, err
	}

	creds, err := credentials(ctx, profile, secrets)
	if err != nil {
		return fail(err)
	}

	driverSession, err := engineDriver.Open(ctx, profile, creds, dialerFor(tunnel))
	if err != nil {
		return fail(err)
	}

	info, err := driverSession.ServerInfo(ctx)
	if err != nil {
		driverSession.Close()
		return fail(err)
	}

	id, err := newID()
	if err != nil {
		driverSession.Close()
		return fail(err)
	}

	s := &Session{
		ID:            id,
		Profile:       profile,
		ServerVersion: info.Version,
		driverSession: driverSession,
		tunnel:        tunnel,
	}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	return s, nil
}

func openTunnel(
	ctx context.Context,
	profile domain.ConnectionProfile,
	secrets Secrets,
) (*sshtunnel.Tunnel, error) {
	if !profile.SSH.Enabled {
		return nil, nil
	}
	return sshtunnel.Open(ctx, profile.SSH, sshtunnel.Secrets{Secret: secrets.SSH})
}

// dialerFor is the tunnel as the driver layer sees it: a function, and nil where there is none.
// A typed nil pointer would not be nil once it were a `driver.Dialer`, which is exactly the bug
// that would make every direct connection try to dial through nothing.
func dialerFor(tunnel *sshtunnel.Tunnel) driver.Dialer {
	if tunnel == nil {
		return nil
	}
	return tunnel.Dial
}

// credentials is the password the database will be given. For most profiles it is the one the
// reader knows; for a cloud identity it is a token minted here, per connection, and never stored:
// it is good for minutes, and one read back out of a keychain later would only fail.
func credentials(
	ctx context.Context,
	profile domain.ConnectionProfile,
	secrets Secrets,
) (driver.Credentials, error) {
	switch profile.Auth {
	case domain.AuthAWSIAM:
		token, err := cloudauth.AWS(ctx, profile.Region, profile.Host, profile.Port, profile.User)
		return driver.Credentials{Password: token}, err

	case domain.AuthGoogleIAM:
		token, err := cloudauth.Google(ctx)
		return driver.Credentials{Password: token}, err

	default:
		return driver.Credentials{Password: secrets.Password}, nil
	}
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
	return s.close()
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, s := range sessions {
		s.close()
	}
}

// close takes the pools down first and the tunnel after them, because every one of those pools
// is carried over it. The session's own failure is what is reported: a tunnel that will not shut
// cleanly is not something the reader who pressed Disconnect can do anything about.
func (s *Session) close() error {
	err := s.driverSession.Close()
	if s.tunnel != nil {
		s.tunnel.Close()
	}
	return err
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

func (m *Manager) Objects(
	ctx context.Context,
	id string,
	ns driver.Namespace,
) (driver.Objects, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.Objects{}, err
	}
	return s.driverSession.Objects(ctx, ns)
}

func (m *Manager) TableList(
	ctx context.Context,
	id string,
	ns driver.Namespace,
) (driver.TableList, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.TableList{}, err
	}
	return s.driverSession.TableList(ctx, ns)
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

// Security hands out the account half of a session, and the environment it would run in beside it.
// The two travel together for the reason Review's do: a change to an account carries a hazard, and
// turning that into a question the reader has to answer is `domain.Environment.NeedsConfirmation`'s.
func (m *Manager) Security(id string) (driver.Security, domain.Environment, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, "", err
	}
	return s.driverSession, s.Profile.Environment, nil
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

func (m *Manager) ColumnValues(
	ctx context.Context,
	id string,
	pick driver.ColumnPick,
) ([]string, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return s.driverSession.ColumnValues(ctx, pick)
}

func (m *Manager) MatchingRows(
	ctx context.Context,
	id string,
	search driver.RowSearch,
) (driver.RowMatches, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.RowMatches{}, err
	}
	return s.driverSession.MatchingRows(ctx, search)
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

// Text is the lexical half of the engine this session speaks: cutting a script into statements,
// laying it out, and reading what each statement will do. None of it needs the connection — it is
// reached through a session only because that is what the caller has, and what says which engine.
func (m *Manager) Text(id string) (driver.Text, error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	engineDriver, err := m.registry.For(s.Profile.Engine)
	if err != nil {
		return nil, err
	}
	return engineDriver.Text(), nil
}

func (m *Manager) Split(id, script string) ([]string, error) {
	text, err := m.Text(id)
	if err != nil {
		return nil, err
	}
	return text.Split(script), nil
}

func (m *Manager) Format(id, script string) (string, error) {
	text, err := m.Text(id)
	if err != nil {
		return "", err
	}
	return text.Format(script), nil
}

// Review reads what a script will do, and answers with the environment it would do it in — which
// is what turns a hazard into a question the reader has to answer. Pairing the two is the app
// layer's, through domain.Environment.NeedsConfirmation; this only hands over both halves.
func (m *Manager) Review(id, script string) (driver.Review, domain.Environment, error) {
	s, err := m.Get(id)
	if err != nil {
		return driver.Review{}, "", err
	}

	text, err := m.Text(id)
	if err != nil {
		return driver.Review{}, "", err
	}
	return text.Review(script), s.Profile.Environment, nil
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

// Begin, Commit and Rollback are the reader's own transaction, and they are the session's rather
// than a tab's for the reason the driver gives: a transaction is one connection, and this is what
// holds it. Anything the session runs between a Begin and its end runs inside it.
func (m *Manager) Begin(ctx context.Context, id string, ns driver.Namespace) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.Begin(ctx, ns)
}

func (m *Manager) Commit(ctx context.Context, id string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.Commit(ctx)
}

func (m *Manager) Rollback(ctx context.Context, id string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.driverSession.Rollback(ctx)
}

// InTransaction asks no server anything, so a connection that has gone away answers `false`
// rather than an error: what it reports is whether *this* side is holding one open.
func (m *Manager) InTransaction(id string) bool {
	s, err := m.Get(id)
	if err != nil {
		return false
	}
	return s.driverSession.InTransaction()
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
