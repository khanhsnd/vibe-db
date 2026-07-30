package driver

import (
	"context"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

type Credentials struct {
	Password string
}

type ServerInfo struct {
	Version string
}

type Driver interface {
	Engine() domain.Engine
	Open(ctx context.Context, profile domain.ConnectionProfile, creds Credentials) (Session, error)
}

type Session interface {
	Ping(ctx context.Context) error
	ServerInfo(ctx context.Context) (ServerInfo, error)
	Databases(ctx context.Context) ([]string, error)
	Close() error
}

type Registry struct {
	drivers map[domain.Engine]Driver
}

func NewRegistry(drivers ...Driver) *Registry {
	r := &Registry{drivers: make(map[domain.Engine]Driver, len(drivers))}
	for _, d := range drivers {
		r.drivers[d.Engine()] = d
	}
	return r
}

func (r *Registry) For(engine domain.Engine) (Driver, error) {
	d, ok := r.drivers[engine]
	if !ok {
		return nil, domain.ValidationError{Field: "engine", Message: "is not supported"}
	}
	return d, nil
}
