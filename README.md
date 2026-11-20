# markfence

markfence is a small sanitizing layer for community comment rendering. It accepts
the limited rich-text input used by the comment composer, parses it into a simple
document tree, applies the sanitizing policy, and serializes the result as an HTML
fragment that the client can insert into the comment area.

The project deliberately depends on the Node.js standard library only. The
tokenizer and serializer are part of this repository.

## Usage

```js
const { sanitize } = require("markfence");

const fragment = sanitize(
  '<p>Hello <strong>world</strong>.</p><script>alert(1)</script>'
);
// => "<p>Hello <strong>world</strong>.</p>"
```

Highlighting is a separate rendering step:

```js
const { sanitize, render } = require("markfence");

const safe = sanitize('<p>Hello brave world</p>');
const highlighted = render.withHighlight(safe, "world");
// => "<p>Hello brave <mark>world</mark></p>"
```

## API

- `sanitize(html, opts?)` — parse, sanitize, and serialize an input fragment.
- `render.withHighlight(fragment, query)` — add `<mark>` wrappers to matches in an
  already-sanitized fragment.
- `render.render(html, opts?)` — sanitize and then highlight in one step
  (`opts.highlight` holds the query).

## Tests

```sh
node --test
```

The baseline suite covers ordinary comments: links, code blocks, images, lists,
block quotes, emphasis, line breaks, user classes, and the two most common
markup-injection attempts. The normative behavior of the sanitizer is documented
in [`docs/sanitize-policy.md`](docs/sanitize-policy.md); the module layout and
data flow are described in [`docs/design.md`](docs/design.md).

## Layout

```text
lib/
  tokenizer.js   lexical scanning and character-reference decoding
  url.js         URL admissibility
  attrs.js       attribute admission
  tree.js        fragment parsing and tree traversal
  serialize.js   tree-to-HTML serialization
  sanitize.js    parse → traverse → serialize pipeline
  render.js      highlighting for the client renderer
  index.js       public entry point
docs/
  sanitize-policy.md
  design.md
test/
  baseline.test.js
  snapshots/
```
