package app

import (
	"context"
	"fmt"

	"github.com/khanhsnd/vibe-db/internal/backup"
	"github.com/khanhsnd/vibe-db/internal/dbtool"
	"github.com/khanhsnd/vibe-db/internal/domain"
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
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/objectstore/s3"
	profilefile "github.com/khanhsnd/vibe-db/internal/profile/jsonfile"
	"github.com/khanhsnd/vibe-db/internal/schedule"
	"github.com/khanhsnd/vibe-db/internal/secret/keyring"
	"github.com/khanhsnd/vibe-db/internal/session"
	"github.com/khanhsnd/vibe-db/internal/sigv4"
	snippetfile "github.com/khanhsnd/vibe-db/internal/snippet/jsonfile"
)

type App struct {
	Connections *ConnectionAPI
	Queries     *QueryAPI
	// Stores is the object-store half, and it is a bound API of its own for the reason
	// `domain.StoreProfile` is a profile of its own: a bucket has no engine, no namespace and
	// nothing that is a row — see internal/app/store_api.go.
	Stores  *StoreAPI
	System  *SystemAPI
	Updates *UpdateAPI

	sessions *session.Manager
	version  string
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

	schedulePath, err := schedule.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	storePath, err := profilefile.DefaultStorePath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	sessions := session.NewManager(engines())
	stores := session.NewStoreManager(openStore)
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
			schedule.New(schedulePath),
		),
		Queries: NewQueryAPI(
			sessions,
			historyfile.New(historyPath),
			snippetfile.New(snippetPath),
			history.NewLog(),
		),
		Stores:   NewStoreAPI(profilefile.NewStores(storePath), keyring.New(), stores),
		System:   NewSystemAPI(),
		Updates:  NewUpdateAPI(currentVersion),
		sessions: sessions,
		version:  currentVersion,
	}, nil
}

// openStore is where the one object-store adapter is named, and it stands exactly where `engines` does
// for a database: `internal/session` opens a store through this and has never heard of S3. There is no
// registry because there is nothing to key one by — every service the app speaks to speaks the same
// protocol, and which one it is is a preset on the profile (`domain.StoreProvider`).
func openStore(
	profile domain.StoreProfile,
	creds objectstore.Credentials,
	dial objectstore.Dialer,
) (objectstore.Store, error) {
	return s3.Open(s3.Options{
		Profile: profile,
		Credentials: sigv4.Credentials{
			AccessKeyID:     creds.AccessKeyID,
			SecretAccessKey: creds.SecretAccessKey,
			SessionToken:    creds.SessionToken,
		},
		Dial: dial,
	})
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
	a.Stores.useContext(ctx)
	a.Updates.useContext(ctx)

	// The clock starts once the window is up, and not before: a job that fires has a list to report
	// to, and nothing here is a service — see internal/app/scheduler.go.
	a.Connections.scheduler.start(ctx)
}

func (a *App) Shutdown(context.Context) {
	a.Connections.scheduler.close()
	// The queue goes down before the sessions it is reading through: a transfer still moving would
	// otherwise be a transfer reading a store that had just been closed underneath it.
	a.Stores.close()
	a.sessions.CloseAll()
}

func (a *App) Bindings() []any {
	return []any{a.Connections, a.Queries, a.Stores, a.System, a.Updates}
}
