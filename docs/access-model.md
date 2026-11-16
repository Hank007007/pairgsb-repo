# Access model

This document is the normative specification of the `scopewall` authorization model. It states
what a correct decision point must do. It deliberately does not describe how the engine is
structured internally.

A decision is computed from four inputs: the **subject**, the **resource path**, the **action**,
and the **policy document** (including its **delegations**). The output is either `allow` or
`deny`, together with the reason that produced it.

## §1 Subject

1. A subject is the triple of the calling workload's service account, the role chain issued to
   it by the platform, and the version of that role chain.
2. An empty service account identifies no principal. Requests whose subject has an empty service
   account, an empty role chain, or an empty role version are **denied**, regardless of what the
   policy contains.
3. A subject is identified by its service account; the role chain is the set of permissions it
   carries. Two requests with the same service account but different role versions are different
   subjects for every purpose in this document.

## §2 Resource paths and patterns

1. Resource paths are hierarchical. They are written as a leading slash followed by segments
   separated by `/`, for example `/orgs/acme/projects/42`. Consecutive slashes, a trailing slash,
   and empty segments carry no meaning and are normalized away before matching.
2. A pattern is a resource path in which a segment may be the wildcard `*` or `**`.
   * `*` matches **exactly one** non-empty segment.
   * `**` matches **zero or more** remaining segments, and is only meaningful as the final
     segment of a pattern.
   * Any other segment matches itself literally.
3. A pattern matches a resource path only if the pattern consumes the path segment by segment.
   Segment boundaries are significant: a literal segment matches only a whole segment, never a
   prefix of one. In particular a rule for `/orgs/acme` does **not** apply to `/orgs/acmeevil/x`,
   and a rule for `/orgs/acme` does not apply to `/orgs/acme/projects` unless the rule ends in
   `**`.
4. Matching is case sensitive.
5. Example matches:

   | Pattern | Path | Matches |
   | --- | --- | --- |
   | `/orgs/acme/**` | `/orgs/acme` | yes |
   | `/orgs/acme/**` | `/orgs/acme/projects/42` | yes |
   | `/orgs/acme` | `/orgs/acmeevil/x` | no |
   | `/orgs/*/projects` | `/orgs/acme/projects` | yes |
   | `/orgs/*/projects` | `/orgs/a/b/projects` | no |
   | `/**` | `/anything/at/all` | yes |

## §3 Rules and evaluation

1. A rule binds zero or more **role names**, a **resource pattern**, and an **effect** of either
   `allow` or `deny`. An action list narrows the rule to those actions; an empty action list
   means the rule applies to every action, and the action `*` in policy documents is a synonym
   for every action.
2. A rule is a candidate for a request when the subject's role chain contains one of its role
   names, its resource pattern matches the resource path, and its actions cover the requested
   action.
3. **Deny precedence.** If any candidate rule has effect `deny`, the decision is `deny`. Only when
   no candidate rule denies may an `allow` candidate produce `allow`.
4. **Order independence.** The order of rules in the document must not influence the decision.
   Reordering the rules of a document, or reordering role names inside a rule, leaves every
   decision unchanged. In particular, an allow rule that appears earlier in the file than a deny
   rule must not shield the request from the deny.
5. If no rule is a candidate for the request, the decision is `deny` (see §6).
6. The decision reason should name the rule that determined the outcome. When several deny rules
   apply, any one of them may be cited.

## §4 Delegation

A delegation is a directed grant from a delegating role to a receiving role, scoped to a
resource pattern and an action set. Its purpose is to let one team lend part of its own
authority to another team without granting anything the lending team does not itself hold.

1. Delegations form a directed graph over role names. A path
   `A -> B -> C` means that `B` may use what `A` delegates to it, and `C` may use what `B`
   delegates to it, subject to the rules below.
2. **Intersection.** The effective permissions obtained through a delegation from `A` to `B`
   are the **intersection** of two sets:
   * the permissions `A` itself holds from the allow rules of the policy, and
   * the scope written on the delegating edge (pattern and actions).
   The receiving role never obtains a permission that the delegating role does not hold. A
   delegation cannot widen authority, only transfer a subset of it.
3. **Depth.** A delegation chain may be at most **three hops** long. A chain of four or more
   hops is rejected in its entirety; it must not contribute a partial grant. A chain of exactly
   three hops is valid.
4. **Cycles.** A delegation chain that revisits a role already on the chain is cyclic and is
   rejected in its entirety. Cycle detection considers the whole chain, not just the immediate
   predecessor, and not just the current recursion frame.
5. **Fail closed.** If any edge of a chain fails to satisfy §4.2, the contribution of that chain
   is empty. A rejected chain grants nothing; it does not fall back to a shorter prefix of itself
   with wider scope.
6. Delegation only ever contributes `allow` authority. It cannot produce a `deny`, and it cannot
   neutralize a `deny` rule that applies directly to the subject.

## §5 Decision cache

1. Decisions may be memoized. A cached entry is keyed by everything that can change the decision:
   the service account, the **role version**, the resource path, and the action.
2. The role version participates in the key. When a subject's role version changes, its
   previously cached decisions must not be reused, even if the TTL has not expired. A
   downgraded role takes effect on the next request.
3. Entries expire after a configurable TTL, measured from insertion. The cache is bounded; when
   full, the least recently used entry is evicted. Lookups and insertions both count as use.
4. The cache is a performance device only. Serving an entry must never change which decision a
   fresh evaluation would produce.

## §6 Failure handling

The engine fails closed in every situation it cannot decide affirmatively:

1. If the policy document cannot be loaded or parsed, every request is **denied**. A failure to
   load a policy is never treated as an empty policy, and an empty or unreachable policy is never
   treated as permission to proceed.
2. If the subject is empty as defined in §1.2, the request is **denied** before evaluation.
3. If no rule and no delegation is a candidate for the request, the request is **denied**.
4. An unrecognized action, a malformed resource path, or a malformed pattern is **denied**.
5. These denials hold even when the policy contains a rule whose pattern would otherwise cover
   everything, such as `/**` or `*` against a root-level path.

## §7 Decision surface

1. A decision carries the effect, a human-readable reason, and the source that produced it:
   `policy` for a fresh evaluation, `cache` for a memoized one.
2. The HTTP middleware denies with `403 Forbidden` and a JSON body describing the decision. An
   allowed request is forwarded to the wrapped handler unchanged.
3. A denied request never reaches the wrapped handler.
