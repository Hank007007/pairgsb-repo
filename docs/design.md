# Design notes

This is a short tour of how the relay is put together. Behavior is defined by
[`credential-routing.md`](credential-routing.md); nothing here overrides it.

## Request lifecycle

```
relay.request(url, opts)
  │
  ├─ bundle store.current ──► inject.selectEntry ──► headers
  │
  ├─ transport ──► response
  │     │
  │     ├─ 3xx + Location ──► redirect.followRedirect ──► next hop
  │     ├─ 429 / 5xx ──────► retry queue ──► next attempt
  │     └─ anything else ──► resolve
  │
  └─ audit.write(record)          (every attempt, redirect and failure)
```

A request is a sequence of **hops**, and each hop is a sequence of **attempts**.
A redirect starts a new hop; a retry repeats the current one.

## Modules

### `bundle.js` — the source of truth

Parses the JSON document, validates it (`version`, `secrets[]`, per-entry
`id`/`host`/`scheme`/`value`, optional timestamps), normalizes hosts (lower
case, port preserved, root dot dropped), and compiles the result into a frozen
**revision**. Revisions are immutable and carry a monotonically increasing
number.

`createBundleStore` holds the active revision. `replace()` swaps it and notifies
subscribers; readers always observe a whole revision, never a partially applied
one. `watchBundle` watches the file, debounces bursts, calls `loadBundle`, and
calls `replace` only on success, so a rejected bundle leaves the previous
revision serving traffic.

Compilation is strict: duplicate ids and two equally specific routes make the
bundle invalid rather than silently picking one.

### `inject.js` — routing and presentation

`effectiveHost(url)` is the routing key: the URL's hostname plus an explicit
port when it is not the scheme default. `selectEntry` maps a routing key to at
most one entry; `credentialHeader` maps an entry to the header it produces.
`applyCredential` replaces any caller-supplied header whose name matches
case-insensitively, so a caller cannot accidentally send two credential
headers.

Routing decisions are inputs, not state. Nothing in this module caches a
previous answer.

### `redirect.js` — hop construction

Classifies redirect statuses, resolves `Location` against the current URL,
decides whether the method collapses to `GET`, and refuses `https` → `http`
unless explicitly allowed. The object it returns describes the *next* request:
its URL, method, body and headers.

### `retry.js` — backoff and queueing

`computeDelay` is exponential backoff with full jitter clamped to
`maxDelayMs`. `createRetryQueue` owns the timers: it accepts a job, waits, and
calls `dispatch` with the material that job will send. Jobs can be cancelled,
and `close()` drains the queue. Timers are deliberately ref'd so a pending
retry keeps the process alive until it fires or is cancelled.

### `audit.js` — log-safe records

`redactHeaders`, `redactUrl` and `redactRecord` turn an in-memory description
of a request into something that can be shipped to a log sink. Header redaction
is name-based; URL redaction is value-based, which is why the active bundle is
consulted. `createAuditLog` assigns sequence numbers, caps retained records,
and calls an optional sink without letting sink failures escape.

The audit pass runs on every record, including redirects, failures and
retries, so the sequence number is a faithful ordering of what actually
happened.

### `transport.js` — the wire

A narrow adapter over `node:http` / `node:https` shaped as
`(request) => Promise<response>`. Tests supply their own implementation with
the same contract, which keeps the rest of the system independent of sockets.

### `index.js` — orchestration

`createRelay` wires the pieces: it resolves options, owns the bundle store and
retry queue, runs hops and attempts, writes audit records, and returns the
public surface (`request`, `store`, `audit`, `diagnostics`, `setBundle`,
`close`).

## Errors

All failures surface as `RelayError` subclasses with a stable `code`
(`BUNDLE_INVALID`, `TOO_MANY_REDIRECTS`, `INSECURE_REDIRECT`,
`TRANSPORT_ERROR`, `ABORTED`, `INVALID_OPTIONS`). Error messages describe the
request, never its credential.

## Testing approach

`test/helpers.js` provides an in-memory transport that records every outbound
request and replays scripted responses, so tests assert on the exact headers
that would have gone on the wire. Time-sensitive tests inject `random` and
small `baseDelayMs` values rather than sleeping for real.
