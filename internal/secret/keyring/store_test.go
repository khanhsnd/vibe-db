package keyring

import (
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestRoundTrip(t *testing.T) {
	store := New()
	const profileID = "vibe-db-test-profile"
	const password = "p@ss:word/?#[]"

	if err := store.Set(profileID, password); err != nil {
		t.Skipf("no usable credential store on this machine: %v", err)
	}
	t.Cleanup(func() { store.Delete(profileID) })

	got, err := store.Get(profileID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != password {
		t.Errorf("Get() = %q, want %q", got, password)
	}

	if err := store.Delete(profileID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := store.Get(profileID); !errors.Is(err, domain.ErrPasswordNotFound) {
		t.Errorf("Get() after Delete error = %v, want ErrPasswordNotFound", err)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	store := New()

	if err := store.Delete("vibe-db-test-absent"); err != nil {
		t.Errorf("Delete() on absent key = %v, want nil", err)
	}
}
