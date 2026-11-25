# protolens

protolens is the byte-facing layer of an internal RPC gateway. Edge collectors
encode requests as protobuf and sign the encoded bytes; the gateway resolves the
schema the sender used, decodes the body, verifies the signature over the bytes
it actually received, and hands the result to routing.

```
bin/protolensd            daemon entry point (arguments and exit codes are fixed)
lib/wire/varint.js        varint read / write
lib/wire/decode.js        decodeMessage / readField
lib/wire/encode.js        encodeMessage
lib/schema/registry.js    published type versions, hot reload
lib/schema/resolve.js     resolveType / fieldByNumber / cacheKey
lib/gateway/verify.js     verifyRequest, signRequest
lib/gateway/server.js     handleRequest
lib/errors.js             ProtolensError and the stable error codes
docs/                     wire.md, gateway.md, schema.md, design.md
testdata/                 schema snapshots and encoded message samples
test/                     node:test suites
tools/fixtures.js         test-data loaders shared by the suites
```

## Requirements

Node.js 20 or newer. No runtime dependencies; only the standard library is used.

## Running

```sh
npm test                 # node --test
node bin/protolensd --help
node bin/protolensd --listen 7070 --schema testdata/schemas/telemetry.v1.json
```

`--help` exits 0, an invalid argument exits 2, and an unreadable schema file
exits 3.

## Reading order

1. `docs/design.md` — how the layers fit together and which layer owns which
   representation.
2. `docs/wire.md` — the byte grammar: varints, tags, lengths, nesting, unknown
   fields, repeated scalars.
3. `docs/schema.md` — versions, hot reload, in-flight requests, caching.
4. `docs/gateway.md` — admission, signature, failure reporting.

The three rule documents are normative. `docs/design.md` is explanatory.

## Request shape

`handleRequest(request, ctx)` takes:

- `request.body` — the raw bytes as received;
- `request.type` — the registered message type;
- `request.version` — the schema version the sender encoded against (optional);
- `request.signature` — the HMAC of the body;
- `request.headers` — transport metadata, not part of the message or signature.

`ctx` supplies `registry`, `secret`, an optional `router`, an optional `logger`,
and a `resolve(type, version)` function that returns the resolved schema for a
request. `handleRequest` pins the version once and passes a `resolve` bound to
that pinned version down to `verifyRequest`.

## Test data

`testdata/schemas/` holds published snapshots (`telemetry.v1.json`,
`telemetry.v2.json`, `envelope.v1.json`). `telemetry.v2.json` renumbers fields,
as a hot reload that changes a schema would. `testdata/messages/` holds encoded
samples used by the tests.
