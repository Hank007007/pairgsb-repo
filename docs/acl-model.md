# ACL model

topicwarden authorizes three broker operations against a rule set:

| Broker operation | Entry point | Question answered |
|---|---|---|
| client subscribes to a filter | `AuthorizeSubscribe` | may this connection subscribe at all? |
| client publishes a message | `AuthorizePublish` | may this connection put this message on the bus? |
| delivery to a subscriber | `Deliver` | should this message reach this subscriber now? |

The rule set is a YAML document (see `testdata/rules.yaml`). It is reloaded at
runtime; a reload replaces the whole rule set atomically.

## 1. Rule shape

```yaml
rules:
  - id: sensors-read
    effect: allow         # allow | deny
    principal: sensor     # principal class, never a user name
    action: sub           # sub | pub
    filter: sensors/+/temp
    tenant: acme          # optional
```

- `id` is the stable identifier copied into audit records. It must be unique
  inside one document; loading rejects duplicates.
- `effect` is exactly `allow` or `deny`. Any other value is a load error: the
  document is rejected as a whole and the previously active rule set stays in
  effect.
- `principal` names a principal *class* (`sensor`, `service`, `operator`,
  `anonymous`, ...). Rules never name individual accounts. Every connection has
  exactly one principal class at a time; re-authentication may change it.
- `action` is `sub` or `pub`. A `sub` rule governs subscribe and delivery; a
  `pub` rule governs publish. A publish rule never authorizes a subscription and
  vice versa.
- `filter` is an MQTT topic filter as described in `matching.md`. It is
  matched against the full normalized topic, tenant prefix included; a rule
  intended for one tenant names that tenant both in `filter` and in `tenant`.
- `tenant` is optional. See §2.6.

## 2. Evaluation

Evaluation happens against a *decision context*:

```go
type Decision struct {
    PrincipalClass string   // current principal class of the connection
    Action         string   // "sub" or "pub"
    Topic          string   // normalized topic (subscribe: one expanded topic)
    Tenant         string   // tenant the topic belongs to, "" if none
    Version        uint64   // policy version the decision was made under
}
```

The context is derived from the connection, the operation, and the rule-set
version — never from a stale snapshot taken when the subscription was created.

### 2.1 Effect precedence

A rule whose filter matches is a *candidate*. Among all candidates:

1. If at least one candidate has `effect: deny`, the decision is **deny**.
2. Otherwise, if at least one candidate has `effect: allow`, the decision is
   **allow**.
3. If there are no candidates, the decision is **deny** (default deny).

Deny wins unconditionally. The order of rules in the YAML file has no effect on
the result: moving a deny rule before or after a matching allow rule must not
change any outcome.

The audit record for a deny names the rule that produced it; when several deny
rules match, the one reported is chosen deterministically (first by document
order) so that repeated evaluations of the same input log the same id.

### 2.2 Principal class

Only rules whose `principal` equals the connection's current principal class
are candidates. A connection that re-authenticates under a different class is
evaluated with the new class from that point on; an earlier decision is not
carried over.

### 2.3 Multi-level wildcard `#`

`#` matches zero or more levels. It is only legal as the last level of a filter.

| Filter | Topic | Match |
|---|---|---|
| `sensors/#` | `sensors` | yes (zero levels) |
| `sensors/#` | `sensors/a` | yes |
| `sensors/#` | `sensors/a/b` | yes |
| `sensors/#` | `sensorsx` | no |
| `#` | any topic | yes |

### 2.4 Single-level wildcard `+`

`+` matches exactly one level. An empty level is not a level:

| Filter | Topic | Match |
|---|---|---|
| `a/+/c` | `a/b/c` | yes |
| `a/+/c` | `a//c` | no |
| `a/+/c` | `a/b/c/d` | no |
| `a/+` | `a/` | no |
| `+` | `a` | yes |

`+` and `#` are only wildcards when they occupy a whole level. A level such as
`a+b` or `sensor#` is literal text.

### 2.5 Literal levels

Literal levels compare byte for byte, case-sensitively, after normalization as
defined in `matching.md` §1.4. There is no notion of a level being
"close enough".

### 2.6 Tenant scoping

Every topic belongs to a tenant or to none. Topics are laid out as
`tenant/<id>/...`; the `<id>` segment identifies the tenant. A rule may carry a
`tenant`:

- A tenant-scoped rule is a candidate **only** for topics whose tenant equals
  the rule's tenant. Tenant ids are case-sensitive and compare byte for byte
  after the topic has been normalized (see `matching.md` §1.4).
- A rule without `tenant` applies regardless of the topic's tenant.
- A tenant-scoped rule must never authorize an operation on a different
  tenant's topic, even when the topic text after the tenant prefix is
  identical.
- Tenant scoping is part of the decision context, so it applies identically to
  subscribe, publish, and delivery.

## 3. Versioning

Each successfully loaded rule set carries a monotonic version. Every decision
is made against exactly one version and that version is part of its cache key
(see `cache.md`). A reload increments the version and invalidates nothing by
editing rules in place: new readers observe the new version, old decisions are
never reused for it.

## 4. Default deny

Anything not explicitly allowed is denied, including:

- connections with no matching rule,
- operations on a topic that normalizes to nothing,
- requests whose tenant cannot be determined for a tenant-scoped check.
