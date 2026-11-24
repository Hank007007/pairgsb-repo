# Policy grammar

A tenant policy is either a JSON document or a single line of text. Both forms
describe the same thing: a list of directives, each of which carries a list of
source expressions.

```
policy      = directive *( ";" directive )
directive   = name [ RWS source *( RWS source ) ]
name        = ALPHA *( ALPHA / DIGIT / "-" )
source      = keyword / nonce-source / hash-source / scheme-source / host-source
```

Directive names are matched case-insensitively; the grammar above is written in
lower case for readability only.

## 1. Input forms

### 1.1 JSON documents

A JSON policy is an object whose keys are directive names and whose values are
either a string (the source list, whitespace separated) or an array of strings
(one entry per source expression). An empty array means "this directive is
present with no sources".

### 1.2 Text policies

A text policy is the serialized form described above. Producers frequently put
several directives on one line, so a single line may contain more than one
directive.

## 2. Lexical structure

### 2.1 Separators

Whitespace is any run of U+0020 SPACE, U+0009 TAB, U+000B VERTICAL TAB,
U+000C FORM FEED, U+000A LINE FEED, U+000D CARRIAGE RETURN, and U+3000
IDEOGRAPHIC SPACE. A run of whitespace separates two tokens; it never belongs
to a token.

`;` separates directives. The semicolon is not part of either directive; it is
consumed as the delimiter and any whitespace around it is insignificant.

Carriage return and line feed are whitespace for the purposes of splitting, and
are never carried into a token. They separate source expressions exactly like a
space does; they do not by themselves start a new directive.

### 2.2 Directive identity

Directive names are case-insensitive: `Script-Src`, `script-src` and
`SCRIPT-SRC` all denote the same directive. When a policy contains more than
one occurrence of the same directive, the occurrences are combined into a
single directive whose source list is the union of every occurrence's sources,
with duplicates removed. Later occurrences never discard earlier ones.

Source expressions are case-sensitive for everything that names a resource:
hosts, schemes and paths compare exactly, so `https://cdn.example` and
`https://CDN.example` are different sources and both are kept.

Quoted keywords are the exception: they are matched case-insensitively and are
normalized to their lower-case spelling before they are stored or compared.
`'SELF'`, `'Self'` and `'self'` therefore all denote the same source and
appear once in the output, as `'self'`.

### 2.3 `'none'`

`'none'` is a source expression that means "nothing is allowed". It is only
valid when it is the sole source expression of its directive. A directive that
lists `'none'` together with any other source expression is invalid.

### 2.4 Empty directives

A directive with no sources is valid. It contributes no source expressions of
its own; what it means in the merged policy is decided in `docs/merge.md`.

## 3. Source expression forms

### 3.1 Keywords, nonces and hashes

Keywords are the quoted tokens `'none'`, `'self'`, `'unsafe-inline'`,
`'unsafe-eval'`, `'strict-dynamic'`, `'unsafe-hashes'`, `'wasm-unsafe-eval'`,
`'report-sample'` and `'inline-speculation-rules'`. A quoted token that is not
one of these is not a keyword.

A nonce source is `'nonce-` followed by a base64-value followed by `'`. The
base64-value is one or more characters from `A-Z a-z 0-9 + / - _`, optionally
followed by `=` padding, and must be valid base64: the characters must be in
the alphabet, padding may only appear at the end, and the total length must be
a multiple of four when padding is present. The alphabet accepts the
URL-safe variants `-` and `_` in place of `+` and `/`.

A hash source is `'` followed by an algorithm followed by `-` followed by a
base64-value followed by `'`. The algorithm is `sha256`, `sha384` or `sha512`,
in lower case. The base64-value follows the same rule as for nonces.

### 3.2 Scheme sources

A scheme source is a scheme followed by `:` — for example `https:` or `data:`.
The scheme is `ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )` and is matched
case-insensitively.

The platform only serves sources over `https`. A scheme source of `https:` is
accepted; every other scheme source — `http:`, `data:`, `blob:`, `filesystem:`,
`javascript:` and so on — is rejected. A scheme that appears in a host source
is subject to the same rule: `https://example.com` is accepted,
`http://example.com` and `ftp://example.com` are not.

## 4. Host sources

### 4.1 Shape

A host source is an optional scheme, followed by a host, followed by an
optional port and an optional path.

```
host-source = [ scheme "://" ] host [ ":" port ] [ path ]
scheme      = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )
host        = [ "*." ] hostname
port        = 1*DIGIT / "*"
path        = "/" *( ... )
```

### 4.2 Wildcard hosts

The only wildcard in a host is the leading `*.` label, which matches the host
itself and any subdomain of it. Wildcards in the path follow the usual CSP
rules: a path that ends in `/` is a prefix match, a path that ends in `*` is a
prefix match on everything before the `*`, and an exact path is an exact match.

### 4.3 Host constraints

* The host part is a hostname, optionally prefixed by `*.`. A bare `*.` with no
  hostname after it is not a valid host.
* A scheme-relative source such as `//example.com/x` (two slashes, no scheme)
  is not a valid source expression. The scheme, when present, must be followed
  by `://`, and a source that starts with `//` has no scheme and is rejected.
* The port is either a run of digits or the single character `*`. A port that
  contains anything else — a second colon, letters, an empty value — is not a
  valid port.
* Wildcards are only recognised as the leading `*.` label. A `*` anywhere else
  in the hostname, or in the middle of a label, is not special.

## 5. Error reporting

A policy that violates any rule above is rejected as a whole. The rejection
names the directive it belongs to and the offending source expression, so an
operator can point at the field in the console that needs to change.

Validation is applied to every entry point that accepts a policy — the text
form, the JSON form, and any policy that is about to be merged with the
baseline.
