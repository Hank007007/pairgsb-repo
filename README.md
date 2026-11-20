# tokelatch-reset-token

A small library for issuing and redeeming one-time credentials used by the
internal account system: password-reset links, invitation links and API-key
handoff links.

A credential is a random token that is shown to the end user exactly once. The
server keeps only the record it needs to recognise that token later — who it
was issued to, what it is for, when it expires — and redeeming it returns that
payload once. The normalised behaviour is specified in
[`docs/one-time-tokens.md`](docs/one-time-tokens.md); the storage layer is
specified in [`docs/storage.md`](docs/storage.md).

## Requirements

- Node.js 20 or newer. No third-party dependencies.

## Layout

```
src/
  index.js      public entry point and service factory
  issue.js      issuing a credential
  consume.js    redeeming a credential
  token.js      token generation and digesting
  store.js      in-memory and JSON-file stores
  audit.js      audit trail
  errors.js     error codes
  time.js       clock helpers and limits
docs/
  one-time-tokens.md   behavioural contract
  storage.md           storage contract and file format
test/
  tokens.test.js       node:test suite
```

## Usage

```js
import { createService, createStore } from './src/index.js';

const store = createStore('file', {
  file: './tokelatch-data/tokens.json',
  pepper: process.env.TOKELATCH_PEPPER,
});

const service = createService({
  store,
  pepper: process.env.TOKELATCH_PEPPER,
});

const { token, expiresAt } = await service.issue({
  subject: 'alice@example.com',
  purpose: 'password-reset',
  meta: { requestedFrom: 'web' },
});

// ...send `token` to the user, then, when the link is followed:

const payload = await service.consume({
  token,
  subject: 'alice@example.com',
  purpose: 'password-reset',
});
```

`consume()` throws a `TokenError` with a `code` from `ERROR_CODES` when the
token cannot be redeemed. `pruneExpired()` removes records whose expiry has
passed; call it from a scheduled job.

## Configuration

| Option | Default | Meaning |
|---|---|---|
| `pepper` | required | Server-side secret used when deriving token digests. |
| `clock` | `() => new Date()` | Service clock; expiry is evaluated against it. |
| `purposes` | `password-reset`, `invite`, `api-key-handoff` | Purposes that may be issued. |
| `defaultTtlMs` | 15 minutes | Lifetime when the caller does not ask for one. |
| `maxTtlMs` | 7 days | Upper bound on a requested lifetime. |

The pepper is a runtime secret. It is not committed to this repository; supply
it through the environment or a secret manager.

## Tests

```sh
node --test
# or
npm test
```

Use `TAP=1 node --test` for TAP output, or `node --test --test-reporter=spec`
for the spec reporter.

## Demo

```sh
node examples/basic.mjs
```
