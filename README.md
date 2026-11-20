# urlseal-presign

Presigned download/upload links for object storage. A link carries the
protocol parameters (`X-Seal-*`) and a `sig` in its query string; the gateway
calls `verify()` to reconstruct the request and decide whether it may reach
the backend.

## Layout

```
src/canonical.js   query and path canonicalization
src/presign.js     canonical string + link issuing
src/verify.js      gateway-side verification
src/keys.js        keyring loading and rotation
src/sign.js        HMAC wrapper and key resolution
keys.json          keyring used by the sample code and tests
docs/              format and verification specification
test/              node:test suites
```

## Usage

```js
import { load, presign, verify } from './src/index.js';

const keyring = load(new URL('./keys.json', import.meta.url));

const url = presign({
  bucket: 'assets',
  key: 'reports/2024/q1.pdf',
  method: 'GET',
  expiresIn: 600,
  tenant: 'demo',
  query: { a: '1', b: '2' },
  keyring,
});

const result = verify(url, { keyring });
// { ok: true, tenant: 'demo', method: 'GET', bucket: 'assets', key: '...' }
```

`presign()` and `verify()` both accept a `now` option (unix seconds) so
callers can drive the clock explicitly.

## Keyring

`keys.json` maps each tenant to an ordered list of key versions. The first
entry signs new links; older entries are kept so links issued before a
rotation keep verifying. See `docs/verification.md` §4.

## Specification

- [`docs/presign-format.md`](docs/presign-format.md) — request fields,
  canonical string, key and query canonicalization, the cross-language test
  vector.
- [`docs/verification.md`](docs/verification.md) — verification steps, bound
  fields, expiry, rotation, comparison and algorithm rules.

## Tests

```
node --test
```
