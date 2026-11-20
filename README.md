# credmux-relay

Outbound credential-injection middleware for Node.js.

Business code makes one call:

```js
const { status, body } = await relay.request('https://api.partner.com/v1/orders', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ qty: 1 }),
});
```

The relay looks up the credential that belongs to the target host in the
active secrets bundle, injects it, follows redirects, retries transient
failures, and writes a redacted audit record for every attempt. Callers never
handle API keys or bearer tokens directly, and secrets never travel to hosts
the bundle does not name.

The normative behavior — host matching, temporal validity, redirect and retry
semantics, audit redaction — is specified in
[`docs/credential-routing.md`](docs/credential-routing.md). Read that first;
this README only covers usage.

## Install and test

Requires Node.js 20 or newer. There are no runtime dependencies.

```sh
node --test
```

## Secrets bundle

```json
{
  "version": 1,
  "secrets": [
    {
      "id": "partner-api",
      "host": "api.partner.com",
      "scheme": "bearer",
      "value": "tok_live_example",
      "not_before": "2026-01-01T00:00:00Z",
      "expires_at": "2026-12-31T23:59:59Z"
    },
    {
      "id": "billing-key",
      "host": "billing.example.net:8443",
      "scheme": "api-key",
      "value": "key_live_example"
    }
  ]
}
```

`scheme` is `bearer` (`Authorization: Bearer <value>`) or `api-key`
(`X-Api-Key: <value>`). `not_before` and `expires_at` are optional RFC 3339
timestamps. See `examples/secrets.json` for a runnable sample.

## Creating a relay

```js
import { createRelay } from 'credmux-relay';
import { loadBundle, watchBundle, createBundleStore } from 'credmux-relay/src/bundle.js';

const bundle = await loadBundle('./secrets.json', 1);
const relay = createRelay({ bundle, maxAttempts: 3, maxRedirects: 5 });

const watcher = watchBundle({ path: './secrets.json', store: relay.store });

try {
  const result = await relay.request('https://api.partner.com/v1/orders');
  console.log(result.status, result.attempts, result.redirects);
} finally {
  watcher.close();
  relay.close();
}
```

`createRelay` accepts:

| option | default | meaning |
| --- | --- | --- |
| `bundle` | empty bundle | initial compiled revision |
| `transport` | `nodeTransport` | `(request) => Promise<response>`; inject a fake in tests |
| `audit` | in-memory log | sink created by `createAuditLog()` |
| `maxAttempts` | `3` | attempts per hop, including the first |
| `baseDelayMs` | `100` | first backoff step |
| `maxDelayMs` | `5000` | backoff ceiling |
| `maxRedirects` | `5` | redirect hops per request |
| `allowInsecureRedirects` | `false` | permit `https` → `http` hops |

`relay.request(url, opts)` resolves with
`{ status, headers, body, url, attempts, redirects, redactedHeaders }` and
rejects with a coded error on transport failure, redirect loops and invalid
options. `opts` accepts `method`, `headers`, `body`, `timeoutMs` and `signal`.

`relay.diagnostics()` reports the active revision, entry count, queued retries
and request counters. `relay.close()` stops the retry queue.

## Hot reload

`watchBundle({ path, store })` watches the bundle file, debounces writes,
parses the new revision and swaps it into the store. A bundle that fails
validation leaves the previous revision in effect. Requests and queued retries
always route against whatever revision is active when they are sent.

## Audit trail

```js
import { createAuditLog } from 'credmux-relay/src/audit.js';

const audit = createAuditLog({
  sink: (record) => process.stdout.write(JSON.stringify(record) + '\n'),
});
const relay = createRelay({ bundle, audit });
```

Records carry the sequence number, timestamp, method, URL, credential `id`,
attempt number, status and headers. The audit pass is responsible for making
those records safe to ship to a log sink; the rules are in §7 of the spec.

## Layout

```
src/bundle.js      bundle parsing, validation, store, hot reload
src/inject.js      host matching and header injection
src/redirect.js    redirect classification and follow-up construction
src/retry.js       backoff and the retry queue
src/audit.js       audit records and redaction
src/transport.js   node:http / node:https adapter
src/index.js       createRelay()
test/              node:test suites
```
