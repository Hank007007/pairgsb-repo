# Design

This note describes the shape of the service and how a request travels from
the HTTP layer to the store. The behaviour each component must deliver is
specified in `docs/query-rules.md`; the field names come from
`docs/field-map.md`. Neither is restated in full here.

## Components

| module           | role                                                      |
| ---------------- | --------------------------------------------------------- |
| `src/server.mjs` | HTTP surface: `POST /query`, `GET /health`                |
| `src/service.mjs`| turns a parsed body into a `{ status, body }` result      |
| `src/builder.mjs`| composes the SELECT statement text plus its bound values  |
| `src/plan.mjs`   | appends the paging window to a composed SELECT            |
| `src/pool.mjs`   | the storage adapter; executes what the builder produced   |
| `src/fields.mjs` | the field map as code, used by the builder                |
| `src/db.mjs`     | schema, seed rows, in-memory store factory                |
| `src/errors.mjs` | `QueryError`, the one error type the service maps to 400  |

`src/fields.mjs` and `docs/field-map.md` describe the same whitelist. The
document is the reference; the module is what the running code consults.

## Request flow

```
HTTP body
  -> server.readBody        parse JSON, reject non-JSON
  -> service.handleQuery    validate the body envelope
  -> builder.buildQuery     { sql, params }
       builder.buildWhere
         builder.quoteIdent
         builder.buildLike
       builder.buildOrderBy
       plan.applyPage
  -> pool.run(db, sql, params)
  -> service.handleQuery    rows or an error envelope
  -> server.sendJson
```

`buildQuery` produces two things: the statement text and the list of values
that accompany it. `buildWhere` renders predicates and contributes to that
list in placeholder order. `applyPage` is the last composition step and closes
the statement with the paging window.

`run` is the only module that touches the driver. It receives the composed
text and the value list and returns the rows of the statement.

## Statement shape

A composed query is a single `SELECT` over `items`:

```
SELECT <columns> FROM items [WHERE <predicate>] [ORDER BY <keys>] LIMIT <n> OFFSET <n>
```

`<columns>` is derived from the field map, in field-map order. Predicates
nest through `AND`/`OR` groups, each group parenthesised so nesting is
unambiguous.

## The store

`src/db.mjs` opens an in-memory store, creates the schema and inserts a fixed
set of rows, so every process starts from the same state. All connections are
opened through `openDatabase` in `src/pool.mjs`, which configures the engine
for the legacy schema.

## Errors

`QueryError` carries `code`, an optional `field`, and an HTTP `status`. The
service maps it to an error envelope and leaves everything else as
`internal_error`. Codes and their triggers are listed in
`docs/query-rules.md` §7.

## Tests

`test/builder.test.js` covers the normal path: composition, filters, nesting,
sorting, paging, and the service envelope. New coverage belongs in its own
file under `test/`.
