# markfence sanitize policy

This document is normative. It defines the input language, the admission rules,
and the observable guarantees of the sanitizing layer. It intentionally describes
behavior only; it does not prescribe an implementation.

The audience is anyone changing the tokenizer, the tree walker, the attribute
filter, the URL filter, the serializer, or the renderer. The appendix is a
regression list: every payload in it must satisfy the guarantees in this
document.

## 1. Scope

markfence accepts the rich-text subset produced by the comment composer: a small
amount of Markdown plus inline HTML. The input is a string. The output is an HTML
fragment that the client inserts into the comment area.

The sanitizer is the only component allowed to decide what survives. The
serializer is the only component allowed to turn the sanitized tree into markup.
The renderer may add highlighting, but it does not relax the policy.

## 2. Output invariants

For every input string `s`, let `out = sanitize(s)`.

1. `out` is a well-formed fragment under the restricted grammar in §3–§6.
2. Every element in `out` is on the tag whitelist, except that the renderer may
   additionally emit `<mark>` as described in §7.
3. No element in `out` carries an event-handler attribute, a `style` attribute,
   a namespaced attribute, or any attribute not admitted by §5.
4. Every URL-bearing attribute in `out` satisfies §5.3.
5. Attribute values in `out` are opaque text: quotes, ampersands, and angle
   brackets in a value never terminate the attribute or create markup.
6. `sanitize(out) === out` (idempotence). Parsing `out` and serializing the
   resulting tree again yields `out`.
7. Sanitizing is deterministic: the same input and options produce the same
   output, independent of unrelated global state.

`out` is a fragment, not a document. It may contain text at the top level.

## 3. Tags

### 3.1 Whitelist

The only admitted element names are:

```text
a  p  strong  em  code  pre  blockquote  ul  ol  li  br  img
```

Element names are ASCII case-insensitive. The serializer writes them in ASCII
lowercase.

### 3.2 Elements removed with their descendants

The following elements are removed together with everything inside them:
`script`, `style`, `iframe`, `object`, `embed`, `applet`, `svg`, `math`, `form`,
`input`, `button`, `select`, `textarea`, `link`, `meta`, `base`, `frame`,
`frameset`, `noembed`, `noframes`, `xmp`, and `plaintext`.

Removing the descendants matters for raw-text and foreign-content elements:
their contents are not ordinary comment text and must not reappear as text.
For example, the text inside `<script>` or `<style>` is discarded, not escaped
and emitted.

### 3.3 Other elements

Any other element not on the whitelist and not listed in §3.2 is unwrapped: the
element itself is removed, its children are sanitized normally, and the
sanitized children take its place in the parent.

Comments, doctypes, processing instructions, and CDATA sections are discarded.

### 3.4 `template` and `noscript`

`<template>` and `<noscript>` are not raw-text elements for the purpose of this
policy. Their contents are ordinary child nodes. Both elements are outside the
whitelist, so they are unwrapped by §3.3 after their children have been
sanitized. An `onerror`, `srcdoc`, `href`, or `src` inside either element is
subject to exactly the same rules as it would be anywhere else.

### 3.5 Void elements

`br` and `img` are void. They are serialized without an end tag. A start tag for
a void element does not open a scope: subsequent content is a sibling, not a
child. An end tag for a void element is ignored.

### 3.6 Nesting

The parser builds a tree from the token stream. End tags close the nearest open
element with the same name; an end tag with no matching open element is ignored.
Elements left open at the end of the input are closed at the end of the
fragment. The sanitizer does not attempt full HTML5 tree-construction
correction; it only needs to produce a tree that round-trips under this policy.

## 4. Text and character references

### 4.1 Text nodes

Text is emitted as text. The serializer escapes `&` and `<` in text. A literal
`<` in comment text therefore remains visible text and never becomes markup.

### 4.2 Character references

Character references are decoded before any policy decision is made. This
includes:

- decimal numeric references such as `&#106;` and `&#x6a;`;
- hexadecimal numeric references, with either case of `x` and either case of
  hex digits;
- the five XML-style named references `&amp;`, `&lt;`, `&gt;`, `&quot;`, and
  `&apos;`;
- other named references relevant to HTML, including legacy forms that omit the
  semicolon, such as `&colon` and `&Tab`.

Decoding happens in text and in attribute values. A character reference must
never be able to hide a scheme, a delimiter, or a tag from a later check. For
example, `java&#115;cript:` is the same input as `javascript:` for the purpose
of §5.3, and `&lt;script&gt;` in text is the same text as `<script>` (which the
serializer then escapes again).

Unknown or malformed references are left as literal text.

## 5. Attributes

### 5.1 Admitted attribute names

The only admitted attribute names are:

```text
href  title  alt  src  width  height  class
```

Attribute names are ASCII case-insensitive. Before the whitelist comparison, an
attribute name is normalized by removing ASCII whitespace and C0 control
characters. A name that is not on the whitelist after normalization is dropped.
The serializer writes admitted names in ASCII lowercase.

### 5.2 Element-specific admission

- `href` is admitted only on `a`.
- `src`, `alt`, `width`, and `height` are admitted only on `img`.
- `title` is admitted on any whitelisted element.
- `class` is admitted on any whitelisted element, subject to §5.5.

An attribute that is not admitted for its element is dropped; the element and
its other attributes remain.

If the same attribute name occurs more than once on an element, only the first
occurrence is considered. Later occurrences are dropped.

### 5.3 URL values

`href` and `src` are URL-bearing. Their values are judged after character
references have been decoded (§4.2) and after the following normalization:

1. Remove ASCII tab (U+0009), line feed (U+000A), and carriage return (U+000D)
   from the entire value. Browsers ignore these characters inside URLs.
2. Trim leading and trailing ASCII whitespace and C0 controls.
3. If any C0 control or DEL remains, the value is unsafe.
4. If the value is empty, the attribute is dropped.

After normalization, the value is classified as follows.

- If it begins with an ASCII letter followed by zero or more ASCII letters,
  digits, `+`, `-`, or `.`, and then a colon, that prefix is a scheme. The
  scheme is compared ASCII case-insensitively. Only `http`, `https`, and
  `mailto` are admitted. Every other scheme is unsafe, including `javascript`,
  `data`, `vbscript`, `file`, and `ftp`.
- Otherwise the value is a relative reference. It is admitted only if it does
  not begin with a network-path or backslash form. Specifically, values
  beginning with `//`, `/\`, `\/`, or `\\` are unsafe. A value beginning with a
  single `/` is a site-relative path and is admitted. Other relative references
  (`posts/1`, `#anchor`, `?q=1`, and so on) are admitted as long as they did not
  match the scheme form above.

An unsafe URL causes the attribute to be dropped. The element itself is not
removed: `<a href="javascript:...">text</a>` becomes `<a>text</a>`.

Examples of admitted URLs include `https://example.com/x`,
`http://example.com`, `mailto:user@example.com`, `/posts/1`, `posts/1`,
`#section`, and `?page=2`. Examples of rejected URLs include `javascript:...`
in any spelling, `data:...`, `vbscript:...`, `ftp://...`, `//evil.example/x`,
`/\evil.example/x`, and any value containing a C0 control after normalization.

### 5.4 URLs on images

The `src` attribute of `img` is URL-bearing and is judged by §5.3 exactly like
`href` on `a`. An image with an unsafe `src` keeps the element and its other
admitted attributes, but loses `src`.

### 5.5 `class`

A `class` value is admitted only if it is a single token and that token begins
with `user-`. The token may contain only ASCII letters, digits, `_`, and `-`
after the prefix. There is no whitespace, comma, quote, or other separator in an
admitted value. Values such as `user-x evil`, `user-x onanimationstart=...`,
`user-`, ` admin`, and `user-x"` are dropped.

The sanitizer does not rewrite a class value; a value that is not admitted is
dropped as a whole.

### 5.6 Other values

`title` and `alt` are text values and may contain arbitrary characters. They are
escaped by the serializer. `width` and `height` are text values in this policy;
they are escaped but not interpreted.

## 6. Serialization

### 6.1 Form

The serializer emits an HTML fragment with:

- lowercase element names;
- double-quoted attribute values;
- no comments, doctypes, or processing instructions;
- no end tags for `br` and `img`;
- an end tag for every other emitted element.

### 6.2 Attribute values

An attribute value is escaped for `&`, `<`, and `"`. The double quote must be
escaped because it is the attribute delimiter. A value containing `"` must not
be able to end the attribute and start a new one, and a value containing `<`
must not be able to start a tag.

Text nodes are escaped for `&` and `<`.

### 6.3 One serialization per tree

A tree is serialized at most once on the way to the client. Do not serialize a
tree, parse the result, and serialize again as part of the normal path; do not
concatenate strings that were produced by separate serializations of the same
tree. The serializer's output must be stable under a parse/serialize round trip:
for any sanitized tree `T`, parsing `serialize(T)` and serializing the parsed
tree yields the same string as `serialize(T)`.

## 7. Highlighting

### 7.1 Text-only matching

Highlighting applies to the visible text of the fragment. The query is matched
case-insensitively against text nodes. Markup, tag names, attribute names, and
attribute values are not searched and are never modified. A query that happens
to equal a tag name, an attribute name, or a delimiter must not split a tag or
an attribute.

### 7.2 Marker element

A match is wrapped in `<mark>`. `mark` carries no attributes. Matches do not
overlap; the renderer must not produce nested `mark` elements. The original text
of a match is preserved exactly (apart from the wrapper).

### 7.3 Re-application of the policy

Highlighting produces a new tree. That tree is serialized under the policy in
§3–§6, with `mark` as the only additional admitted element. The resulting
fragment must satisfy §2 with respect to the whitelist plus `mark`: no other
tag, no event attributes, no unescaped attribute values, and idempotence under a
second application of the same pipeline.

The renderer does not accept a fragment that has bypassed the sanitizer and
return it unchanged. If the fragment contains content that the policy would
remove, the highlighted result contains the sanitized form of that content, not
the original.

### 7.4 Empty query

If the query is not a string or is empty, the fragment is returned after the
normal sanitizing pipeline, without `mark` elements.

## 8. Regression payloads

The following table is the regression list for this policy. Each row is a
payload that must satisfy §2 after `sanitize()`, and again after a second
`sanitize()` of the result. "Removed" means the named attribute is absent from
the output. "Dropped" means the element and its descendants are absent.

| # | payload | expected result |
|---|---|---|
| P1 | `<script>alert(1)</script>` | script dropped |
| P2 | `<img src="/a.png" onerror="alert(1)">` | onerror removed, img and src kept |
| P3 | `<a href="javascript:alert(1)">x</a>` | href removed, a and text kept |
| P4 | `<a href="java&#9;script:alert(1)">x</a>` | href removed |
| P5 | `<a href="java&Tab;script:alert(1)">x</a>` | href removed |
| P6 | `<a href="java&colon;script:alert(1)">x</a>` | href removed |
| P7 | `<a href="&#106;avascript:alert(1)">x</a>` | href removed |
| P8 | `<a href="&#x6a;avascript:alert(1)">x</a>` | href removed |
| P9 | `<a href="&#0;javascript:alert(1)">x</a>` | href removed |
| P10 | `<a href="//evil.example/x">x</a>` | href removed |
| P11 | `<a href="/\evil.example/x">x</a>` | href removed |
| P12 | `<a href="/posts/1">x</a>` | href kept as `/posts/1` |
| P13 | `<img src="javascript:alert(1)">` | src removed, img kept |
| P14 | `<img src="data:image/svg+xml;base64,PHN2Zz4=">` | src removed, img kept |
| P15 | `<img src="//evil.example/x">` | src removed, img kept |
| P16 | `<img src="/\evil.example/x">` | src removed, img kept |
| P17 | `<img src="/img/a.png" alt="A" width="100" height="50">` | all four attributes kept |
| P18 | `<a href="/x" title="a&quot; onmouseover=&quot;alert(1)">x</a>` | title is one escaped value; no onmouseover attribute |
| P19 | `<p class="user-x onanimationstart=alert(1)">x</p>` | class removed |
| P20 | `<p class="user-tip">x</p>` | class kept as `user-tip` |
| P21 | `<template><img src="x" onerror="alert(1)"></template>` | template unwrapped, onerror removed, img kept |
| P22 | `<noscript><img src="x" onerror="alert(1)"></noscript>` | noscript unwrapped, onerror removed, img kept |
| P23 | `<svg><script>alert(1)</script><a href="javascript:alert(1)">x</a></svg>` | svg dropped with descendants |
| P24 | `<iframe srcdoc="&lt;script&gt;alert(1)&lt;/script&gt;"></iframe>` | iframe dropped |
| P25 | `<math><mtext><img src="x" onerror="alert(1)"></mtext></math>` | math dropped with descendants |
| P26 | `<form action="javascript:alert(1)"><input value="x"></form>` | form dropped with descendants |
| P27 | `<a href="mailto:user@example.com">mail</a>` | href kept |
| P28 | `<a href="/x?a=1&amp;b=2">x</a>` | href kept with a single `&amp;` in the output |
| P29 | `<a href="ftp://example.com/x">x</a>` | href removed |
| P30 | `<a href="data:text/html,<script>alert(1)</script>">x</a>` | href removed; the `<script>` inside the value never becomes an element |
| P31 | `<p title="&lt;img src=x onerror=alert(1)&gt;">x</p>` | title is escaped text; no img element is created |
| P32 | `<img src="x" onerror="alert(1)" onload="alert(2)">` | both handlers removed |
| P33 | `<blockquote><p>quote</p></blockquote>` | structure kept |
| P34 | `<ul><li>one</li><li>two</li></ul>` | structure kept |
| P35 | `<pre><code>if (a &lt; b) go();</code></pre>` | structure kept, `&lt;` remains escaped |
| P36 | `<a href="/x" class="user-link" title="ok">x</a>` | all three attributes kept |
| P37 | `<p>hello <em>world</em> <strong>again</strong><br>next</p>` | structure kept |
| P38 | `<a href="&#x2f;&#x2f;evil.example/x">x</a>` | href removed |
| P39 | `<a href="&#92;&#92;evil.example/x">x</a>` | href removed |
| P40 | `<a href="?page=2#top">x</a>` | href kept |

Rows P18, P30, and P31 are also the check for §6.2: the output must be
parseable without creating a new attribute or element from the value.

The list is frozen. Changes to the tokenizer, the attribute filter, the URL
filter, the tree walker, the serializer, or the renderer must keep every row
passing.
