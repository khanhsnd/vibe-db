package keyring

import (
	"errors"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/zalando/go-keyring"
)

const serviceName = "vibe-db"

type Store struct{}

func New() *Store {
	return &Store{}
}

func (*Store) Set(profileID, password string) error {
	return keyring.Set(serviceName, profileID, password)
}

func (*Store) Get(profileID string) (string, error) {
	password, err := keyring.Get(serviceName, profileID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", domain.ErrPasswordNotFound
	}
	if err != nil {
		return "", err
	}
	return password, nil
}

func (*Store) Delete(profileID string) error {
	err := keyring.Delete(serviceName, profileID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
