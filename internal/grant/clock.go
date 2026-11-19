package grant

import "time"

// WithinSkew reports whether a credential expiring at expiresAt is still
// usable at now. A zero expiry means the credential does not expire.
func WithinSkew(kind Kind, now, expiresAt time.Time) bool {
	if expiresAt.IsZero() {
		return true
	}
	skew := skewFor(kind)
	return !now.After(expiresAt.Add(skew))
}

func skewFor(kind Kind) time.Duration {
	switch kind {
	case KindOIDCAccessToken:
		return 120 * time.Second
	case KindDelegated:
		return 60 * time.Second
	case KindServiceAccount:
		return 30 * time.Second
	default:
		return 0
	}
}
