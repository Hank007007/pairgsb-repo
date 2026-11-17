package chain

import (
	"fmt"
	"time"

	"capbaton/internal/mac"
	"capbaton/internal/scope"
	"capbaton/internal/token"
)

// CaveatExpires is the name of the caveat that bounds a link's lifetime.
// Its value is an RFC 3339 timestamp.
const CaveatExpires = "expires"

// Claim is the declaration a party makes when it issues a link.
type Claim struct {
	TokenID string
	Issuer  string
	Subject string
	Scopes  []string
	Caveats []token.Caveat
	// IssuedAt defaults to the current time when zero.
	IssuedAt time.Time
}

// IssueRoot creates a root link: a link that was not derived from any
// other and carries no parent MAC.
func IssueRoot(claim Claim, key mac.Key) (token.Link, error) {
	if claim.TokenID == "" || claim.Issuer == "" || claim.Subject == "" {
		return token.Link{}, fmt.Errorf("%w: token id, issuer and subject are required", ErrInvalidClaim)
	}
	if _, err := scope.ParseAll(claim.Scopes); err != nil {
		return token.Link{}, err
	}
	p := payloadFrom(claim)
	p.PrevMAC = ""
	return sign(p, key), nil
}

// Narrow derives a child link from parent. The child is issued by the
// parent's subject, may only ask for capabilities the parent already
// holds, and must carry every caveat of the parent.
func Narrow(parent token.Link, claim Claim, key mac.Key) (token.Link, error) {
	if claim.TokenID == "" || claim.Issuer == "" || claim.Subject == "" {
		return token.Link{}, fmt.Errorf("%w: token id, issuer and subject are required", ErrInvalidClaim)
	}
	if claim.Issuer != parent.Payload.Subject {
		return token.Link{}, fmt.Errorf("%w: %q tried to delegate for %q", ErrIssuerNotDelegate, claim.Issuer, parent.Payload.Subject)
	}

	parentScopes, err := scope.ParseAll(parent.Payload.Scopes)
	if err != nil {
		return token.Link{}, err
	}
	childScopes, err := scope.ParseAll(claim.Scopes)
	if err != nil {
		return token.Link{}, err
	}
	if !scope.IsSetSubset(childScopes, parentScopes) {
		return token.Link{}, fmt.Errorf("%w: %v", ErrScopeNotSubset, claim.Scopes)
	}
	if sameScopeSet(childScopes, parentScopes) {
		return token.Link{}, fmt.Errorf("%w: %v", ErrScopeNotNarrowed, claim.Scopes)
	}
	if err := ensureCaveatsInherited(parent.Payload.Caveats, claim.Caveats); err != nil {
		return token.Link{}, err
	}

	p := payloadFrom(claim)
	p.Caveats = append([]token.Caveat(nil), claim.Caveats...)
	p.PrevMAC = parent.MAC
	return sign(p, key), nil
}

// ensureCaveatsInherited enforces §5: a child must carry every caveat of
// its parent with the same value. It may add caveats of its own.
func ensureCaveatsInherited(parentCaveats, childCaveats []token.Caveat) error {
	for _, parentCaveat := range parentCaveats {
		matched := false
		for _, childCaveat := range childCaveats {
			if childCaveat.Name == parentCaveat.Name {
				if childCaveat.Value != parentCaveat.Value {
					return fmt.Errorf("%w: caveat %q changed from %q to %q",
						ErrParentCaveatMissed, parentCaveat.Name, parentCaveat.Value, childCaveat.Value)
				}
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: caveat %q is absent from the child link",
				ErrParentCaveatMissed, parentCaveat.Name)
		}
	}
	return nil
}

func payloadFrom(claim Claim) token.Payload {
	issuedAt := claim.IssuedAt.UTC()
	if claim.IssuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	return token.Payload{
		TokenID:  claim.TokenID,
		Issuer:   claim.Issuer,
		Subject:  claim.Subject,
		Scopes:   append([]string(nil), claim.Scopes...),
		Caveats:  append([]token.Caveat(nil), claim.Caveats...),
		IssuedAt: issuedAt,
	}
}

func sign(p token.Payload, key mac.Key) token.Link {
	return token.Link{Payload: p, MAC: mac.Compute(key, p)}
}

func sameScopeSet(a, b []scope.Scope) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s.String()]++
	}
	for _, s := range b {
		if seen[s.String()] == 0 {
			return false
		}
		seen[s.String()]--
	}
	return true
}
