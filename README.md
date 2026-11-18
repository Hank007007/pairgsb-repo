# parambind

Query builder for the internal data service. The frontend posts filter, sort
and paging parameters to `POST /query`; the service turns them into one SQL
statement and runs it against the embedded store.

Standard library only — no runtime dependencies, no build step.

## Layout

```
src/
  builder.mjs   composes the SELECT: WHERE, ORDER BY, identifiers
  plan.mjs      appends the LIMIT/OFFSET window
  pool.mjs      thin driver adapter: hands composed SQL to the store
  fields.mjs    the field map as code (mirrors docs/field-map.md)
  errors.mjs    QueryError and its error codes
  db.mjs        schema and seed data
  service.mjs   request body -> builder -> pool -> HTTP-shaped result
  server.mjs    node:http server exposing POST /query and GET /health
  main.mjs      process entry point
docs/
  query-rules.md  normative behaviour of POST /query
  field-map.md    authoritative field whitelist
  design.md       how the pieces fit together
test/
  builder.test.js normal-path coverage
```

## Run

```sh
npm start                 # listens on 127.0.0.1:3000, PORT overrides
node --test               # run the test suite
```

```sh
curl -s localhost:3000/query \
  -H 'content-type: application/json' \
  -d '{"filter":{"field":"status","op":"eq","value":"paid"},"sort":{"field":"amount","direction":"desc"},"limit":5}'
```

The store starts in memory and is reset on every process start.

## Reading order

`docs/query-rules.md` is the contract. `docs/field-map.md` is the whitelist and
is not edited. `docs/design.md` explains the components and the request flow.
