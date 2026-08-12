package app

import (
	"context"
	"fmt"

	"github.com/khanhsnd/vibe-db/internal/backup"
	"github.com/khanhsnd/vibe-db/internal/dbtool"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/mongo"
	"github.com/khanhsnd/vibe-db/internal/driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/driver/oracle"
	"github.com/khanhsnd/vibe-db/internal/driver/postgres"
	"github.com/khanhsnd/vibe-db/internal/driver/redis"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlite"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlserver"
	"github.com/khanhsnd/vibe-db/internal/history"
	historyfile "github.com/khanhsnd/vibe-db/internal/history/jsonfile"
	profilefile "github.com/khanhsnd/vibe-db/internal/profile/jsonfile"
	"github.com/khanhsnd/vibe-db/internal/secret/keyring"
	"github.com/khanhsnd/vibe-db/internal/session"
	snippetfile "github.com/khanhsnd/vibe-db/internal/snippet/jsonfile"
)

type App struct {
	Connections *ConnectionAPI
	Queries     *QueryAPI
	System      *SystemAPI
	Updates     *UpdateAPI

	sessions *session.Manager
}

func New(version ...string) (*App, error) {
	profilePath, err := profilefile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	historyPath, err := historyfile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	snippetPath, err := snippetfile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	backupRoot, err := backup.DefaultRoot()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	toolPath, err := dbtool.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	sessions := session.NewManager(engines())
	currentVersion := "dev"
	if len(version) > 0 {
		currentVersion = version[0]
	}

	return &App{
		Connections: NewConnectionAPI(
			profilefile.New(profilePath),
			keyring.New(),
			sessions,
			backup.New(backupRoot),
			dbtool.New(toolPath),
		),
		Queries: NewQueryAPI(
			sessions,
			historyfile.New(historyPath),
			snippetfile.New(snippetPath),
			history.NewLog(),
		),
		System:   NewSystemAPI(),
		Updates:  NewUpdateAPI(currentVersion),
		sessions: sessions,
	}, nil
}

// engines is the registry every session is opened through, and the one place an adapter is named.
// It is a function of its own so a test can walk it: an engine `domain` knows about and this list
// does not is a profile the reader can save and then cannot connect — a mistake that costs one line
// to make and shows up nowhere until somebody tries it.
func engines() *driver.Registry {
	return driver.NewRegistry(
		mysql.New(),
		postgres.New(),
		sqlite.New(),
		sqlserver.New(),
		oracle.New(),
		mongo.New(),
		redis.New(),
	)
}

func (a *App) Startup(ctx context.Context) {
	a.Connections.useContext(ctx)
	a.Queries.useContext(ctx)
	a.System.useContext(ctx)
	a.Updates.useContext(ctx)
}

func (a *App) Shutdown(context.Context) {
	a.sessions.CloseAll()
}

func (a *App) Bindings() []any {
	return []any{a.Connections, a.Queries, a.System, a.Updates}
}
