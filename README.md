# cookieveil

Sealed cookies for edge gateways. The login service issues a cookie; every
downstream service opens it locally with a shared key ring.

- Wire format and acceptance rules: [`docs/cookie-envelope.md`](docs/cookie-envelope.md)
- Incident history: [`docs/incident-reports.md`](docs/incident-reports.md)
- Architecture and invariants: [`DESIGN.md`](DESIGN.md)

## Requirements

Node.js 20 or newer. No third-party dependencies.

## Usage

```js
import { issue, read } from "cookieveil";

const keys = [
  { kid: "2026-11", secret: currentSecret },
  { kid: "2026-10", secret: previousSecret },
];

// At the login service.
const cookie = issue(
  {
    iss: "login",
    sub: "user-42",
    aud: ["profile", "billing"],
    scopes: ["profile:read"],
    iat: Math.floor(Date.now() / 1000),
    exp: Math.floor(Date.now() / 1000) + 900,
    jti: "7f2c1a90",
  },
  { keys },
);

// At a downstream service.
try {
  const claims = read(cookie, {
    keys,
    service: "profile",
    now: Date.now(),
  });
  // claims.sub, claims.scopes, ...
} catch (error) {
  // error.code is a stable rejection code; treat every error as a rejection.
}
```

`issue(claims, { keys, kid? })` returns a `v2.h.b.m` string.
`read(cookie, { keys, service?, now? })` returns a fresh claims object or
throws a `CookieError`.

## Tests

```sh
node --test
```

Test keys are generated at run time; the suite does not use the network.
