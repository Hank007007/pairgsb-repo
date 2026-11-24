# topicwarden

topicwarden sits in front of an internal MQTT cluster and answers the broker's
authorization questions: may this connection subscribe to this filter, may it
publish to this topic, and should a message on this topic be delivered to this
subscriber.

Rules are a YAML document matched on principal *class* (never on individual
user names), reloaded at runtime. Topics are multi-tenant and laid out as
`tenant/<id>/...`; subscriptions support MQTT `+` and `#` as well as
`$share/<group>/<filter>`.

## Layout

```
cmd/topicwardend/      service entry point (HTTP decision API, reload)
internal/acl/          rule loading, filter matching, policy evaluation, expansion cache
internal/broker/       connection-facing authorization and delivery decisions
internal/tenant/       topic normalization and tenant derivation
internal/audit/        decision records (one JSON object per line)
docs/                  normative behaviour
  design.md            pipeline overview
  acl-model.md         rules, effect precedence, principal classes, tenants
  matching.md          filters, shared subscriptions, slashes, reload
  cache.md             what each cache is allowed to key on
  audit.md             what a record contains
testdata/rules.yaml    example policy document
```

## Running

```sh
go test ./...
go run ./cmd/topicwardend -rules testdata/rules.yaml -addr 127.0.0.1:8088
```

The service exposes:

- `POST /v1/authorize` with
  `{"client_id":"c-1","principal_class":"sensor","action":"sub","filter":"sensors/+/temp"}`,
  or `action: "pub"` with `"topic": "sensors/a/temp"`.
- `POST /v1/reload` to re-read the rule document.

SIGHUP reloads the document as well.

## Behaviour

The normative description lives in `docs/`. In short:

- deny beats allow independent of document order, and an unrecognized effect
  makes the whole document fail to load;
- `#` matches zero or more levels, `+` exactly one non-empty level, and
  leading/trailing slashes are significant levels;
- a tenant-scoped rule only decides topics of its own tenant;
- a wildcard subscription is accepted only if its whole expansion is allowed;
- shared subscriptions authorize their inner filter;
- already-established subscriptions are re-checked against the live rule set
  on every delivery, and a new principal class after re-authentication applies
  immediately;
- every deny is audited with the rule id that caused it, and payloads are
  never logged.
