# Topic matching

This document defines how filters are turned into topics and how a topic is
tested against a filter. `acl-model.md` defines what happens once a rule
matches.

## 1. Filters and topics

### 1.1 Grammar

A *topic* is a sequence of levels separated by `/`. A *filter* is a topic that
may additionally contain `+` and `#` levels. Both are UTF-8 byte strings.

### 1.2 Shared subscriptions

A subscription may be written `$share/<group>/<filter>`. It is a shared
subscription in the named group.

- The **inner filter** is everything after `$share/<group>/`. It is the filter
  that is authorized, matched, and cached.
- The group name never participates in topic matching and never appears in the
  topic side of a rule. A rule `filter: sensors/+/temp` therefore covers
  `$share/g/sensors/+/temp`.
- `$share` with a missing or empty group, or with an empty inner filter, is
  malformed. It is denied and audited like any other rejection.
- Nested shared subscriptions (`$share/g/$share/h/...`) are not supported; the
  inner filter is taken literally after the first group segment, so such a
  filter is a literal filter beginning with `$share/`.

### 1.3 Expanding a filter

Authorization of a wildcard subscription asks whether the subscriber may
receive every topic the filter can produce. The expansion of a filter is the
set of topics obtained by replacing each `+` with a literal level and `#` with
zero or more literal levels. Because the literal alphabet is unbounded, the
implementation reasons over representative topics rather than enumerating all
of them; the rule the implementation must satisfy is stated in §2.2.

### 1.4 Leading, trailing, and repeated slashes

`/` is the level separator and every occurrence is significant:

- `a/b`, `/a/b`, and `/a/b/` are three distinct topics.
- `a//b` has an empty level between `a` and `b`.
- Splitting never discards empty leading or trailing levels.

A topic is *normalized* before it is compared: percent-encoded slashes
(`%2F`, either case) are decoded to `/` so that a slash cannot hide inside a
level. Normalization changes nothing else: case is preserved, separators are
never trimmed or collapsed, so `/a/b/` stays `/a/b/` and `a//b` stays `a//b`.

Tenant ids are case-sensitive: `tenant/ACME/...` and `tenant/acme/...` are two
different tenants.

The same normalization is applied wherever a topic is compared — subscribe,
publish, delivery, and cache keys — so that a topic is treated identically on
all three paths.

## 2. Matching rules

### 2.1 A single topic against a filter

A topic matches a filter when both agree level by level under these rules:

- a literal level matches only an identical literal,
- `+` matches exactly one non-empty level,
- `#` matches zero or more remaining levels and must be the final level.

### 2.2 A wildcard subscription

A subscribe request for a filter is allowed only when **all** topics in that
filter's expansion are individually allowed for the connection. If any topic
the filter could produce is denied, the subscription is denied as a whole, the
denial is audited with the id of the rule that caused it, and no partial
subscription is created.

A subscribe request for a filter with no wildcard is the single-topic case of
the same rule.

This applies equally to the inner filter of a shared subscription.

### 2.3 Delivery

Delivery is decided per concrete topic, not per filter: for a message published
on a topic, each subscriber whose (inner) filter matches that topic is a
candidate, and delivery to that subscriber requires an allow decision for the
current principal class, the topic, and the current rule-set version.

## 3. Hot reload

The rule set can be replaced at any time. Reload is not limited to new
connections:

- Subscriptions that are already established are re-evaluated for delivery
  against the new rule set on every delivery attempt.
- A subscriber that lost permission across a reload stops receiving messages
  immediately, even though its subscription was accepted earlier.
- A subscriber that gained permission across a reload starts receiving
  messages that match its existing filter without re-subscribing.
- Reload never rewrites a connection's principal class; the class is whatever
  the connection's most recent authentication established.

## 4. Worked examples

Assuming rules `allow sensor sub sensors/+/temp` and `allow sensor sub
sensors/#`:

| Filter subscribed | Allowed? | Why |
|---|---|---|
| `sensors/+/temp` | yes | every expansion is `sensors/<level>/temp` and is allowed |
| `sensors/#` | yes | includes `sensors` itself, also allowed |
| `sensors/+/+` | no | `sensors/a/b` not covered by either rule |
| `$share/g/sensors/+/temp` | yes | inner filter is allowed |
| `$share/g/sensors/+/+` | no | inner filter not fully covered |
