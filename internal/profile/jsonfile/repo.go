package jsonfile

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	fileName = "connections.json"
	version  = 1
)

type record struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Engine       string `json:"engine"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Database     string `json:"database"`
	TLSMode      string `json:"tlsMode"`
	SavePassword bool   `json:"savePassword"`
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

func toDomain(rec record) domain.ConnectionProfile {
	return domain.ConnectionProfile{
		ID:           rec.ID,
		Name:         rec.Name,
		Engine:       domain.Engine(rec.Engine),
		Host:         rec.Host,
		Port:         rec.Port,
		User:         rec.User,
		Database:     rec.Database,
		TLSMode:      domain.TLSMode(rec.TLSMode),
		SavePassword: rec.SavePassword,
	}
}

func fromDomain(profile domain.ConnectionProfile) record {
	return record{
		ID:           profile.ID,
		Name:         profile.Name,
		Engine:       string(profile.Engine),
		Host:         profile.Host,
		Port:         profile.Port,
		User:         profile.User,
		Database:     profile.Database,
		TLSMode:      string(profile.TLSMode),
		SavePassword: profile.SavePassword,
	}
}
