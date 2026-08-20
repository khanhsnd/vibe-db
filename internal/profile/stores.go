package profile

import "github.com/khanhsnd/vibe-db/internal/domain"

// StoreRepository is where the object-store connections are kept, and it is `Repository`'s sibling
// for the same reason `domain.StoreProfile` is `domain.ConnectionProfile`'s: the same four questions
// about a different kind of connection.
//
// It is a **second port and not a second field** on the first. A store profile shares five things
// with a database one — a name, an environment, a colour, an access mode, the hops that carry it —
// and differs in every other field, so one repository answering with both would be one record with
// two thirds of its fields empty in either direction, and one `Save` that could write a profile no
// engine could open.
type StoreRepository interface {
	List() ([]domain.StoreProfile, error)
	Get(id string) (domain.StoreProfile, error)
	Save(profile domain.StoreProfile) error
	Delete(id string) error
}
