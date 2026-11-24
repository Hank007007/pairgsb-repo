# Caches

topicwarden keeps two caches on the authorization path. Both exist only to
avoid repeated work; neither may ever change an answer. A cache hit must be
indistinguishable from recomputing the decision.

## 1. Expansion cache

`acl.CompiledCache` memoizes work on the subscribe path. Whatever it stores
must be usable for every caller that asks; a hit must give the same answer a
recomputation would. There are two sound shapes:

- store only context-free parse results (the filter's levels and wildcard
  positions), which are valid for every tenant, principal class, and version;
- or store decisions, in which case the key must bind the filter, the tenant,
  the principal class, and the rule-set version, and must be purged on reload.

| Input | Why it must be bound if decisions are stored |
|---|---|
| filter text | the thing being expanded |
| tenant | a tenant-scoped rule applies only to its own tenant |
| principal class | the same filter can flip after re-authentication |
| rule-set version | a reload can add, remove, or reorder rules |

Both shapes are correct; the choice is an implementation detail as long as a
hit always equals a recomputation.

## 2. Decision cache

`broker.Authorizer` caches whole decisions. A cached decision is only valid for
the exact context that produced it, so the key must bind every input that can
change the answer:

| Input | Why it must be in the key |
|---|---|
| client id | a decision belongs to one connection |
| principal class | re-authentication can change the class; the same filter may flip |
| action | `sub` and `pub` are separate rule sets |
| filter / topic | the thing being decided |
| tenant | a tenant-scoped rule applies only to its own tenant |
| rule-set version | a reload can add, remove, or reorder rules |

### 2.1 Reload

On reload the decision cache is invalidated wholesale. Entries computed under
an older version must never be served again, not even transiently. In
particular a cached allow from before a reload must not let a subscriber keep
receiving messages after a deny rule was added.

### 2.2 Re-authentication

When a connection's principal class changes, decisions taken under the old
class are dead. Either the class is part of the key (so the old entry is simply
unreachable) or the connection's entries are purged on re-authentication;
either way a stale class must never decide a request.

### 2.3 Subscribe entries

A subscribe decision is about a filter, so its key uses the filter — for shared
subscriptions, the inner filter. The expansion check described in
`matching.md` §2.2 is part of computing the decision; it is not a separate
cache.

## 3. Concurrency

Both caches are safe for concurrent use. Readers may run while a reload
installs a new version; a reader either observes the old complete rule set or
the new complete one, never a mixture.

## 4. Size

Caches are bounded only by the number of distinct keys observed in a process
lifetime; there is no eviction requirement. Operators reload rarely and key
cardinality is small relative to message volume.
