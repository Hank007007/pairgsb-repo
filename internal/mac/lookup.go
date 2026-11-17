package mac

// Lookup resolves the signing key belonging to an issuer.
type Lookup interface {
	// Lookup returns the key for issuer and whether one is known.
	Lookup(issuer string) (Key, bool)
}

// Keyring is a simple in-memory Lookup.
type Keyring map[string]Key

// Lookup implements Lookup.
func (k Keyring) Lookup(issuer string) (Key, bool) {
	key, ok := k[issuer]
	return key, ok
}

// Clone returns an independent copy of the keyring.
func (k Keyring) Clone() Keyring {
	out := make(Keyring, len(k))
	for issuer, key := range k {
		out[issuer] = key
	}
	return out
}
