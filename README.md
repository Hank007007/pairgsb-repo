# keywaytransit

A small encryption-as-a-service for platform teams. Callers hand over a
plaintext and a logical key name; the service returns an opaque blob that can
be stored as-is and decrypted later. Keys are versioned, optionally bound to a
context, and scoped per tenant. AES-256-GCM throughout.

## Layout

```
keyway/            service library
  aead.py          AES-256-GCM seal/open
  blob.py          blob encoder/decoder
  context.py       context canonicalisation and comparison
  keystore.py      tenant key material and service operations
  audit.py         audit sink
  errors.py        error codes
  format.py        shared wire constants
  http/            request routing and batch handling
keywayd/main.py    process entry point
docs/              binding specifications
DESIGN.md          behaviour contract
tests/             pytest suite
testdata/keys/     fixture key records
```

## Running

```
pip install -r requirements.txt
python -m keywayd.main --keys-dir keys --port 8080
```

Every request carries `x-tenant-id`. Example:

```
curl -s -X POST localhost:8080/v1/encrypt \
  -H 'x-tenant-id: acme' -H 'content-type: application/json' \
  -d '{"key_name": "billing", "plaintext": "4111", "context": {"field": "pan"}}'
```

The reply is `{"blob": "<base64>"}`. Pass that string back to `/v1/decrypt`
with the same context to recover the plaintext.

## Tests

```
python -m pytest -q -p no:cacheprovider
```

## Specifications

Read `docs/blob-format.md`, `docs/key-lifecycle.md`, `docs/context.md` and
`docs/api.md` before changing anything in `keyway/`. `DESIGN.md` summarises the
contract the service is expected to satisfy.
