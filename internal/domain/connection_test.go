package domain

import (
	"errors"
	"testing"
)

func validProfile() ConnectionProfile {
	return ConnectionProfile{
		ID:      "abc",
		Name:    "local",
		Engine:  EngineMySQL,
		Host:    "127.0.0.1",
		Port:    DefaultMySQLPort,
		User:    "root",
		TLSMode: TLSPreferred,
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ConnectionProfile)
		wantField string
	}{
		{"valid", func(*ConnectionProfile) {}, ""},
		{"blank name", func(p *ConnectionProfile) { p.Name = "  " }, "name"},
		{"blank host", func(p *ConnectionProfile) { p.Host = "" }, "host"},
		{"port zero", func(p *ConnectionProfile) { p.Port = 0 }, "port"},
		{"port too high", func(p *ConnectionProfile) { p.Port = 70000 }, "port"},
		{"port negative", func(p *ConnectionProfile) { p.Port = -1 }, "port"},
		{"blank user", func(p *ConnectionProfile) { p.User = "" }, "user"},
		{"unsupported engine", func(p *ConnectionProfile) { p.Engine = "postgres" }, "engine"},
		{"unknown tls mode", func(p *ConnectionProfile) { p.TLSMode = "sometimes" }, "tlsMode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)
			err := p.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}

			var validationErr ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Validate() = %v, want ValidationError", err)
			}
			if validationErr.Field != tt.wantField {
				t.Errorf("field = %q, want %q", validationErr.Field, tt.wantField)
			}
			if !errors.Is(err, ErrInvalidProfile) {
				t.Error("ValidationError should unwrap to ErrInvalidProfile")
			}
		})
	}
}

func TestAddressBracketsIPv6(t *testing.T) {
	p := validProfile()
	p.Host = "::1"

	if got, want := p.Address(), "[::1]:3306"; got != want {
		t.Errorf("Address() = %q, want %q", got, want)
	}
}

func TestTargetFallsBackWhenNoDatabase(t *testing.T) {
	p := validProfile()

	if got, want := p.Target(), "(no database)@127.0.0.1:3306"; got != want {
		t.Errorf("Target() = %q, want %q", got, want)
	}

	p.Database = "shop"
	if got, want := p.Target(), "shop@127.0.0.1:3306"; got != want {
		t.Errorf("Target() = %q, want %q", got, want)
	}
}
