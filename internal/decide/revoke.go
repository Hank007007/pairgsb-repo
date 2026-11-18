package decide

import (
	"errors"
	"fmt"

	"permcache/internal/cache"
)

// ErrInvalidRevocation reports a revocation with an empty field.
var ErrInvalidRevocation = errors.New("decide: invalid revocation")

// Revocation names a set of cached decisions that must be dropped.
//
// Each field is either a literal value or "*", which stands for every value in
// that dimension.
type Revocation struct {
	Tenant   string
	Subject  string
	Action   string
	Resource string
}

// Validate checks that every field is populated.
func (r Revocation) Validate() error {
	switch {
	case r.Tenant == "":
		return fmt.Errorf("%w: tenant", ErrInvalidRevocation)
	case r.Subject == "":
		return fmt.Errorf("%w: subject", ErrInvalidRevocation)
	case r.Action == "":
		return fmt.Errorf("%w: action", ErrInvalidRevocation)
	case r.Resource == "":
		return fmt.Errorf("%w: resource", ErrInvalidRevocation)
	}
	return nil
}

// covered reports whether the cached entry falls inside the revocation scope.
func (r Revocation) covered(e cache.Entry) bool {
	return e.Tenant == r.Tenant &&
		e.Subject == r.Subject &&
		e.Action == r.Action &&
		e.Resource == r.Resource
}

// ApplyRevocation drops the cached decisions covered by r and returns how many
// entries were removed.
func (e *Engine) ApplyRevocation(r Revocation) (int, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	return e.store.RemoveWhere(func(entry cache.Entry) bool { return !r.covered(entry) }), nil
}
