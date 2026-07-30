package jsonfile

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

const (
	fileMode = 0o600
	dirMode  = 0o700
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
	path string
	mu   sync.Mutex
}

func New(path string) *Repo {
	return &Repo{path: path}
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vibe-db", "connections.json"), nil
}

func (r *Repo) List() ([]domain.ConnectionProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.read()
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
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.read()
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
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.read()
	if err != nil {
		return err
	}

	updated := fromDomain(profile)
	replaced := false
	for i, rec := range doc.Profiles {
		if rec.ID == profile.ID {
			doc.Profiles[i] = updated
			replaced = true
			break
		}
	}
	if !replaced {
		doc.Profiles = append(doc.Profiles, updated)
	}

	return r.write(doc)
}

func (r *Repo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.read()
	if err != nil {
		return err
	}

	kept := make([]record, 0, len(doc.Profiles))
	for _, rec := range doc.Profiles {
		if rec.ID != id {
			kept = append(kept, rec)
		}
	}
	if len(kept) == len(doc.Profiles) {
		return domain.ErrProfileNotFound
	}

	doc.Profiles = kept
	return r.write(doc)
}

func (r *Repo) read() (document, error) {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return document{Version: 1}, nil
	}
	if err != nil {
		return document{}, err
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, err
	}
	return doc, nil
}

func (r *Repo) write(doc document) error {
	doc.Version = 1

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".connections-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(fileMode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, r.path)
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
