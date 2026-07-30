package domain

import (
	"net"
	"strconv"
	"strings"
)

type Engine string

const (
	EngineMySQL    Engine = "mysql"
	EnginePostgres Engine = "postgres"
)

const (
	DefaultMySQLPort    = 3306
	DefaultPostgresPort = 5432
)

// DefaultPort is the port an engine listens on when the profile names none.
func (e Engine) DefaultPort() int {
	switch e {
	case EnginePostgres:
		return DefaultPostgresPort
	default:
		return DefaultMySQLPort
	}
}

func (e Engine) Valid() bool {
	switch e {
	case EngineMySQL, EnginePostgres:
		return true
	default:
		return false
	}
}

type TLSMode string

const (
	TLSDisabled  TLSMode = "disabled"
	TLSPreferred TLSMode = "preferred"
	TLSRequired  TLSMode = "required"
	TLSVerifyCA  TLSMode = "verify-ca"
)

func (m TLSMode) Valid() bool {
	switch m {
	case TLSDisabled, TLSPreferred, TLSRequired, TLSVerifyCA:
		return true
	default:
		return false
	}
}

type ConnectionProfile struct {
	ID           string
	Name         string
	Engine       Engine
	Host         string
	Port         int
	User         string
	Database     string
	TLSMode      TLSMode
	SavePassword bool
}

func (p ConnectionProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ValidationError{Field: "name", Message: "is required"}
	}
	if strings.TrimSpace(p.Host) == "" {
		return ValidationError{Field: "host", Message: "is required"}
	}
	if p.Port < 1 || p.Port > 65535 {
		return ValidationError{Field: "port", Message: "must be between 1 and 65535"}
	}
	if strings.TrimSpace(p.User) == "" {
		return ValidationError{Field: "user", Message: "is required"}
	}
	if !p.Engine.Valid() {
		return ValidationError{Field: "engine", Message: "is not supported"}
	}
	// A PostgreSQL session is bound to one database for its whole life and cannot cross to
	// another, so a profile that names none has nothing to connect to.
	if p.Engine == EnginePostgres && strings.TrimSpace(p.Database) == "" {
		return ValidationError{Field: "database", Message: "is required for PostgreSQL"}
	}
	if !p.TLSMode.Valid() {
		return ValidationError{Field: "tlsMode", Message: "is not a known TLS mode"}
	}
	return nil
}

func (p ConnectionProfile) Target() string {
	database := p.Database
	if database == "" {
		database = "(no database)"
	}
	return database + "@" + p.Address()
}

func (p ConnectionProfile) Address() string {
	return net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
}
