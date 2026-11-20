# tenantmint

Internal file service's short-lived access-grant mint. A tenant administrator
asks for a grant with `POST /grants`; the service checks the request against
the tenant policy and returns a token. When a file is fetched, `verify` decodes
the token and decides whether the operation is allowed. A grant can be
withdrawn before it expires.

The behaviour the service is supposed to have is written down in
[`docs/grant-model.md`](docs/grant-model.md). [`docs/incident-reports.md`](docs/incident-reports.md)
records a past incident against this component.

## Layout

```
src/
  codec.js              grant token encoding and decoding
  index.js              service wiring (issue / verify / revoke)
  server.js             HTTP surface, plus a demo entry point
  grant/issue.js        grant request handling
  grant/verify.js       token checking
  pathnorm/norm.js      path normalisation and containment
  policy/tenant.js      tenant policy lookup
  store/revoke.js       withdrawal bookkeeping
test/                   node:test suites
docs/                   behavioural specification and incident write-up
```

A tenant policy looks like this:

```js
{
  tenantId: 'tenant-a',
  root: '/tenant-a',
  adminActions: ['read', 'write'],
  defaultTtlMs: 60_000,
  maxTtlMs: 600_000,
}
```

## Running

Requires Node.js 20 or newer; no third-party dependencies.

```sh
npm test          # node --test
npm start         # serve the HTTP surface on $PORT (default 8080)
```

## HTTP surface

| Method | Path | Body | Result |
|---|---|---|---|
| `POST` | `/grants` | `{ tenantId, path, actions, ttlMs? }` | `201` with `{ token, grant }` |
| `POST` | `/verify` | `{ token, action, path, tenantId? }` | `200` when allowed, `403` otherwise |
| `POST` | `/revocations` | `{ tenantId, id }` | `204` |

`npm start` loads a small demo policy table (`tenant-a`, `tenant-b`) and keeps
withdrawal state under `data/`.
