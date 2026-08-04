package domain

import (
	"errors"
	"testing"
)

func validProfile() ConnectionProfile {
	return ConnectionProfile{
		ID:     "abc",
		Name:   "local",
		Engine: EngineMySQL,
		Host:   "127.0.0.1",
		Port:   DefaultMySQLPort,
		User:   "root",
		Auth:   AuthPassword,
		TLS:    TLSOptions{Mode: TLSPreferred},
	}
}

func validTunnel() SSHTunnel {
	return SSHTunnel{
		Enabled:       true,
		Host:          "bastion.example.com",
		Port:          DefaultSSHPort,
		User:          "deploy",
		Auth:          SSHKey,
		KeyPath:       `C:\Users\me\.ssh\id_ed25519`,
		StrictHostKey: true,
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
		{"unsupported engine", func(p *ConnectionProfile) { p.Engine = "oracle" }, "engine"},
		{"unknown tls mode", func(p *ConnectionProfile) { p.TLS.Mode = "sometimes" }, "tlsMode"},
		{"unknown environment", func(p *ConnectionProfile) { p.Environment = "uat" }, "environment"},
		{"unknown auth", func(p *ConnectionProfile) { p.Auth = "kerberos" }, "auth"},
		{
			"postgres without a database",
			func(p *ConnectionProfile) { p.Engine = EnginePostgres },
			"database",
		},
		{
			"postgres with a database",
			func(p *ConnectionProfile) { p.Engine, p.Database = EnginePostgres, "shop" },
			"",
		},
		{
			"a root certificate under a mode that never opens one",
			func(p *ConnectionProfile) { p.TLS.RootCert = `C:\ca.pem` },
			"tlsMode",
		},
		{
			"a root certificate under a mode that does",
			func(p *ConnectionProfile) {
				p.TLS.Mode, p.TLS.RootCert = TLSVerifyFull, `C:\ca.pem`
			},
			"",
		},
		{
			"a client certificate without its key",
			func(p *ConnectionProfile) {
				p.TLS.Mode, p.TLS.ClientCert = TLSRequired, `C:\client.pem`
			},
			"clientCert",
		},
		{
			"a client certificate with its key",
			func(p *ConnectionProfile) {
				p.TLS.Mode = TLSRequired
				p.TLS.ClientCert, p.TLS.ClientKey = `C:\client.pem`, `C:\client.key`
			},
			"",
		},
		{"a tunnel", func(p *ConnectionProfile) { p.SSH = validTunnel() }, ""},
		{
			"a tunnel with no host",
			func(p *ConnectionProfile) {
				p.SSH = validTunnel()
				p.SSH.Host = " "
			},
			"sshHost",
		},
		{
			"a tunnel with no port",
			func(p *ConnectionProfile) {
				p.SSH = validTunnel()
				p.SSH.Port = 0
			},
			"sshPort",
		},
		{
			"a tunnel with no user",
			func(p *ConnectionProfile) {
				p.SSH = validTunnel()
				p.SSH.User = ""
			},
			"sshUser",
		},
		{
			"key authentication with no key",
			func(p *ConnectionProfile) {
				p.SSH = validTunnel()
				p.SSH.KeyPath = ""
			},
			"sshKeyPath",
		},
		{
			"agent authentication needs no key",
			func(p *ConnectionProfile) {
				p.SSH = validTunnel()
				p.SSH.Auth, p.SSH.KeyPath = SSHAgent, ""
			},
			"",
		},
		{
			// A tunnel nobody turned on is not half-filled-in, it is off.
			"a disabled tunnel is not checked",
			func(p *ConnectionProfile) { p.SSH = SSHTunnel{} },
			"",
		},
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

func TestDefaultPortPerEngine(t *testing.T) {
	if got := EngineMySQL.DefaultPort(); got != DefaultMySQLPort {
		t.Errorf("EngineMySQL.DefaultPort() = %d, want %d", got, DefaultMySQLPort)
	}
	if got := EnginePostgres.DefaultPort(); got != DefaultPostgresPort {
		t.Errorf("EnginePostgres.DefaultPort() = %d, want %d", got, DefaultPostgresPort)
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

// A MongoDB server started without access control takes no credentials at all, so a profile that
// names no user is a connection rather than a validation failure.
func TestValidateDoesNotAskAMongoProfileForAUser(t *testing.T) {
	profile := ConnectionProfile{
		Name:   "local documents",
		Engine: EngineMongo,
		Host:   "127.0.0.1",
		Port:   DefaultMongoPort,
		Auth:   AuthPassword,
		TLS:    TLSOptions{Mode: TLSPreferred},
	}

	if err := profile.Validate(); err != nil {
		t.Errorf("Validate() = %v, want no error", err)
	}
}

// Every other engine authenticates every connection, so a blank user is still refused there.
func TestValidateStillAsksASQLProfileForAUser(t *testing.T) {
	for _, engine := range []Engine{EngineMySQL, EnginePostgres} {
		t.Run(string(engine), func(t *testing.T) {
			profile := ConnectionProfile{
				Name:     "server",
				Engine:   engine,
				Host:     "127.0.0.1",
				Port:     engine.DefaultPort(),
				Database: "shop",
				Auth:     AuthPassword,
				TLS:      TLSOptions{Mode: TLSPreferred},
			}

			if err := profile.Validate(); err == nil {
				t.Error("Validate() = no error, want one about the user")
			}
		})
	}
}

// A MongoDB session reads every database on the server, so a profile naming none has something to
// connect to — unlike PostgreSQL's, which is bound to one for its whole life.
func TestValidateDoesNotAskAMongoProfileForADatabase(t *testing.T) {
	profile := ConnectionProfile{
		Name:   "local documents",
		Engine: EngineMongo,
		Host:   "127.0.0.1",
		Port:   DefaultMongoPort,
		User:   "root",
		Auth:   AuthPassword,
		TLS:    TLSOptions{Mode: TLSPreferred},
	}

	if err := profile.Validate(); err != nil {
		t.Errorf("Validate() = %v, want no error", err)
	}
}

func TestMongoDefaultPort(t *testing.T) {
	if got := EngineMongo.DefaultPort(); got != DefaultMongoPort {
		t.Errorf("DefaultPort() = %d, want %d", got, DefaultMongoPort)
	}
}
