package profile

import "github.com/khanhsnd/vibe-db/internal/domain"

type Repository interface {
	List() ([]domain.ConnectionProfile, error)
	Get(id string) (domain.ConnectionProfile, error)
	Save(profile domain.ConnectionProfile) error
	Delete(id string) error
}
