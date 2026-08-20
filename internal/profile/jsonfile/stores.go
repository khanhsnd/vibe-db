package jsonfile

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	storeFileName = "object-stores.json"
	storeVersion  = 1
)

// storeRecord is an object-store connection as it sits on disk. It is a **file of its own** rather
// than a second array in `connections.json`: a reader who has never opened a bucket keeps the file
// they already had, and a reader who has never opened a database keeps this one — and neither file
// can be half read by a version of the app that has not heard of the other.
//
// **No secret is here**, exactly as in `record`: a secret access key lives in the credential store
// under the profile's id, a bastion's pass phrase under `<id>:ssh`, a proxy's password under
// `<id>:proxy`. The access key **id** is here, because it identifies a key and cannot sign anything.
type storeRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Environment string `json:"environment,omitempty"`
	Colour      string `json:"colour,omitempty"`
	Access      string `json:"access,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Region      string `json:"region,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
	PathStyle   bool   `json:"pathStyle,omitempty"`
	Auth        string `json:"auth,omitempty"`
	AccessKeyID string `json:"accessKeyId,omitempty"`

	TLSMode       string `json:"tlsMode,omitempty"`
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

	SaveSecret bool `json:"saveSecret"`
}

type storeDocument struct {
	Version int           `json:"version"`
	Stores  []storeRecord `json:"stores"`
}

// StoreRepo is the object-store half, and it is the same shape `Repo` is — one `jsonstore.File`,
// read whole and written whole, so neither adapter spells atomic writing out.
type StoreRepo struct {
	file *jsonstore.File[storeDocument]
}

func NewStores(path string) *StoreRepo {
	return &StoreRepo{file: jsonstore.New[storeDocument](path)}
}

func DefaultStorePath() (string, error) {
	return jsonstore.Path(storeFileName)
}

func (r *StoreRepo) List() ([]domain.StoreProfile, error) {
	doc, err := r.file.Read()
	if err != nil {
		return nil, err
	}

	profiles := make([]domain.StoreProfile, 0, len(doc.Stores))
	for _, rec := range doc.Stores {
		profiles = append(profiles, toStoreDomain(rec))
	}
	return profiles, nil
}

func (r *StoreRepo) Get(id string) (domain.StoreProfile, error) {
	doc, err := r.file.Read()
	if err != nil {
		return domain.StoreProfile{}, err
	}

	for _, rec := range doc.Stores {
		if rec.ID == id {
			return toStoreDomain(rec), nil
		}
	}
	return domain.StoreProfile{}, domain.ErrProfileNotFound
}

func (r *StoreRepo) Save(profile domain.StoreProfile) error {
	return r.file.Update(func(doc *storeDocument) error {
		doc.Version = storeVersion
		updated := fromStoreDomain(profile)

		for at, rec := range doc.Stores {
			if rec.ID == profile.ID {
				doc.Stores[at] = updated
				return nil
			}
		}

		doc.Stores = append(doc.Stores, updated)
		return nil
	})
}

func (r *StoreRepo) Delete(id string) error {
	return r.file.Update(func(doc *storeDocument) error {
		kept := make([]storeRecord, 0, len(doc.Stores))
		for _, rec := range doc.Stores {
			if rec.ID != id {
				kept = append(kept, rec)
			}
		}
		if len(kept) == len(doc.Stores) {
			return domain.ErrProfileNotFound
		}

		doc.Version = storeVersion
		doc.Stores = kept
		return nil
	})
}

// toStoreDomain reads a record back, filling in the two fields a document written before they
// existed cannot carry: a profile that names no authentication uses a key pair, which is what every
// store profile did before there was anything else to name, and one that names no TLS mode verifies
// the certificate — the strictest of the five, because an object store is reached over https and a
// profile that quietly stopped checking would be the wrong thing to default to.
func toStoreDomain(rec storeRecord) domain.StoreProfile {
	auth := domain.StoreAuth(rec.Auth)
	if auth == "" {
		auth = domain.StoreKey
	}

	mode := domain.TLSMode(rec.TLSMode)
	if mode == "" {
		mode = domain.TLSVerifyFull
	}

	return domain.StoreProfile{
		ID:          rec.ID,
		Name:        rec.Name,
		Provider:    domain.StoreProvider(rec.Provider),
		Environment: domain.Environment(rec.Environment),
		Colour:      domain.Colour(rec.Colour),
		Access:      domain.Access(rec.Access),
		Endpoint:    rec.Endpoint,
		Region:      rec.Region,
		Bucket:      rec.Bucket,
		PathStyle:   rec.PathStyle,
		Auth:        auth,
		AccessKeyID: rec.AccessKeyID,
		TLS: domain.TLSOptions{
			Mode:       mode,
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
		SaveSecret: rec.SaveSecret,
	}
}

func fromStoreDomain(profile domain.StoreProfile) storeRecord {
	return storeRecord{
		ID:          profile.ID,
		Name:        profile.Name,
		Provider:    string(profile.Provider),
		Environment: string(profile.Environment),
		Colour:      string(profile.Colour),
		Access:      string(profile.Access),
		Endpoint:    profile.Endpoint,
		Region:      profile.Region,
		Bucket:      profile.Bucket,
		PathStyle:   profile.PathStyle,
		Auth:        string(profile.Auth),
		AccessKeyID: profile.AccessKeyID,

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

		SaveSecret: profile.SaveSecret,
	}
}
