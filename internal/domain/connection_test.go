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

func validProxy() ProxyOptions {
	return ProxyOptions{
		Enabled: true,
		Kind:    ProxySOCKS5,
		Host:    "10.0.0.9",
		Port:    DefaultSOCKSPort,
		User:    "reader",
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
		{"unsupported engine", func(p *ConnectionProfile) { p.Engine = "cassandra" }, "engine"},
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
		{"unknown access mode", func(p *ConnectionProfile) { p.Access = "append-only" }, "access"},
		{"read-only is an access mode", func(p *ConnectionProfile) { p.Access = AccessReadOnly }, ""},
		{"a valid proxy", func(p *ConnectionProfile) { p.Proxy = validProxy() }, ""},
		{
			"unknown proxy kind",
			func(p *ConnectionProfile) {
				p.Proxy = validProxy()
				p.Proxy.Kind = "socks4"
			},
			"proxyKind",
		},
		{
			"blank proxy host",
			func(p *ConnectionProfile) {
				p.Proxy = validProxy()
				p.Proxy.Host = "  "
			},
			"proxyHost",
		},
		{
			"proxy port out of range",
			func(p *ConnectionProfile) {
				p.Proxy = validProxy()
				p.Proxy.Port = 70000
			},
			"proxyPort",
		},
		{
			// The same rule the tunnel follows: off is off, not half-filled-in.
			"a disabled proxy is not checked",
			func(p *ConnectionProfile) { p.Proxy = ProxyOptions{} },
			"",
		},
		{
			// A proxy on a private network usually asks for nobody at all.
			"a proxy needs no user",
			func(p *ConnectionProfile) {
				p.Proxy = validProxy()
				p.Proxy.User = ""
			},
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
	if got := EngineMongo.DefaultPort(); got != DefaultMongoPort {
		t.Errorf("EngineMongo.DefaultPort() = %d, want %d", got, DefaultMongoPort)
	}
	if got := EngineRedis.DefaultPort(); got != DefaultRedisPort {
		t.Errorf("EngineRedis.DefaultPort() = %d, want %d", got, DefaultRedisPort)
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

// A Redis database is numbered rather than named, so the box takes an index — and anything that is
// not one is not a database this server has.
func TestValidateReadsARedisDatabaseAsANumber(t *testing.T) {
	profile := ConnectionProfile{
		Name:   "local keys",
		Engine: EngineRedis,
		Host:   "127.0.0.1",
		Port:   DefaultRedisPort,
		Auth:   AuthPassword,
		TLS:    TLSOptions{Mode: TLSPreferred},
	}

	// No user, and no database at all: both are a connection rather than a failure.
	if err := profile.Validate(); err != nil {
		t.Errorf("Validate() = %v, want no error", err)
	}

	profile.Database = "3"
	if err := profile.Validate(); err != nil {
		t.Errorf("Validate() with database 3 = %v, want no error", err)
	}

	for _, named := range []string{"shop", "-1", "3.5"} {
		profile.Database = named
		var invalid ValidationError
		if err := profile.Validate(); !errors.As(err, &invalid) || invalid.Field != "database" {
			t.Errorf("Validate() with database %q = %v, want it refused on the database", named, err)
		}
	}
}

// TestAccessRefusesEveryHazard is the read-only rule against the one it is the counterpart of: an
// Environment turns a hazard into a question, and this turns one into a refusal.
func TestAccessRefusesEveryHazard(t *testing.T) {
	hazards := []Hazard{HazardWrite, HazardUnfiltered, HazardDestructive}

	for _, hazard := range hazards {
		if !AccessReadOnly.Refuses(hazard, "delete") {
			t.Errorf("read-only allowed %s", hazard)
		}
		if AccessReadWrite.Refuses(hazard, "delete") {
			t.Errorf("read and write refused %s", hazard)
		}
	}

	if AccessReadOnly.Refuses(HazardNone, "select") {
		t.Error("read-only refused a SELECT")
	}
}

// TestAccessRefusesAWritingClause is where the two rules part company: a CTE writes without saying
// so in its first word, so a confirmation lets it by and a refusal does not.
func TestAccessRefusesAWritingClause(t *testing.T) {
	if !AccessReadOnly.Refuses(HazardNone, "with") {
		t.Error("read-only allowed a WITH, which may hold a DELETE")
	}
	if !AccessReadOnly.Refuses(HazardNone, " WITH ") {
		t.Error("read-only read the keyword without trimming or folding it")
	}
	if Environment("").NeedsConfirmation(HazardNone) {
		t.Error("a confirmation asked about a statement that reads")
	}
}

// TestProxyDefaultPortPerKind is why the default is not one number: a SOCKS proxy and an HTTP one
// are on different ports, and a single default would be wrong half the time.
func TestProxyDefaultPortPerKind(t *testing.T) {
	if got := ProxySOCKS5.DefaultPort(); got != DefaultSOCKSPort {
		t.Errorf("SOCKS5 default port = %d, want %d", got, DefaultSOCKSPort)
	}
	if got := ProxyHTTP.DefaultPort(); got != DefaultHTTPProxyPort {
		t.Errorf("HTTP default port = %d, want %d", got, DefaultHTTPProxyPort)
	}
}

func TestProxyAddressBracketsIPv6(t *testing.T) {
	proxy := ProxyOptions{Host: "::1", Port: DefaultSOCKSPort}
	if got, want := proxy.Address(), "[::1]:1080"; got != want {
		t.Errorf("Address() = %q, want %q", got, want)
	}
}
