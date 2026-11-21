# spoolbatch-forward

Edge spool reader / shipper and central collector for audit events.

Applications append NDJSON audit records to a spool directory at the edge. The
agent (`spoolctl ship`) batches them per origin and epoch, signs a manifest for
each batch, and POSTs it to the collector (`spoolctl serve`). The collector
verifies the manifest and payload, deduplicates, tracks gaps, stores records,
and returns an accepted watermark.

## Layout

```
bin/spoolctl.js           command entry point (ship / serve)
src/protocol/manifest.js  manifest shape, canonical JSON, signing, digest
src/forwarder/spool.js    segment discovery and batch reading
src/forwarder/cursor.js   durable delivery position per stream
src/forwarder/client.js   batch assembly, signing, HTTP delivery
src/collector/verify.js   manifest shape + signature + payload verification
src/collector/dedup.js    duplicate detection
src/collector/gap.js      watermark and missing-range tracking
src/collector/store.js    append-only record store
src/collector/server.js   HTTP surface: POST /batches, GET /status
docs/                     normative design notes
test/                     node:test suites
```

## Running

Requirements: Node.js 20 or newer. No third-party dependencies.

```sh
# start a collector
node bin/spoolctl.js serve --store ./var/store --keyring ./keys.json --port 8787

# ship one batch per stream from a spool directory
node bin/spoolctl.js ship \
  --spool ./var/spool \
  --state ./var/state \
  --collector http://127.0.0.1:8787 \
  --key-id edge-a \
  --secret-file ./secret.txt \
  --limit 100
```

`keys.json` maps key ids to secrets and the origins each key may sign for:

```json
{
  "edge-a": { "secret": "s3cret", "origins": ["web-01#a", "web-02#b"] }
}
```

The agent's secret file contains the raw secret for `--key-id`.

## Spool input

Files are named `<percent-encoded origin>.<epoch>.<index>.ndjson`; each line is
one JSON record. See `docs/spool-format.md`.

## Tests

```sh
node --test
```

The suites use only `node:test`, `node:assert`, and the standard library, and
run entirely on local temporary directories and ephemeral HTTP servers.

## Documents

* `docs/spool-format.md` — segment layout, record framing, cursor semantics
  and durability
* `docs/batch-manifest.md` — manifest fields, canonical JSON, signing input,
  key binding, transport
* `docs/delivery.md` — at-least-once delivery, acknowledgements, retries,
  deduplication identity, out-of-order tolerance, gaps and watermarks
