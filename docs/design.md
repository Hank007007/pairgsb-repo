# markfence design

## Purpose

markfence sits between the comment composer and the client renderer. The author
submits a limited Markdown subset with inline HTML; markfence turns it into an
HTML fragment that is safe to insert into the comment area with `innerHTML`.

The normative behavior is defined in [`sanitize-policy.md`](sanitize-policy.md).
This document describes the shape of the implementation and the reasoning behind
the split; it does not restate the policy in full.

## Pipeline

```text
input string
   │
   ▼
tokenizer.tokenize        characters → start tags, end tags, text
   │
   ▼
tree.parseFragment        token stream → node tree
   │
   ▼
tree.walk + attrs         tree → sanitized tree
   │
   ▼
serialize.toHtml          sanitized tree → fragment string
```

The renderer adds highlighting on top of the sanitized fragment:

```text
fragment ──► render.withHighlight ──► highlighted fragment
```

## Modules

### `lib/tokenizer.js`

The tokenizer is a hand-written scanner. It recognizes comments, declarations,
start tags, end tags, and text. It also owns character-reference decoding, since
decoding is a lexical concern: it is about the input spelling, not about the
meaning of a tag or attribute.

Two responsibilities are deliberately separated here:

- `tokenize` produces a flat token stream. Attribute values are captured exactly
  as written, so downstream stages can decide when decoding is appropriate.
- `decodeEntities` maps a spelling to a character. Every consumer that makes a
  policy decision about a string is expected to run its input through this step
  first; a decision made on the raw spelling can be made about a different
  string than the one the browser will see.

Raw-text elements (`script`, `style`, `textarea`, `title`, and the legacy raw
text names) are scanned to their matching end tag so their contents arrive as a
single text token rather than as nested markup.

### `lib/url.js`

`isSafeUrl` decides one question: may this string be emitted in a URL-bearing
attribute? It is the single place where scheme rules live. It receives a value
whose character references have already been decoded, and it is responsible for
the normalization steps that determine which scheme (if any) the browser will
see.

The function is intentionally conservative: anything whose interpretation is
ambiguous — an unparseable prefix, an unexpected control character, a
protocol-relative reference — is treated as inadmissible.

### `lib/attrs.js`

`filterAttrs` implements the attribute policy: which names are admitted, on
which elements, with which value constraints. It is a pure function of the
element name and the attribute list; it does not know about serialization.

The attribute filter is also where `class` is validated as a single token. The
`class` value is a token list in HTML, so a value that contains separators is a
different thing from a single user class and is not admitted.

### `lib/sanitize.js`

`sanitize` composes the pipeline. Its contract is fixed: it accepts the input
fragment plus an options object and returns a string. Options are passed through
to the stages that need them; unknown options are ignored.

### `lib/tree.js`

`parseFragment` builds a tree from the token stream. It is not a full HTML5 tree
constructor; it only has to build the tree that the rest of the pipeline
understands, and it must be stable under a parse/serialize round trip.

`walk` is the traversal primitive. A visitor decides, per element, whether the
element is kept, dropped with its descendants, or unwrapped. The distinction
matters:

- dropping removes content that must never be shown (scripts, styles, embedded
  documents, foreign content);
- unwrapping removes only the wrapper, because the content is ordinary comment
  text.

`template` and `noscript` fall into the second category: their contents are
ordinary children, not opaque payloads, and their attributes are subject to the
same admission rules as any other element.

### `lib/serialize.js`

`toHtml` is the only component that turns a tree into markup. It emits a
canonical fragment: lowercase names, double-quoted attribute values, no
comments, no self-closing syntax for non-void elements.

The serializer is the last line of defense for attribute values. Whatever the
attribute filter admits, the serializer must emit in a form that reparses to the
same value — otherwise a value could become markup on the next parse. Because
the fragment is reparsed by the client, the serializer's escaping rules and the
tokenizer's parsing rules must agree.

### `lib/render.js`

`withHighlight` adds emphasis to search matches in a fragment. The renderer runs
after sanitizing, and its output is what the client receives, so the renderer's
output carries the same guarantees as the sanitizer's output.

Highlighting is a text transformation. Matches are found in text nodes, not in
markup; the wrapper element is `mark`, which is the only element the renderer
may introduce. The renderer does not relax the policy for its input: if the
fragment it receives contains content the policy would remove, the result
contains the sanitized form.

## Invariants

These are the properties the pipeline is intended to preserve. The full
statement is in the policy document; they are repeated here because they drive
the module boundaries.

1. **Whitelist closure.** The output contains only admitted element names and
   admitted attributes.
2. **Decode before decide.** A policy decision is made about the string the
   browser will decode, not about the input spelling.
3. **One serialization.** A tree is serialized once; the client reparses that
   output. The serializer must therefore produce output that reparses to an
   equivalent tree.
4. **Idempotence.** `sanitize(sanitize(s)) === sanitize(s)`.
5. **Uniform treatment.** Attributes inside `template` and `noscript` are judged
   by the same rules as attributes anywhere else.
6. **Renderer closure.** The renderer's output satisfies the same invariants,
   with `mark` added to the whitelist.

## Testing

`test/baseline.test.js` covers normal comment rendering: formatting, links,
images, lists, block quotes, code blocks, user classes, comments, and text
escaping, plus the two most common injection attempts. Snapshot expectations
for ordinary comments live in `test/snapshots/comments.json`.

The regression payload table in the policy appendix is the reference for
non-ordinary input. A change to any module above is expected to be checked
against that table.
