package app

import (
	"context"
	"fmt"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/mongo"
	"github.com/khanhsnd/vibe-db/internal/driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/driver/postgres"
	"github.com/khanhsnd/vibe-db/internal/driver/redis"
	historyfile "github.com/khanhsnd/vibe-db/internal/history/jsonfile"
	profilefile "github.com/khanhsnd/vibe-db/internal/profile/jsonfile"
	"github.com/khanhsnd/vibe-db/internal/secret/keyring"
	"github.com/khanhsnd/vibe-db/internal/session"
)

type App struct {
	Connections *ConnectionAPI
	Queries     *QueryAPI
	System      *SystemAPI

	sessions *session.Manager
}

func New() (*App, error) {
	profilePath, err := profilefile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	historyPath, err := historyfile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	registry := driver.NewRegistry(mysql.New(), postgres.New(), mongo.New(), redis.New())
	sessions := session.NewManager(registry)

	return &App{
		Connections: NewConnectionAPI(profilefile.New(profilePath), keyring.New(), sessions),
		Queries:     NewQueryAPI(sessions, historyfile.New(historyPath)),
		System:      NewSystemAPI(),
		sessions:    sessions,
	}, nil
}

func (a *App) Startup(ctx context.Context) {
	a.Connections.useContext(ctx)
	a.Queries.useContext(ctx)
	a.System.useContext(ctx)
}

func (a *App) Shutdown(context.Context) {
	a.sessions.CloseAll()
}

func (a *App) Bindings() []any {
	return []any{a.Connections, a.Queries, a.System}
}
