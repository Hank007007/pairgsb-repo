package grant

import "time"

// WithinSkew reports whether a credential expiring at expiresAt is still
// usable at now. A zero expiry means the credential does not expire.
func WithinSkew(kind Kind, now, expiresAt time.Time) bool {
	if expiresAt.IsZero() {
		return true
	}
	skew := 5 * time.Minute
	return now.Before(expiresAt.Add(skew))
}
