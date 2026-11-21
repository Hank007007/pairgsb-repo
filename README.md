# sealferry-relay

sealferry is the audit-log shipping segment between business nodes and a central
SIEM collector. A business process appends events to a local append-only source
file; the relay daemon tails that file, assembles batches, redacts secrets,
computes a digest per batch, and POSTs each batch to the collector. The collector
answers with an acknowledgement containing the batch id and digest. Only an
acknowledgement that matches the batch that was sent may advance the read cursor.

```
source file  ──►  reader  ──►  spool  ──►  batch  ──►  redact  ──►  deliver  ──►  collector
 (append-only)                 (queue +      (order +     (secrets)     (HTTP, retry,
                                cursor)       digest)                   ack)
```

## Layout

| Path | Role |
|---|---|
| `src/reader.js` | Reads and parses complete lines from the append-only source file. |
| `src/spool.js` | Durable local queue, cursor persistence, capacity trimming. |
| `src/batch.js` | Batch assembly, record ordering, digest computation. |
| `src/deliver.js` | HTTP delivery, retry policy, acknowledgement verification. |
| `src/redact.js` | Send-side redaction of secrets. |
| `src/clock.js` | Source/ship timestamps and skew. |
| `src/daemon.js` | Wires the pieces together and exposes `flush()`. |
| `test/` | `node:test` suites. |

## Running

```sh
node --test          # run the test suite
node src/daemon.js   # run the relay once against ./data (see daemon.js)
```

Only Node.js standard-library modules are used; there are no runtime
dependencies.

## Specifications

The normative behaviour is described in:

- [`docs/shipping.md`](docs/shipping.md) — source file, cursor, spool, capacity.
- [`docs/batch-format.md`](docs/batch-format.md) — batch envelope, record fields,
  ordering, digest scope, digest chain.
- [`docs/delivery.md`](docs/delivery.md) — acknowledgements, retries, idempotency,
  concurrency, failure reporting.
- [`docs/redaction.md`](docs/redaction.md) — what must be removed before sending.
- [`docs/time.md`](docs/time.md) — source time, ship time, skew.
