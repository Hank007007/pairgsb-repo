package chain

import (
	"errors"
	"fmt"
	"time"

	"capbaton/internal/mac"
	"capbaton/internal/token"
)

// MaxChainLength is the longest delegation chain a verifier will accept.
const MaxChainLength = 6

// Errors returned by chain validation and attenuation. Callers can test
// for them with errors.Is.
var (
	ErrEmptyChain      = errors.New("chain: empty chain")
	ErrChainTooLong    = errors.New("chain: chain exceeds the maximum length")
	ErrBrokenLink      = errors.New("chain: link does not belong to this chain")
	ErrUnknownIssuer   = errors.New("chain: no key for link issuer")
	ErrExpired         = errors.New("chain: link has expired")
	ErrMalformedExpiry = errors.New("chain: malformed expires caveat")
	ErrRevoked         = errors.New("chain: a link in the chain has been revoked")

	ErrIssuerNotDelegate  = errors.New("chain: issuer is not the delegate of the parent link")
	ErrScopeNotSubset     = errors.New("chain: requested scopes exceed the parent link")
	ErrScopeNotNarrowed   = errors.New("chain: requested scopes do not narrow the parent link")
	ErrParentCaveatMissed = errors.New("chain: a parent caveat is missing from the child link")
	ErrInvalidClaim       = errors.New("chain: incomplete claim")
)

// Options configures chain verification.
type Options struct {
	// Keys resolves the signing key for a link's issuer. It is required.
	Keys mac.Lookup
	// Revoked lists revoked token identifiers. It may be nil.
	Revoked RevocationList
	// Now supplies the current time. It may be nil, in which case the
	// system clock is used.
	Now func() time.Time
}

// RevocationList reports whether any link of a chain has been revoked.
type RevocationList interface {
	IsRevoked(links []token.Link) bool
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// VerifyChain checks that links forms one intact delegation chain from a
// root link to a leaf link.
func VerifyChain(links []token.Link, opts Options) error {
	if len(links) == 0 {
		return ErrEmptyChain
	}
	if len(links) > MaxChainLength {
		return fmt.Errorf("%w: got %d links, limit is %d", ErrChainTooLong, len(links), MaxChainLength)
	}
	if opts.Keys == nil {
		return ErrUnknownIssuer
	}
	if links[0].Payload.PrevMAC != "" {
		return fmt.Errorf("%w: root link carries a parent MAC", ErrBrokenLink)
	}

	for _, link := range links {
		key, ok := opts.Keys.Lookup(link.Payload.Issuer)
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownIssuer, link.Payload.Issuer)
		}
		if err := mac.Verify(key, link); err != nil {
			return fmt.Errorf("%w: token %q", ErrBrokenLink, link.Payload.TokenID)
		}
	}

	// §3: every link after the root must record the MAC of the link
	// immediately preceding it, so a link issued into another chain
	// cannot be spliced into this one.
	for i := 1; i < len(links); i++ {
		if links[i].Payload.PrevMAC != links[i-1].MAC {
			return fmt.Errorf("%w: token %q does not follow token %q",
				ErrBrokenLink, links[i].Payload.TokenID, links[i-1].Payload.TokenID)
		}
	}

	// §5: the effective caveat set is the union of every link's
	// caveats, so expiry contributed by an intermediate link still
	// binds the whole chain.
	now := opts.now()
	for _, link := range links {
		expires, present, err := expiryOf(link.Payload)
		if err != nil {
			return err
		}
		if present && !now.Before(expires) {
			return fmt.Errorf("%w: token %q", ErrExpired, link.Payload.TokenID)
		}
	}

	if opts.Revoked != nil && opts.Revoked.IsRevoked(links) {
		return fmt.Errorf("%w", ErrRevoked)
	}
	return nil
}

// expiryOf returns the effective expiry of a payload. It reports
// present=false when no expires caveat is attached.
func expiryOf(p token.Payload) (expires time.Time, present bool, err error) {
	for _, c := range p.Caveats {
		if c.Name != CaveatExpires {
			continue
		}
		t, perr := time.Parse(time.RFC3339, c.Value)
		if perr != nil {
			return time.Time{}, false, fmt.Errorf("%w: token %q: %v", ErrMalformedExpiry, p.TokenID, perr)
		}
		if !present || t.Before(expires) {
			expires = t.UTC()
		}
		present = true
	}
	return expires, present, nil
}
