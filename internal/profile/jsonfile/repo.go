package jsonfile

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	fileName = "connections.json"
	version  = 1
)

// record is a profile as it sits on disk. Every field added since version 1 is `omitempty`, so a
// reader who has never opened the SSH or TLS tabs keeps the file they already had — and a field a
// document written before it existed does not carry reads back as its zero value, which `toDomain`
// then fills in.
//
// **No secret is here.** A password, a bastion's pass phrase and a cloud token all live in the
// credential store; this file is copied into backups and read by anything that can read the
// reader's home directory.
type record struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Engine      string `json:"engine"`
	Environment string `json:"environment,omitempty"`
	Colour      string `json:"colour,omitempty"`
	Access      string `json:"access,omitempty"`
	MCP         bool   `json:"mcp,omitempty"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Database    string `json:"database"`
	Auth        string `json:"auth,omitempty"`
	Region      string `json:"region,omitempty"`

	TLSMode       string `json:"tlsMode"`
	TLSRootCert   string `json:"tlsRootCert,omitempty"`
	TLSClientCert string `json:"tlsClientCert,omitempty"`
	TLSClientKey  string `json:"tlsClientKey,omitempty"`
	TLSServerName string `json:"tlsServerName,omitempty"`

	SSHEnabled       bool   `json:"sshEnabled,omitempty"`
	SSHHost          string `json:"sshHost,omitempty"`
	SSHPort          int    `json:"sshPort,omitempty"`
	SSHUser          string `json:"sshUser,omitempty"`
	SSHAuth          string `json:"sshAuth,omitempty"`
	SSHKeyPath       string `json:"sshKeyPath,omitempty"`
	SSHKnownHosts    string `json:"sshKnownHosts,omitempty"`
	SSHStrictHostKey bool   `json:"sshStrictHostKey,omitempty"`

	ProxyEnabled bool   `json:"proxyEnabled,omitempty"`
	ProxyKind    string `json:"proxyKind,omitempty"`
	ProxyHost    string `json:"proxyHost,omitempty"`
	ProxyPort    int    `json:"proxyPort,omitempty"`
	ProxyUser    string `json:"proxyUser,omitempty"`

	SavePassword bool `json:"savePassword"`
}

type document struct {
	Version  int      `json:"version"`
	Profiles []record `json:"profiles"`
}

type Repo struct {
	file *jsonstore.File[document]
}

func New(path string) *Repo {
	return &Repo{file: jsonstore.New[document](path)}
}

func DefaultPath() (string, error) {
	return jsonstore.Path(fileName)
}

func (r *Repo) List() ([]domain.ConnectionProfile, error) {
	doc, err := r.file.Read()
	if err != nil {
		return nil, err
	}

	profiles := make([]domain.ConnectionProfile, 0, len(doc.Profiles))
	for _, rec := range doc.Profiles {
		profiles = append(profiles, toDomain(rec))
	}
	return profiles, nil
}

func (r *Repo) Get(id string) (domain.ConnectionProfile, error) {
	doc, err := r.file.Read()
	if err != nil {
		return domain.ConnectionProfile{}, err
	}

	for _, rec := range doc.Profiles {
		if rec.ID == id {
			return toDomain(rec), nil
		}
	}
	return domain.ConnectionProfile{}, domain.ErrProfileNotFound
}

func (r *Repo) Save(profile domain.ConnectionProfile) error {
	return r.file.Update(func(doc *document) error {
		doc.Version = version
		updated := fromDomain(profile)

		for i, rec := range doc.Profiles {
			if rec.ID == profile.ID {
				doc.Profiles[i] = updated
				return nil
			}
		}

		doc.Profiles = append(doc.Profiles, updated)
		return nil
	})
}

func (r *Repo) Delete(id string) error {
	return r.file.Update(func(doc *document) error {
		kept := make([]record, 0, len(doc.Profiles))
		for _, rec := range doc.Profiles {
			if rec.ID != id {
				kept = append(kept, rec)
			}
		}
		if len(kept) == len(doc.Profiles) {
			return domain.ErrProfileNotFound
		}

		doc.Version = version
		doc.Profiles = kept
		return nil
	})
}

// toDomain reads a record back, filling in the one field a document written before it existed
// cannot carry: a profile that names no authentication method uses a password, which is what
// every profile did before there was anything else to name.
func toDomain(rec record) domain.ConnectionProfile {
	auth := domain.AuthMethod(rec.Auth)
	if auth == "" {
		auth = domain.AuthPassword
	}

	return domain.ConnectionProfile{
		ID:          rec.ID,
		Name:        rec.Name,
		Engine:      domain.Engine(rec.Engine),
		Environment: domain.Environment(rec.Environment),
		Colour:      domain.Colour(rec.Colour),
		Access:      domain.Access(rec.Access),
		MCP:         rec.MCP,
		Host:        rec.Host,
		Port:        rec.Port,
		User:        rec.User,
		Database:    rec.Database,
		Auth:        auth,
		Region:      rec.Region,
		TLS: domain.TLSOptions{
			Mode:       domain.TLSMode(rec.TLSMode),
			RootCert:   rec.TLSRootCert,
			ClientCert: rec.TLSClientCert,
			ClientKey:  rec.TLSClientKey,
			ServerName: rec.TLSServerName,
		},
		SSH: domain.SSHTunnel{
			Enabled:       rec.SSHEnabled,
			Host:          rec.SSHHost,
			Port:          rec.SSHPort,
			User:          rec.SSHUser,
			Auth:          domain.SSHAuth(rec.SSHAuth),
			KeyPath:       rec.SSHKeyPath,
			KnownHosts:    rec.SSHKnownHosts,
			StrictHostKey: rec.SSHStrictHostKey,
		},
		Proxy: domain.ProxyOptions{
			Enabled: rec.ProxyEnabled,
			Kind:    domain.ProxyKind(rec.ProxyKind),
			Host:    rec.ProxyHost,
			Port:    rec.ProxyPort,
			User:    rec.ProxyUser,
		},
		SavePassword: rec.SavePassword,
	}
}

func fromDomain(profile domain.ConnectionProfile) record {
	return record{
		ID:          profile.ID,
		Name:        profile.Name,
		Engine:      string(profile.Engine),
		Environment: string(profile.Environment),
		Colour:      string(profile.Colour),
		Access:      string(profile.Access),
		MCP:         profile.MCP,
		Host:        profile.Host,
		Port:        profile.Port,
		User:        profile.User,
		Database:    profile.Database,
		Auth:        string(profile.Auth),
		Region:      profile.Region,

		TLSMode:       string(profile.TLS.Mode),
		TLSRootCert:   profile.TLS.RootCert,
		TLSClientCert: profile.TLS.ClientCert,
		TLSClientKey:  profile.TLS.ClientKey,
		TLSServerName: profile.TLS.ServerName,

		SSHEnabled:       profile.SSH.Enabled,
		SSHHost:          profile.SSH.Host,
		SSHPort:          profile.SSH.Port,
		SSHUser:          profile.SSH.User,
		SSHAuth:          string(profile.SSH.Auth),
		SSHKeyPath:       profile.SSH.KeyPath,
		SSHKnownHosts:    profile.SSH.KnownHosts,
		SSHStrictHostKey: profile.SSH.StrictHostKey,

		ProxyEnabled: profile.Proxy.Enabled,
		ProxyKind:    string(profile.Proxy.Kind),
		ProxyHost:    profile.Proxy.Host,
		ProxyPort:    profile.Proxy.Port,
		ProxyUser:    profile.Proxy.User,

		SavePassword: profile.SavePassword,
	}
}
