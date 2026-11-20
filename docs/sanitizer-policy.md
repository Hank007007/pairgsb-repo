# SVG sanitizer policy

`svgfence` sanitizes user-supplied SVG avatars. The sanitized bytes are written
to disk and served to an `<img src="...">` element, so every decision has to be
made on the same interpretation of the bytes that the browser and its XML parser
will use. A sanitizer that understands the input differently from the consumer
does not sanitize it.

This document is the normative specification. It describes required behavior,
not how to implement it.

Scope: the pipeline accepts one SVG document, produces one SVG document, or
rejects the input. The output must be well-formed XML that conforms to the rules
below.

## §1 Elements

Only elements on the allowlist may appear in the output. The allowlist covers
`svg`, `g`, `defs`, `title`, `desc`, `symbol`, `use`, `path`, `rect`, `circle`,
`ellipse`, `line`, `polyline`, `polygon`, `text`, `tspan`, `image`, `clipPath`,
`mask`, `linearGradient`, `radialGradient`, `stop`, `pattern`.

Element names are compared after normalization: ASCII case-insensitive, with any
namespace prefix removed, and with the prefix itself treated as untrusted (see
§2). An element not on the allowlist is removed together with everything nested
inside it — it is not unwrapped.

- `script` is never allowed, in any spelling, whether written
  `<script>...</script>` or as a self-closing `<script/>`. Both spellings denote
  the same element and must receive the same treatment.
- `foreignObject` is never allowed. Its content is HTML from another namespace
  and none of it may survive into the output.
- `style` is allowed as an element only for the restricted use described in §4;
  its text content is sanitized as CSS rather than copied.
- Element and attribute name comparison is ASCII case-insensitive. A name with
  a namespace prefix is judged by its local name, and a prefixed element whose
  local name is not allowlisted is removed.

## §2 Attributes

Allowlist only. An attribute appears in the output only when its normalized name
is on the allowlist for the element it belongs to. Everything else is removed,
including but not limited to:

- every event handler attribute: any name beginning `on`, in any ASCII case,
  with or without a namespace prefix, and also when a `xmlns:*` declaration on
  the element or an ancestor would bind a prefix used by the attribute;
- any `xmlns` or `xmlns:*` declaration that would re-bind a prefix to a foreign
  namespace;
- namespaced attributes whose prefix is not one of the prefixes the allowlist
  explicitly understands (`xlink` for `href` and `title`);
- unknown attributes of any other name. There is no "unknown attributes are
  harmless" fallback: an allowlist is a closed set.

Normalization for attribute names: strip an optional namespace prefix, then
compare ASCII case-insensitively. An attribute with a prefix that is not
allowlisted is removed regardless of its local name.

Attributes on the allowlist are additionally type-checked:

| Attribute | Type |
|---|---|
| `id` | XML name |
| `class` | space-separated tokens |
| `width`, `height`, `x`, `y`, `x1`, `y1`, `x2`, `y2`, `cx`, `cy`, `r`, `rx`, `ry`, `fx`, `fy`, `fr`, `offset`, `stroke-width`, `opacity`, `fill-opacity`, `stroke-opacity`, `stop-opacity`, `font-size`, `stroke-dashoffset` | length or number, no `url(` |
| `viewBox` | four numbers |
| `preserveAspectRatio` | enumerated keyword |
| `transform` | matrix/translate/scale/rotate/skew keywords only |
| `points` | list of numbers |
| `fill`, `stroke` | color keyword, hex color, `none`, or a reference that passes §3 |
| `clip-path`, `mask`, `filter`, `marker-start`, `marker-mid`, `marker-end` | `url(#local-id)` or `none` only |
| `href`, `xlink:href` | reference passing §3 |
| `title` | plain text, no markup |
| `style` | sanitized by §4 |
| `d`, `path` | path data; no attribute value may contain `<script`, `javascript:`, or a CSS escape |
| `font-family`, `font-weight`, `font-style`, `text-anchor`, `dominant-baseline`, `stroke-linecap`, `stroke-linejoin`, `fill-rule`, `clip-rule` | enumerated keyword |

Type validation operates on the whole attribute value. A failure removes the
whole attribute; an attribute is never truncated to a valid prefix.

## §3 References

Every value that denotes a resource reference (`href`, `xlink:href`, and any
`url(...)` inside an attribute or in CSS) must be an absolute URL or a local
fragment:

- `http:` and `https:` URLs, without embedded credentials and without a
  `javascript:`/`vbscript:` scheme hidden behind whitespace, control characters,
  NULs, HTML character references, percent-encoding, or case variation;
- `data:image/png`, `data:image/jpeg`, `data:image/gif`, and `data:image/webp`
  URLs whose payload is base64 (the media type is followed by `;base64,`);
  `data:text/html` and every other `data:` media type are rejected;
- a fragment reference of the form `#id`.

The decision must be made on the value after it has been interpreted the way the
consumer will interpret it: HTML character references decoded, leading and
trailing ASCII whitespace and control characters stripped, internal tabs and
newlines removed, and the scheme compared ASCII case-insensitively. A
blacklist of a single known-bad prefix is not sufficient and is not the
specification; the rule is the positive list above. Any value whose scheme is
not `http`, `https`, `data` (restricted as above), or the empty scheme of a
fragment is rejected.

## §4 CSS

CSS is sanitized wherever it appears: the `style` attribute and the text content
of a `<style>` element. The sanitizer must parse declarations in a way that
agrees with a browser, including quoted strings, escapes, and nested parentheses.

- Only allowlisted properties survive: `fill`, `fill-opacity`, `fill-rule`,
  `stroke`, `stroke-width`, `stroke-opacity`, `stroke-linecap`,
  `stroke-linejoin`, `stroke-dasharray`, `stroke-dashoffset`, `opacity`,
  `stop-color`, `stop-opacity`, `color`, `font-family`, `font-size`,
  `font-weight`, `font-style`, `text-anchor`, `display`, `visibility`,
  `background`, `background-image`, `background-color`. For the `background*`
  properties the only permitted value forms are colors and `url(...)` values
  that pass §3; `background-image` may not name a remote stylesheet or use
  `image-set()`.
- `@import`, `@media`, `@font-face`, and all other at-rules are removed.
- `expression(`, `-moz-binding`, `behavior:`, `-o-link`, and `-o-link-source` are
  removed.
- Any `url(...)` value must pass §3. The `url` tokenizer must honor quoting and
  CSS escapes; `url('java\73 cript:...')` decodes to a `javascript:` URL and is
  rejected. `@import` appearing inside a quoted string is not an at-rule; an
  at-rule appearing at the start of a declaration is removed.
- CSS escapes (e.g. `\75 rl`, `\00003c`) are decoded before inspection, so an
  escaped spelling of `javascript:`, `expression(`, or `-moz-binding` is caught.
- A declaration whose value cannot be parsed unambiguously is removed rather
  than emitted.
- `<style>` element text may contain several rules; rules outside the allowed
  subset are dropped rule by rule. CDATA wrappers, comments, and HTML comment
  syntax must not be able to smuggle CSS past the parser.

## §5 Prolog, DOCTYPE, and processing instructions

The output is a plain SVG document that begins at the root element. No prolog
content is carried over.

- A DOCTYPE is removed, including any internal subset (`<!ENTITY ...>`,
  parameter entities, conditional sections). Entity expansion must never be
  relied on for safety: any document that defines entities is rejected or
  emitted without the definition and without expanded use.
- Processing instructions (`<?xml ...?>`, `<?xml-stylesheet ...?>`, `<?php ...?>`,
  and any other) are removed.
- Comments may be preserved, but must not contain markup that the consumer
  would treat as content; when in doubt they are removed.

## §6 Tokenization contract

The tokenizer must agree with an XML parser about where a tag ends and where an
attribute value ends. In particular:

- Inside a quoted attribute value, `>` is data, not a tag terminator. The value
  runs to its closing quote.
- An attribute with a quoted value containing `>` is one attribute. It is either
  kept whole or removed whole; it is never split so that one part is inspected
  and the remainder escapes into the output as text.
- The same holds for `'` and `"` quoting, and for unquoted values terminated by
  whitespace or `>`.
- A `<` inside an attribute value does not begin a new tag.
- Malformed input (unterminated quote, unterminated tag) is not repaired into
  something the browser would read differently.

## §7 Output

The output contains only allowlisted elements and attributes, only references
permitted by §3, and only CSS permitted by §4. Nothing in the output can cause
script execution, external stylesheet loading, or navigation when rendered via
`<img src>`.
