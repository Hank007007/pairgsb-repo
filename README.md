# mergeknot-config

`mergeknot-config` is the configuration SDK used by Mergeknot services. It
composes a single configuration object from several layers:

```
defaults -> base.json -> environment overlays -> environment variables -> --set
```

A JSON Patch entry point is provided for hot reloads.

## Requirements

- Node.js 20.11 or newer
- No runtime dependencies; tests use the built-in `node:test` runner

## Usage

```js
import { load, applyPatch } from 'mergeknot-config';

const { config, errors } = load({
  defaults: { db: { pool: { size: 4 } }, features: [] },
  layers: [
    'config/base.json',
    'config/production.json',
  ],
  env: process.env,
  prefix: 'APP_',
  argv: process.argv.slice(2),
});

const hot = applyPatch(config, [
  { op: 'replace', path: '/db/pool/size', value: 16 },
]);
```

`load()` returns `{ config, errors }`. `applyPatch()` returns
`{ document, errors }`. Both are documented further in
[`docs/merge-rules.md`](docs/merge-rules.md) and
[`docs/design.md`](docs/design.md).

## Scripts

- `npm test` — run the test suite with `node --test`
- `npm run demo` — print the result of a three-layer load
- `npm run check:proto` — load a fixture that carries a hostile key and report
  whether the global object stayed clean

## Layout

```
src/            SDK implementation
test/           node:test suites
docs/           behaviour specification and design notes
examples/       runnable demos and sample configuration
```
