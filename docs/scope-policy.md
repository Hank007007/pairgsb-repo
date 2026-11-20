# Scope Policy

Scope strings are the contract between the service asking for access and the
issuer granting it.  Because scope strings are typed by humans and assembled
by callers in different ways, the cache defines one canonical form and uses it
everywhere a scope set is compared, stored or reported.

## §1 Accepted inputs

§1.1 A scope request is either a single string containing scopes separated by
whitespace (spaces, tabs, newlines) or an iterable — list, tuple, set, generator
— whose elements are individual scope strings.

§1.2 Both spellings describe a set of scopes.  `"read write"`, `["read",
"write"]`, `("write", "read")` and `{"read", "write"}` all denote the same
request.

§1.3 A scope string is split on whitespace.  Empty segments carry no meaning
and are dropped.

## §2 Canonical form

§2.1 The canonical form of a scope request is the de-duplicated set of its
scope tokens, held in a deterministic order.

§2.2 Ordering is part of canonicalization, not part of the caller's
obligation: `"write read"` and `"read write"` canonicalize identically.  The
same holds for repeats — `"read write read"` denotes the same request as
`"read write"`.

§2.3 Canonicalization is total: feeding the canonical form back in yields the
same canonical form.

§2.4 Canonicalization never invents scopes and never drops one that was
present, beyond the de-duplication in §2.1.

## §3 Use of the canonical form

§3.1 Cache keys are built from the canonical form of the request (see
`token-cache.md` §2.1).

§3.2 Coverage checks compare canonical sets of granted and requested scopes
(`token-cache.md` §2.3).

§3.3 Audit records and error messages report the canonical form so that two
equivalent requests produce comparable log lines.

## §4 Interaction with the issuer

§4.1 The `scope` parameter sent to the token endpoint is the canonical form
joined by single spaces.

§4.2 If the issuer echoes a `scope` field, that echo is canonicalized before
it is compared against the request.  Issuers are not required to echo scopes
in the order they were asked for.
