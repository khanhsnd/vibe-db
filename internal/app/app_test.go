package app

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Every engine the domain will accept on a profile has to have an adapter behind it. Without this,
// adding one to `domain.Engine` and forgetting the registry line is a profile the reader can fill
// in, save and test — and which then fails to connect with "engine is not supported", naming a
// field they filled in correctly.
//
// The list is written out rather than derived, which is the point: adding an engine means adding it
// here, and this test is where a reader is reminded of the other end.
func TestEveryEngineHasAnAdapter(t *testing.T) {
	registry := engines()

	all := []domain.Engine{
		domain.EngineMySQL,
		domain.EnginePostgres,
		domain.EngineSQLite,
		domain.EngineSQLServer,
		domain.EngineOracle,
		domain.EngineMongo,
		domain.EngineRedis,
	}

	for _, engine := range all {
		t.Run(string(engine), func(t *testing.T) {
			if !engine.Valid() {
				t.Fatalf("%s is registered but domain.Engine.Valid refuses it", engine)
			}

			found, err := registry.For(engine)
			if err != nil {
				t.Fatalf("registry.For(%s) = %v", engine, err)
			}
			if found.Engine() != engine {
				t.Errorf("adapter for %s answers with %s", engine, found.Engine())
			}
			if found.Text() == nil {
				t.Errorf("%s has no Text: a script is laid out whether or not anything is dialled", engine)
			}
		})
	}

	if _, err := registry.For("cassandra"); err == nil {
		t.Error("registry.For accepted an engine that has no adapter")
	}
}
