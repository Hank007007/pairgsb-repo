package keyring

import "fmt"

// SelectForWrite returns the key version that new data keys must be wrapped
// with.
func (r *Keyring) SelectForWrite() (Key, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.keys) == 0 {
		return Key{}, ErrEmpty
	}

	var (
		best  Key
		found bool
	)
	for _, k := range r.keys {
		if k.Status == StatusActive && (!found || k.Version > best.Version) {
			best = k
			found = true
		}
	}
	if !found {
		return Key{}, ErrNoWriter
	}
	return copyKey(best), nil
}

// SelectForRead returns the key version identified by version.
func (r *Keyring) SelectForRead(version uint32) (Key, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, k := range r.keys {
		if k.Version == version {
			return copyKey(k), nil
		}
	}
	return Key{}, fmt.Errorf("%w: %d", ErrUnknownVersion, version)
}
