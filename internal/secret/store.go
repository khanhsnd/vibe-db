package secret

type Store interface {
	Set(profileID, password string) error
	Get(profileID string) (string, error)
	Delete(profileID string) error
}
