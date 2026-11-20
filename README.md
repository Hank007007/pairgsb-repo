# svgfence

An allowlist-based SVG sanitizer for avatar uploads. User-supplied SVG files
pass through `sanitize()` before being written to disk and served to an
`<img src="...">` element, so the sanitizer's reading of the bytes has to match
what the browser and its XML parser will do.

The implementation is self-contained: a small tokenizer (`src/tokenize.js`),
attribute filtering (`src/attr.js`), reference validation (`src/url.js`), CSS
sanitization (`src/css.js`), and the main walk (`src/sanitize.js`). It has no
runtime dependencies.

## Layout

```
docs/sanitizer-policy.md   normative policy: what the output must contain
fixtures/                  sample uploads, including hostile ones
scripts/demo.js            prints each fixture before/after
src/tokenize.js            tag, attribute, comment, DOCTYPE and PI tokens
src/attr.js                allowlist + per-type attribute checks
src/url.js                 reference scheme decision
src/css.js                 style= and <style> declaration filtering
src/sanitize.js            token stream -> sanitized document
test/                      node:test suites
```

## Usage

```js
import { sanitize } from './src/index.js';

const safe = sanitize(untrustedSvgString);
```

`sanitize()` throws a `TypeError` when given anything other than a string.

## Commands

```
node --test            run the test suite
node scripts/demo.js   print every fixture before and after sanitization
```

Both use only the Node.js standard library (Node 20 or newer).
