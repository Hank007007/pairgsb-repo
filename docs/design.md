# Design

topicwarden answers one question three times per message: "may this
connection do this?" The broker asks on SUBSCRIBE, on PUBLISH, and again on
each delivery attempt. The design keeps a single evaluation path so the three
answers cannot drift apart.

## 1. Pipeline

```
connection ──> authorize subscribe ──> decision cache ──> publish authorize ──> deliver
     │                    │                    │                    │                │
  principal class    expand filter       keyed by context       topic lookup     re-evaluate now
  (may change)       check coverage      policy version         tenant check     against live rules
```

The order matters because each stage constrains the next:

1. **Connection** — the principal class is the identity rules are written
   against. It is read at decision time, never captured when the subscription
   was created, because re-authentication changes it.
2. **Subscribe** — a subscription is a promise about a whole filter, so the
   answer is about every topic the filter can produce (see
   `matching.md` §2.2). Shared subscriptions are unwrapped to their inner
   filter first (`matching.md` §1.2).
3. **Cache** — a cached decision is only usable while every input that can
   change it is unchanged (`cache.md`).
4. **Publish** — the same topic normalization and tenant derivation as the
   subscribe path, otherwise a topic could be acceptable on one path and not
   the other.
5. **Delivery** — authorization is re-checked against the *current* rule set
   for every candidate subscriber (`matching.md` §3). Accepting a subscription
   earlier does not grant permission later.

## 2. Data flow

- `acl.LoadRules` turns the YAML document into `RuleSet`. A document that
  fails to load is rejected whole; the previously active set stays live.
- `acl.Store` holds the active `RuleSet` and its version. Reload installs a
  new set with a higher version.
- `acl.Evaluate` decides one concrete topic for one principal class, action,
  and rule set.
- `acl.CompiledCache` memoizes the expansion of a filter, which is the
  expensive part of a subscribe decision.
- `broker.Authorizer` owns the connection-facing API and the decision cache.
- `internal/tenant` owns normalization and tenant derivation so that both
  the subscribe and the publish path agree on what a topic *is*.
- `internal/audit` writes one record per decision, with a rule id on every
  deny a rule caused.

## 3. Invariants

- Default deny: no matching rule means deny, on every path.
- Deny dominates allow regardless of document order.
- An unknown effect rejects the document.
- A tenant-scoped rule never decides another tenant's topic.
- A policy change is visible to already-established subscriptions.
- A principal class change is visible to every subsequent decision for that
  connection.
- Topic normalization is identical on subscribe, publish, delivery, and in
  cache keys.

## 4. What this repository is

This repository is the plugin, its rule document, and its decision engine. It
does not embed an MQTT broker; `cmd/topicwardend` exposes the decision API
over HTTP and reloads the rule document, so the three call sites can be
exercised directly.
