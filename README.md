# partfence

`partfence` is the upload gateway's own `multipart/form-data` front door. Every
request that may carry files is split into parts here first; the downstream
storage service only ever sees the normalised `parts[]` array produced by this
layer.

The parser is streaming: request bytes arrive in arbitrary chunks through
`MultipartParser.write()`, and `end()` closes the request. Nothing is handed to
storage until the whole request has been accepted.

## Layout

```
src/parser.js     multipart framing state machine
src/headers.js    part header block and Content-Disposition parsing
src/filename.js   maps a client filename onto a tenant-relative path
src/limits.js     request budgets and per-part accounting
src/index.js      public API
bin/demo.js       replays fixture requests and prints the result
fixtures/         raw request bytes plus a replay manifest
docs/             the normative parsing policy
```

## Usage

```js
const { MultipartParser, parseMultipart } = require('partfence');

// One-shot
const parts = parseMultipart(bodyBuffer, request.headers['content-type']);

// Streaming
const parser = new MultipartParser({
  contentType: request.headers['content-type'],
  limits: { maxTotalBytes: 32 * 1024 * 1024, maxParts: 128 },
});
req.on('data', (chunk) => parser.write(chunk));
req.on('end', () => {
  const parts = parser.end();
  // [{ name, filename, contentType, size }, ...]
});
```

`parseMultipart()` and `end()` return an array of parts. Each entry carries the
normalised `name`, the tenant-relative `filename` (or `null` for non-file
fields), the declared `contentType` (or `null`), and the body `size` in bytes.

On rejection a `MultipartError` is thrown. It carries the HTTP `status` the
gateway should return, a stable `code`, and a human-readable `message`:

```js
try {
  const parts = parseMultipart(body, contentType);
} catch (err) {
  if (err instanceof MultipartError) {
    res.writeHead(err.status, { 'content-type': 'application/json' });
    res.end(JSON.stringify(err.toJSON()));
  }
}
```

## Running

```sh
node --test          # test suite
node bin/demo.js     # replay every fixture
node bin/demo.js folded-disposition.txt   # replay one fixture
```

## Policy

`docs/multipart-policy.md` is the normative description of what this layer
accepts and how it normalises metadata. It states requirements, not code.
