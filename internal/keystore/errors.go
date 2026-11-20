package keystore

import "errors"

// Sentinel errors returned by the keystore package. Callers should test
// errors with errors.Is.
var (
	ErrAuthentication       = errors.New("kekseal: authentication failed")
	ErrUnsupportedAlgorithm = errors.New("kekseal: unsupported record algorithm")
	ErrUnknownVersion       = errors.New("kekseal: unknown key version")
	ErrVersionRetired       = errors.New("kekseal: key version retired")
	ErrDuplicateVersion     = errors.New("kekseal: duplicate key version")
	ErrNoActiveVersion      = errors.New("kekseal: no active key version")
	ErrEmptyRing            = errors.New("kekseal: key ring has no versions")
	ErrInvalidRecord        = errors.New("kekseal: invalid record")
	ErrInvalidRing          = errors.New("kekseal: invalid key ring")
	ErrRotationFailed       = errors.New("kekseal: rotation failed")
)
