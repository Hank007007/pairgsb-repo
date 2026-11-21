# scimfence

SCIM 2.0 facade in front of the internal identity store. Upstream HR systems
call it with a Bearer token to provision users and groups; sensitive
attributes (`groups`, `roles`, `active`) are authorized separately on read and
write.

Only Node.js built-in modules are used. No third-party packages.

## Run

```sh
node src/index.js
```

The process listens on an ephemeral port (override with `PORT`), seeds a small
in-memory data set, and prints one token per tenant.

## Test

```sh
node --test
```

## Layout

```
src/http/router.js       route table, entity tag parsing
src/http/app.js          request lifecycle, query params, conditional requests
src/auth/token.js        token issue/verify, scope predicates
src/filter/parse.js      SCIM filter -> syntax tree
src/filter/compile.js    syntax tree -> internal query condition
src/patch/apply.js       PATCH add/replace/remove over attribute paths
src/repo/users.js        in-memory user repository (tenant-scoped)
src/repo/groups.js       in-memory group repository (tenant-scoped)
src/repo/query.js        condition evaluation
src/scim/schema.js       attribute definitions
src/scim/serialize.js    SCIM resource rendering and projection
docs/                    normative profile, authorization model, design notes
test/                    node:test suites
support/                 test harness (server bootstrap, token helpers)
```

## Normative documents

- [`docs/scim-profile.md`](docs/scim-profile.md) — resources, filter grammar,
  PATCH semantics, pagination, conditional requests.
- [`docs/attribute-authz.md`](docs/attribute-authz.md) — token model, tenant
  binding, attribute-level read/write scopes.
- [`docs/design.md`](docs/design.md) — intended behavior and invariants.
- [`docs/migration-notes.md`](docs/migration-notes.md) — historical incidents.
