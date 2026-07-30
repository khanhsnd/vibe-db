package app

import (
	"context"
	"fmt"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/mysql"
	"github.com/khanhsnd/vibe-db/internal/driver/postgres"
	"github.com/khanhsnd/vibe-db/internal/profile/jsonfile"
	"github.com/khanhsnd/vibe-db/internal/secret/keyring"
	"github.com/khanhsnd/vibe-db/internal/session"
)

type App struct {
	Connections *ConnectionAPI
	Queries     *QueryAPI

	sessions *session.Manager
}

func New() (*App, error) {
	profilePath, err := jsonfile.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	registry := driver.NewRegistry(mysql.New(), postgres.New())
	sessions := session.NewManager(registry)

	return &App{
		Connections: NewConnectionAPI(jsonfile.New(profilePath), keyring.New(), sessions),
		Queries:     NewQueryAPI(sessions),
		sessions:    sessions,
	}, nil
}

func (a *App) Startup(ctx context.Context) {
	a.Connections.useContext(ctx)
	a.Queries.useContext(ctx)
}

func (a *App) Shutdown(context.Context) {
	a.sessions.CloseAll()
}

func (a *App) Bindings() []any {
	return []any{a.Connections, a.Queries}
}
