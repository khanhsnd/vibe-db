package app

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestProfileInputEngineDecidesTheDefaultPort(t *testing.T) {
	tests := []struct {
		name       string
		in         ProfileInput
		wantEngine domain.Engine
		wantPort   int
	}{
		{
			name:       "a blank engine is MySQL, as a profile written before the second engine means",
			in:         ProfileInput{Name: "local"},
			wantEngine: domain.EngineMySQL,
			wantPort:   domain.DefaultMySQLPort,
		},
		{
			name:       "postgres without a port",
			in:         ProfileInput{Name: "local", Engine: "postgres"},
			wantEngine: domain.EnginePostgres,
			wantPort:   domain.DefaultPostgresPort,
		},
		{
			name:       "a named port wins over the engine default",
			in:         ProfileInput{Name: "local", Engine: "postgres", Port: 6543},
			wantEngine: domain.EnginePostgres,
			wantPort:   6543,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.toDomain()

			if got.Engine != tt.wantEngine {
				t.Errorf("Engine = %q, want %q", got.Engine, tt.wantEngine)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
		})
	}
}

func TestQueryInputMaxRows(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to the default", 0, DefaultQueryRows},
		{"negative falls back to the default", -5, DefaultQueryRows},
		{"a sane value is kept", 250, 250},
		{"a large typed value is not clamped", 250_000, 250_000},
		{"all becomes the driver's uncapped read", AllRows, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueryInput{SQL: "SELECT 1", MaxRows: tt.in}.toDriver()

			if got.MaxRows != tt.want {
				t.Errorf("MaxRows = %d, want %d", got.MaxRows, tt.want)
			}
		})
	}
}

func TestTablePageInputLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to the default", 0, DefaultPageSize},
		{"negative falls back to the default", -5, DefaultPageSize},
		{"a sane value is kept", 250, 250},
		{"a large typed value is not clamped", 250_000, 250_000},
		{"all becomes the driver's uncapped read", AllRows, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TablePageInput{Table: "users", Limit: tt.in}.toDriver()

			if got.Limit != tt.want {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.want)
			}
		})
	}
}

func TestTablePageInputClampsNegativeOffset(t *testing.T) {
	got := TablePageInput{Table: "users", Limit: 100, Offset: -1}.toDriver()

	if got.Offset != 0 {
		t.Errorf("Offset = %d, want 0", got.Offset)
	}
}
