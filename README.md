# routeward

Route-aware authorization middleware for the internal HTTP API. It sits in
front of the business handlers, authenticates the session cookie, and then
decides — from `config/routes.json` — whether the incoming method and path may
proceed.

Only Node.js built-ins are used.

## Layout

```
config/routes.json     rule table
docs/route-policy.md   behavioural specification
src/index.js           process entry point
src/server.js          http.Server wiring
src/app.js             middleware pipeline
src/sessions.js        cookie -> session (user, tenant, scopes)
src/normalize.js       request target -> normalized path
src/rules.js           rule table loading and selection
src/middleware.js      authorization decision
src/dispatch.js        effective method resolution
src/handlers.js        business handlers
test/                  node:test suites
```

## Running

```sh
npm start            # listens on 127.0.0.1:8080 (override with PORT / HOST)
npm test             # node --test
```

## Trying it by hand

The session cookie is `rw_session`; `src/sessions.js` lists the seeded
sessions. For example:

```sh
curl -i -H 'cookie: rw_session=sess_bob' http://127.0.0.1:8080/admin/users
curl -i -H 'cookie: rw_session=sess_dave' http://127.0.0.1:8080/admin/users
```

## Configuration

A rule has `id`, `pattern`, `methods`, `scopes`, and an optional `tenants`
allow-list. `docs/route-policy.md` is the source of truth for pattern syntax,
normalization, selection among multiple matches, method overrides, and the
status codes returned for each outcome.
