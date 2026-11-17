# envwrap

`envwrap` moves a small session object between stateless services as a compact,
signed string. A gateway serializes the session to JSON, base64url-encodes it,
and appends an HMAC-SHA256 signature. A downstream service opens the envelope,
verifies it, and reconstructs the session object.

The signature provides integrity only. The payload is not encrypted and must
not carry secrets.

## Usage

```js
const { seal, open } = require('envwrap');

process.env.ENVWRAP_SECRET = 'replace-with-a-long-random-value';

const token = seal({ sub: 'user-1234', roles: ['writer'] });
const session = open(token);
```

A key can also be passed per call as `{ key: '...' }`; it takes precedence over
the `ENVWRAP_SECRET` environment variable.

## Commands

```sh
node --test       # run the test suite
node scripts/demo.js
```

The wire format, validation rules, and rejection semantics are specified in
[docs/envelope.md](docs/envelope.md).

## Layout

| Path | Contents |
| --- | --- |
| `src/envelope.js` | sealing and opening envelopes |
| `src/codec.js` | JSON payload and base64url conversion |
| `src/limits.js` | envelope size limits |
| `src/key.js` | key resolution |
| `src/errors.js` | error types and codes |
| `test/` | test suite |
| `scripts/demo.js` | end-to-end demonstration |
