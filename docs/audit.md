# Audit

Every authorization decision that ends in deny produces exactly one audit
record. Allowed decisions are recorded as well, at the same granularity, but
denials are the ones operators and the compliance pipeline rely on.

## 1. Record shape

Records are the `internal/audit.Event` struct serialized as one JSON object per
line:

```json
{"time":"2026-01-02T03:04:05Z","decision":"deny","action":"sub",
 "client_id":"c-17","principal_class":"sensor","filter":"sensors/+/+",
 "tenant":"acme","rule_id":"sensors-read","reason":"not covered"}
```

- `decision` is `deny` or `allow`.
- `action` is `sub` or `pub`. Delivery denials use the action of the
  subscription they belong to (`sub`).
- `rule_id` is present on every deny that a rule caused, and names the rule
  whose match made the decision deny. A deny caused by the absence of any
  matching rule carries a reason instead and no `rule_id`.
- For a subscribe request, `filter` is the filter as the client sent it,
  including the `$share/<group>/` prefix for a shared subscription. It is the
  inner filter that is authorized and matched; the group is not part of any
  topic.
- For publish and delivery, `topic` is the normalized topic and `filter` is
  empty.

## 2. Volume

One record per decision, not one per rule considered. A wildcard subscribe that
is rejected because one representative topic is uncovered produces a single
deny record naming the rule responsible.

## 3. Prohibited content

Audit records never contain message payloads, payload fragments, payload
lengths, or credentials. Only identifiers, topic text, and decisions are
recorded. Topic text is normalized before it is recorded so that a topic
encoded in two ways does not look like two topics in the log.

## 4. Delivery

Record a delivery denial when a message is not delivered to a subscriber that
was a candidate because the current policy denies it. Do not record delivery
denials for subscribers whose filter simply does not match the topic.

## 5. Retention

The process writes records to the configured sink and keeps an in-memory copy
for tests and for the admin endpoint. The in-memory copy is the same data; no
transformation happens between the two.
