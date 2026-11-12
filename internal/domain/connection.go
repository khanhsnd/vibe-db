package domain

import (
	"net"
	"strconv"
	"strings"
)

type Engine string

const (
	EngineMySQL     Engine = "mysql"
	EnginePostgres  Engine = "postgres"
	EngineMongo     Engine = "mongo"
	EngineRedis     Engine = "redis"
	EngineSQLite    Engine = "sqlite"
	EngineSQLServer Engine = "sqlserver"
	EngineOracle    Engine = "oracle"
)

const (
	DefaultMySQLPort     = 3306
	DefaultPostgresPort  = 5432
	DefaultMongoPort     = 27017
	DefaultRedisPort     = 6379
	DefaultSQLServerPort = 1433
	DefaultOraclePort    = 1521
	DefaultSSHPort       = 22
	// The two ports a proxy is nearly always on. They differ per kind, which is why the dialog
	// fills one in from the picker rather than offering a single number that is wrong half the time.
	DefaultSOCKSPort     = 1080
	DefaultHTTPProxyPort = 8080
)

// DefaultPort is the port an engine listens on when the profile names none. A file engine has
// none at all, and says so with a zero rather than borrowing somebody else's number.
func (e Engine) DefaultPort() int {
	switch e {
	case EnginePostgres:
		return DefaultPostgresPort
	case EngineMongo:
		return DefaultMongoPort
	case EngineRedis:
		return DefaultRedisPort
	case EngineSQLServer:
		return DefaultSQLServerPort
	case EngineOracle:
		return DefaultOraclePort
	case EngineSQLite:
		return 0
	default:
		return DefaultMySQLPort
	}
}

func (e Engine) Valid() bool {
	switch e {
	case EngineMySQL, EnginePostgres, EngineMongo, EngineRedis,
		EngineSQLite, EngineSQLServer, EngineOracle:
		return true
	default:
		return false
	}
}

// Documents says the engine stores documents rather than rows, which is the one fact about an
// engine the rest of the app has to know without asking a driver: a document has no declared
// column list, so the fields a collection shows are read off the documents themselves and a
// profile is not asked for a user name it may not have.
func (e Engine) Documents() bool {
	return e == EngineMongo
}

// Keys says the engine stores values under keys and nothing else: one flat key space to a
// database, with no tables in it to name and no columns to declare. It is the second fact of
// Documents' kind — what a reader sees is a key, its type and its value, and every word the app
// would otherwise say about a table is untrue here.
func (e Engine) Keys() bool {
	return e == EngineRedis
}

// File says the whole database is one file on this machine rather than a server somewhere: there
// is nothing to dial and nobody to be, so the profile's Database is a *path* and a host, a port
// and a user would each be a box asking for something that does not exist.
func (e Engine) File() bool {
	return e == EngineSQLite
}

// Backups says a database of this engine can be written out as a file that would put it back.
// A backup here *is* that file — the dump's own statements, under a header — so the engines that
// have one are exactly the engines whose statements declare a table and write a row. A document
// store's commands and a key space's keys are neither, and claiming a backup for them would be
// promising a restore nothing could perform.
func (e Engine) Backups() bool {
	return !e.Documents() && !e.Keys()
}

// NeedsUser says every connection to this engine is made *as* somebody the server knows. A MongoDB
// server started without access control and a Redis server with no ACL both take none at all, and
// a file has nobody to be, so insisting on a user name for any of the three would be refusing a
// profile that connects perfectly well.
func (e Engine) NeedsUser() bool {
	switch e {
	case EngineMySQL, EnginePostgres, EngineSQLServer, EngineOracle:
		return true
	default:
		return false
	}
}

// NeedsDatabase says a connection cannot be made without one being named, and the three that do
// mean three different things by it: a PostgreSQL session is bound to one database for its whole
// life, an Oracle connection asks the listener for a *service*, and a SQLite database is the file
// itself. Everything else is browsed from wherever the server put the connection.
func (e Engine) NeedsDatabase() bool {
	switch e {
	case EnginePostgres, EngineOracle, EngineSQLite:
		return true
	default:
		return false
	}
}

// Environment is what a server is *for*, and the one fact about a profile that changes how the
// app behaves rather than how it connects: every surface wearing a connection's name wears its
// colour, and a statement that cannot be taken back is confirmed before it runs on a guarded one.
type Environment string

const (
	// EnvironmentUnset is a profile that has not said, which is every profile written before
	// this field existed. It is coloured like the app and guards nothing.
	EnvironmentUnset       Environment = ""
	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
)

func (e Environment) Valid() bool {
	switch e {
	case EnvironmentUnset, EnvironmentDevelopment, EnvironmentStaging, EnvironmentProduction:
		return true
	default:
		return false
	}
}

// Guarded says a statement that cannot be taken back has to be answered for before it runs here.
// Production alone is: a warning a reader meets on every environment is one they stop reading,
// and that habit is exactly what this exists to prevent.
func (e Environment) Guarded() bool {
	return e == EnvironmentProduction
}

// NeedsConfirmation is the rule the query tab asks before it runs anything. Two things are
// confirmed, and for different reasons: **anything at all that writes** on a guarded environment,
// because the cost of being on the wrong connection is the whole point of labelling one; and an
// **unfiltered** write anywhere, because `DELETE FROM users` with no WHERE is a mistake on a
// laptop as surely as it is in production.
func (e Environment) NeedsConfirmation(h Hazard) bool {
	if h == HazardNone {
		return false
	}
	return e.Guarded() || h == HazardUnfiltered
}

// Access is what this app will let a connection do at all. It is the second fact about a profile
// that changes how the app *behaves* rather than how it connects, and it is the harder of the two:
// an Environment turns a hazard into a **question**, and this turns one into a **refusal**.
//
// It is a client-side guard and says so plainly. A read-only profile refuses to *send* what would
// write — nothing reaches the server to be rejected by it — which is exactly what protects a reader
// who opened the wrong tab, and exactly what does not protect a database from anybody who can point
// another client at it. **A read-only account on the server is the protection**; this is the guard.
type Access string

const (
	// AccessReadWrite is a connection with nothing held back, which is every profile written before
	// this field existed.
	AccessReadWrite Access = ""
	AccessReadOnly  Access = "read-only"
)

func (a Access) Valid() bool {
	switch a {
	case AccessReadWrite, AccessReadOnly:
		return true
	default:
		return false
	}
}

func (a Access) ReadOnly() bool {
	return a == AccessReadOnly
}

// writesInAClause is every leading keyword whose statement may write without saying so in its first
// word. There is one: `WITH names AS (DELETE FROM …) SELECT …` is a single statement whose verb is
// inside a parenthesis, and PostgreSQL and SQL Server both run it.
//
// StatementHazard leaves it alone on purpose — a warning on every ordinary CTE is a warning a reader
// stops reading — and this is where the two rules part company. **A confirmation defaults to
// allowing and a refusal defaults to refusing**, because being wrong costs the opposite thing in
// each: a question nobody needed is a nuisance, and a write nobody expected is the mistake the mode
// exists to prevent.
var writesInAClause = map[string]bool{"with": true}

// Refuses is the rule a read-only connection is held to, and is Environment.NeedsConfirmation's
// counterpart: the same Hazard, read by the same reviewer, answered with "not at all" rather than
// with "are you sure".
//
// What it cannot see it says so about: a hazard is read off a statement's first word, so a procedure
// whose body writes is a `CALL` — which *is* refused — while a statement this side cannot tell apart
// from a read is not. See the note on Access.
func (a Access) Refuses(hazard Hazard, keyword string) bool {
	if !a.ReadOnly() {
		return false
	}
	return hazard != HazardNone || writesInAClause[strings.ToLower(strings.TrimSpace(keyword))]
}

// Colour is the tint a connection wears down the sidebar, and is the only thing a profile carries
// that no server ever hears about: it is how a reader tells two connections apart when the names
// are alike and the environments are the same. The palette is closed on purpose — a colour picked
// from a wheel would be a colour that reads on one theme and disappears on the other.
//
// ColourNone is a connection that has not picked one, which is every connection saved before this
// existed, and it wears the sidebar's own background.
type Colour string

const (
	ColourNone   Colour = ""
	ColourRed    Colour = "red"
	ColourOrange Colour = "orange"
	ColourYellow Colour = "yellow"
	ColourGreen  Colour = "green"
	ColourTeal   Colour = "teal"
	ColourBlue   Colour = "blue"
	ColourPurple Colour = "purple"
	ColourPink   Colour = "pink"
)

func (c Colour) Valid() bool {
	switch c {
	case ColourNone, ColourRed, ColourOrange, ColourYellow, ColourGreen,
		ColourTeal, ColourBlue, ColourPurple, ColourPink:
		return true
	default:
		return false
	}
}

type TLSMode string

const (
	TLSDisabled   TLSMode = "disabled"
	TLSPreferred  TLSMode = "preferred"
	TLSRequired   TLSMode = "required"
	TLSVerifyCA   TLSMode = "verify-ca"
	TLSVerifyFull TLSMode = "verify-full"
)

func (m TLSMode) Valid() bool {
	switch m {
	case TLSDisabled, TLSPreferred, TLSRequired, TLSVerifyCA, TLSVerifyFull:
		return true
	default:
		return false
	}
}

// Verifies says the mode holds the server to a certificate this machine can check, which is what
// decides whether a root certificate is worth carrying. `required` encrypts and verifies nothing,
// which is the honest reading of both engines' own middle mode.
func (m TLSMode) Verifies() bool {
	return m == TLSVerifyCA || m == TLSVerifyFull
}

// Encrypts says TLS is definitely in force. `preferred` is left out on purpose: it takes TLS
// where the server offers it and plain text where it does not, so it can promise neither.
func (m TLSMode) Encrypts() bool {
	return m == TLSRequired || m.Verifies()
}

// TLSOptions is the whole of what a profile says about the transport's own security: the mode,
// and the files this machine holds the server — and itself — to. A path is a path on *this*
// machine, which is why none of them is ever sent anywhere.
type TLSOptions struct {
	Mode TLSMode
	// RootCert is the authority the server's chain is checked against. Empty falls back to the
	// machine's own trust store, which is what a server with a publicly issued certificate needs
	// and what a private authority — a cloud provider's bundle — does not.
	RootCert string
	// ClientCert and ClientKey are the certificate this client presents. Neither is any use
	// without the other, so a profile carrying one and not the other is refused.
	ClientCert string
	ClientKey  string
	// ServerName is the name the certificate is checked against when it is not the host dialled,
	// which is what a tunnel makes ordinary: the connection goes to 127.0.0.1 and the certificate
	// is still the server's own.
	ServerName string
}

func (o TLSOptions) Validate() error {
	if !o.Mode.Valid() {
		return ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}

	// A file named under a mode that never opens one is not a harmless extra: it is a reader
	// believing they have configured something. Say so rather than ignoring it.
	if !o.Mode.Encrypts() && (o.RootCert != "" || o.ClientCert != "" || o.ClientKey != "") {
		return ValidationError{
			Field:   "tlsMode",
			Message: "must be Required or higher for a certificate file to be used",
		}
	}
	if (o.ClientCert == "") != (o.ClientKey == "") {
		return ValidationError{
			Field:   "clientCert",
			Message: "needs both a certificate and its key",
		}
	}
	return nil
}

// SSHAuth is how the bastion authenticates the reader. The private key is the ordinary one and
// the default; a password is what a server still asking for one takes; and the agent is the
// key the reader has already unlocked, which is the only one of the three this app never holds.
type SSHAuth string

const (
	SSHKey      SSHAuth = "key"
	SSHPassword SSHAuth = "password"
	SSHAgent    SSHAuth = "agent"
)

func (a SSHAuth) Valid() bool {
	switch a {
	case SSHKey, SSHPassword, SSHAgent:
		return true
	default:
		return false
	}
}

// SSHTunnel is the bastion a profile reaches its server through. The database host and port stay
// what they are: they are resolved *on the far side* of the tunnel, which is why a server bound to
// localhost on the bastion is reached as `127.0.0.1` here and means the bastion's own loopback.
type SSHTunnel struct {
	Enabled bool
	Host    string
	Port    int
	User    string
	Auth    SSHAuth
	// KeyPath is the private key file, for SSHKey. Its pass phrase is a secret and lives in the
	// credential store beside the connection's own password, never here.
	KeyPath string
	// KnownHosts is the file the bastion's host key is checked against. Empty means the reader's
	// own `~/.ssh/known_hosts`, and a bastion missing from it is refused rather than trusted —
	// see StrictHostKey.
	KnownHosts string
	// StrictHostKey says an unknown bastion is refused. It defaults to on, and turning it off is
	// a deliberate choice a reader makes for a host they cannot enrol: an SSH tunnel whose far end
	// is not identified protects the credentials it carries from nobody.
	StrictHostKey bool
}

func (t SSHTunnel) Validate() error {
	if !t.Enabled {
		return nil
	}
	if strings.TrimSpace(t.Host) == "" {
		return ValidationError{Field: "sshHost", Message: "is required"}
	}
	if t.Port < 1 || t.Port > 65535 {
		return ValidationError{Field: "sshPort", Message: "must be between 1 and 65535"}
	}
	if strings.TrimSpace(t.User) == "" {
		return ValidationError{Field: "sshUser", Message: "is required"}
	}
	if !t.Auth.Valid() {
		return ValidationError{Field: "sshAuth", Message: "is not a known SSH authentication"}
	}
	if t.Auth == SSHKey && strings.TrimSpace(t.KeyPath) == "" {
		return ValidationError{Field: "sshKeyPath", Message: "is required for key authentication"}
	}
	return nil
}

func (t SSHTunnel) Address() string {
	return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

// ProxyKind is which of the two protocols the proxy speaks. Both do the same job — they take an
// address and hand back a connection to it — and they differ in one thing this app cares about:
// SOCKS5 carries the address as its own field and HTTP carries it in a `CONNECT` request line.
//
// There is no SOCKS4 here. It cannot carry a name, so every host would have to be resolved on
// *this* machine, which is exactly what a reader puts a proxy in front of a private network to
// avoid.
type ProxyKind string

const (
	ProxySOCKS5 ProxyKind = "socks5"
	ProxyHTTP   ProxyKind = "http"
)

func (k ProxyKind) Valid() bool {
	switch k {
	case ProxySOCKS5, ProxyHTTP:
		return true
	default:
		return false
	}
}

// DefaultPort is the port this kind of proxy is usually on, which is what the dialog fills in when
// the kind is picked.
func (k ProxyKind) DefaultPort() int {
	if k == ProxyHTTP {
		return DefaultHTTPProxyPort
	}
	return DefaultSOCKSPort
}

// ProxyOptions is the proxy every connection of this profile is carried through. It is **the outer
// hop**: a proxy is how this machine reaches the network at all, so a profile that has both a proxy
// and a bastion dials the bastion *through* the proxy and then the database through the bastion.
//
// The address it is asked for is resolved on the far side, exactly as a tunnel's is — which is the
// whole reason a proxy is worth having in front of a private network, and why the host and port on
// the General tab are left alone.
//
// A password is a secret and lives in the credential store under `<id>:proxy`, never here.
type ProxyOptions struct {
	Enabled bool
	Kind    ProxyKind
	Host    string
	Port    int
	// User is empty for a proxy that asks for nobody, which is most of them on a private network.
	User string
}

func (o ProxyOptions) Validate() error {
	if !o.Enabled {
		return nil
	}
	if !o.Kind.Valid() {
		return ValidationError{Field: "proxyKind", Message: "is not a known proxy type"}
	}
	if strings.TrimSpace(o.Host) == "" {
		return ValidationError{Field: "proxyHost", Message: "is required"}
	}
	if o.Port < 1 || o.Port > 65535 {
		return ValidationError{Field: "proxyPort", Message: "must be between 1 and 65535"}
	}
	return nil
}

func (o ProxyOptions) Address() string {
	return net.JoinHostPort(o.Host, strconv.Itoa(o.Port))
}

// AuthMethod is what the *database* authenticates the reader by. A password is one the reader
// knows; the other two are tokens this app mints per connection from a cloud identity the machine
// already holds, and are therefore never stored — a token that lasts a quarter of an hour is not
// a secret worth keeping.
type AuthMethod string

const (
	AuthPassword AuthMethod = "password"
	// AuthAWSIAM is an RDS IAM authentication token: the password is a request to the RDS
	// control plane, signed with the machine's AWS credentials and good for fifteen minutes.
	AuthAWSIAM AuthMethod = "aws-iam"
	// AuthGoogleIAM is a Google OAuth 2.0 access token from the machine's application default
	// credentials, which is what Cloud SQL IAM database authentication takes as a password.
	AuthGoogleIAM AuthMethod = "google-iam"
)

func (a AuthMethod) Valid() bool {
	switch a {
	case AuthPassword, AuthAWSIAM, AuthGoogleIAM:
		return true
	default:
		return false
	}
}

// Minted says the password is made at connect time rather than known, which is what decides
// whether there is anything to save, prompt for, or read back out of the credential store.
func (a AuthMethod) Minted() bool {
	return a == AuthAWSIAM || a == AuthGoogleIAM
}

type ConnectionProfile struct {
	ID          string
	Name        string
	Engine      Engine
	Environment Environment
	Colour      Colour
	// Access is what this app will let the connection do at all — see Access, and note that it is
	// the second fact about a profile that changes how the app *behaves* rather than how it
	// connects. Environment decides what is answered for; this decides what is not sent.
	Access Access
	// MCP says an AI agent may reach this connection through `vibe-db mcp`. It is off until the
	// reader turns it on, because a connection an agent can see is one a prompt can name — see
	// ForAgent, which is what an agent is handed instead of this profile.
	MCP      bool
	Host     string
	Port     int
	User     string
	Database string
	Auth     AuthMethod
	// Region is the AWS region the IAM token is signed for. Empty reads it out of the host name,
	// which is where an RDS endpoint already carries it.
	Region string
	TLS    TLSOptions
	SSH    SSHTunnel
	// Proxy is the outer hop, and carries the bastion as well as the database — see ProxyOptions.
	Proxy        ProxyOptions
	SavePassword bool
}

// ForAgent is this profile as an AI agent is given it: **always read-only**, whatever the reader
// chose for themselves. An agent has nobody in front of a dialog to answer "are you sure", so the
// question Environment.NeedsConfirmation asks would have nobody to ask — and a refusal is the only
// answer left that does not default to allowing.
func (p ConnectionProfile) ForAgent() ConnectionProfile {
	p.Access = AccessReadOnly
	return p
}

func (p ConnectionProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ValidationError{Field: "name", Message: "is required"}
	}
	if !p.Engine.Valid() {
		return ValidationError{Field: "engine", Message: "is not supported"}
	}
	// A file engine dials nothing, so a host and a port are not merely optional here: there is no
	// server for either of them to name.
	if !p.Engine.File() {
		if strings.TrimSpace(p.Host) == "" {
			return ValidationError{Field: "host", Message: "is required"}
		}
		if p.Port < 1 || p.Port > 65535 {
			return ValidationError{Field: "port", Message: "must be between 1 and 65535"}
		}
	}
	// A MongoDB server started without access control and a Redis server with no ACL take no
	// credentials at all, so a user name is asked for only by an engine that authenticates every
	// connection.
	if p.Engine.NeedsUser() && strings.TrimSpace(p.User) == "" {
		return ValidationError{Field: "user", Message: "is required"}
	}
	if !p.Environment.Valid() {
		return ValidationError{Field: "environment", Message: "is not a known environment"}
	}
	if !p.Colour.Valid() {
		return ValidationError{Field: "colour", Message: "is not a known colour"}
	}
	if !p.Access.Valid() {
		return ValidationError{Field: "access", Message: "is not a known access mode"}
	}
	// Three engines cannot be connected to without one: a PostgreSQL session is bound to its
	// database for life, an Oracle listener is asked for a service by name, and a SQLite database
	// *is* the file. See Engine.NeedsDatabase.
	if p.Engine.NeedsDatabase() && strings.TrimSpace(p.Database) == "" {
		message := "is required for this engine"
		if p.Engine.File() {
			message = "is the database file, and is required"
		}
		return ValidationError{Field: "database", Message: message}
	}
	// A Redis database is not named but numbered — one of a fixed set the server was started with —
	// so anything that is not a number is not a database this server has.
	if p.Engine == EngineRedis && strings.TrimSpace(p.Database) != "" {
		if index, err := strconv.Atoi(strings.TrimSpace(p.Database)); err != nil || index < 0 {
			return ValidationError{Field: "database", Message: "is a number on Redis, counting from 0"}
		}
	}
	if !p.Auth.Valid() {
		return ValidationError{Field: "auth", Message: "is not a known authentication method"}
	}
	if err := p.TLS.Validate(); err != nil {
		return err
	}
	if err := p.Proxy.Validate(); err != nil {
		return err
	}
	return p.SSH.Validate()
}

// Target is where this connection goes, as the status bar says it. A file engine is the path and
// nothing else: there is no address to put after an `@`.
func (p ConnectionProfile) Target() string {
	if p.Engine.File() {
		return p.Database
	}

	database := p.Database
	if database == "" {
		database = "(no database)"
	}
	return database + "@" + p.Address()
}

func (p ConnectionProfile) Address() string {
	return net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
}
