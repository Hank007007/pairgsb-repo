package grant

import (
	"time"
)

// EffectiveGrant is the authorization a request actually carries after merging,
// narrowing and filtering its credentials.
type EffectiveGrant struct {
	Subject string
	Scopes  []string
	Roles   []Role
}

// Request groups the credentials presented with a single request. Now is the
// evaluation time; a zero value means time.Now().
type Request struct {
	Now   time.Time
	Creds []Credential
}

// Compute produces the effective grant for a request.
//
// Credentials are first checked against their expiry window. Delegated
// credentials are then resolved against the credentials held by the subject
// that issued them; unresolved or over-broad delegations are dropped whole.
// Surviving credentials contribute their scopes and roles, and the result is
// normalised and sorted.
func Compute(req Request) EffectiveGrant {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	type liveCredential struct {
		index int
		cred  Credential
	}

	live := make([]liveCredential, 0, len(req.Creds))
	for i, cred := range req.Creds {
		if cred.Kind == KindAnonymous {
			continue
		}
		if !WithinSkew(cred.Kind, now, cred.ExpiresAt) {
			continue
		}
		live = append(live, liveCredential{index: i, cred: cred})
	}

	// Index what each subject holds, so delegated credentials can be checked
	// against their delegator.
	held := make(map[string]Credential, len(live))
	effective := make([]Credential, 0, len(live))
	subject := ""
	hasSubject := false
	for _, entry := range live {
		cred := entry.cred
		if cred.Kind == KindDelegated {
			continue
		}
		held[cred.Subject] = mergeInto(held[cred.Subject], cred)
		effective = append(effective, cred)
		if !hasSubject {
			subject = cred.Subject
			hasSubject = true
		}
	}

	// Delegation chains are resolved in rounds: a delegated credential becomes
	// usable once its delegator has been resolved.
	pending := make([]liveCredential, 0, len(live))
	for _, entry := range live {
		if entry.cred.Kind == KindDelegated {
			pending = append(pending, entry)
		}
	}
	acceptedDelegated := make([]bool, len(req.Creds))
	for progress := true; progress && len(pending) > 0; {
		progress = false
		remaining := pending[:0]
		for _, entry := range pending {
			del := entry.cred
			delegator, ok := held[del.Delegator]
			if !ok {
				remaining = append(remaining, entry)
				continue
			}
			narrowed, ok := NarrowDelegated(del, delegator)
			if !ok {
				progress = true
				continue
			}
			held[del.Subject] = mergeInto(held[del.Subject], narrowed)
			effective = append(effective, narrowed)
			acceptedDelegated[entry.index] = true
			progress = true
		}
		pending = remaining
	}

	if !hasSubject {
		for _, entry := range live {
			if entry.cred.Kind == KindDelegated && acceptedDelegated[entry.index] {
				subject = entry.cred.Subject
				break
			}
		}
	}

	scopes := FilterSpecialScopes(Union(effective...), effective...)
	roles := MergeRoles(effective...)

	return EffectiveGrant{Subject: subject, Scopes: scopes, Roles: roles}
}

func mergeInto(dst, src Credential) Credential {
	if dst.Kind == "" {
		dst = Credential{
			Kind:      src.Kind,
			Issuer:    src.Issuer,
			Subject:   src.Subject,
			Delegator: src.Delegator,
			IssuedAt:  src.IssuedAt,
			ExpiresAt: src.ExpiresAt,
		}
	}
	dst.Scopes = append(dst.Scopes, src.Scopes...)
	dst.Roles = append(dst.Roles, src.Roles...)
	return dst
}
