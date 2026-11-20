# routeclaim

routeclaim is the edge gateway that sits in front of the internal HTTP services.
It matches every inbound request against the route table in `config/routes.yaml`,
authenticates the caller, and forwards a normalized identity to the upstream.

## Layout

```
config/routes.yaml     route table (paths, methods, auth mode, scopes, upstream)
docs/route-auth.md     the authorization contract the gateway is expected to uphold
docs/incident-reports.md  postmortems that motivated parts of the contract
src/match.js           URL normalization and route selection
src/scope.js           scope list handling and scope coverage
src/introspect.js      token introspection client and result caching
src/proxy.js           upstream request construction
src/server.js          wiring: match -> authenticate -> authorize -> forward
src/config.js          route table loader
src/jwt.js             local verification for `jwt` routes
test/                  node:test suites
```

## Running

Node.js 20 or newer, no third-party dependencies.

```sh
node src/server.js
```

The listener defaults to `127.0.0.1:8080`; override it in the `server` block of
`config/routes.yaml` or point `ROUTECLAIM_CONFIG` at another file.

Every upstream named in the route table must be reachable from the gateway.
The introspection endpoint is configured in the `introspection` block.

## Tests

```sh
node --test
```

The suites start real local upstreams and a stub authorization service; no
network access is required.

## Request flow

1. The path is matched against the route table.
2. The route's `auth` mode decides how the caller is identified.
3. The route's `scopes` are checked against the identity.
4. A request is built for the route's upstream with gateway-owned identity
   headers and without caller credentials.
