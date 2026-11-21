# URL construction

This document is normative. It defines how an integration's base URL and path
template are combined with event fields to produce the request target that is
sent to a tenant endpoint.

## 1. Inputs

* **Base URL** — the endpoint root configured on the integration.
* **Path template** — a tenant-supplied path such as
  `/v1/orders/{order_id}/events`.
* **Event fields** — a mapping of field names to values produced by the event
  platform. Values may be strings, numbers, booleans, or null, and are not
  trusted to be URL-safe.

Neither the template nor any field value may be assumed to be printable ASCII,
to be free of delimiters, or to be non-empty.

## 2. Path construction

### 2.1 Template variables

A template variable is written `{name}` where `name` matches
`[A-Za-z_][A-Za-z0-9_]*`.

Each variable occupies **exactly one complete path segment**. A variable must
therefore be bounded by `/` or by the beginning or end of the template path; a
template that places a variable inside a larger literal segment (for example
`/files/{name}.json` or `/files/prefix{name}`) is invalid.

The substituted value is treated as text, encoded to UTF-8, and then
percent-encoded **as a whole segment**:

* The only bytes emitted verbatim are `A-Z`, `a-z`, `0-9`, `-`, `_`, and `~`.
* Every other byte is emitted as `%` followed by two **uppercase** hexadecimal
  digits.
* `.` is deliberately not in the safe set, so a value can never render as the
  special segments `.` or `..`.
* In particular `/` becomes `%2F`, `?` becomes `%3F`, `#` becomes `%23`,
  `%` becomes `%25`, space becomes `%20`, and `..` becomes `%2E%2E`.
* A value that is missing, null, or the empty string does not produce a valid
  segment; such a request is invalid rather than being sent with an empty or
  collapsed segment.

Percent-encoding happens once, on the field value. Literal template text is
not re-encoded, and a `%` that is part of a field value is encoded as data
(`%25`), never treated as the start of an escape.

### 2.2 Template form

* The template path must begin with `/`.
* The path template must not contain `?` or `#`. Query parameters declared by
  an integration are covered by `docs/query.md`.
* Literal segments must be non-empty and must not be `.` or `..`.
* Every `{` must have a matching `}`. An unknown variable name is invalid.
* A variable value is data for exactly one segment. It must not be able to
  introduce a separator, a query string, a fragment, or an authority.

### 2.3 Base URL

The base URL must be an absolute `http` or `https` URL. It must not contain
userinfo, a query string, or a fragment.

The base URL's path is a **prefix**, not a file name. The rendered template
path is appended to that prefix:

* an empty prefix or `/` contributes a single leading `/`;
* a prefix with a trailing `/` keeps exactly one separator;
* a prefix without a trailing `/` gets one separator inserted before the
  rendered template path.

For example, with base `https://hooks.example/tenant-a` and template
`/v1/orders/{order_id}/events`, the result is
`https://hooks.example/tenant-a/v1/orders/<encoded>/events`.

### 2.4 Containment

The final path must be the literal concatenation described in §2.3. No
dot-segment removal, path collapsing, or authority replacement may change
which resource is addressed.

A rendered template that would do any of the following is invalid and must be
rejected before a connection is opened:

* start with `//` or otherwise change the URL authority;
* introduce a `.` or `..` segment at any level;
* introduce an empty segment that changes the path hierarchy;
* escape, truncate, or replace the base path prefix;
* introduce a `?` or `#` delimiter that was not present as literal template
  syntax.

An encoded slash (`%2F`) is part of a segment's data. It does not create a new
path level.

### 2.5 Fragments

A fragment is never transmitted. Any `#` originating in a field value is data
and must be percent-encoded before it reaches the request target.

## 3. Result

The output of path construction is a URL whose scheme, authority, and base
path prefix come only from the base URL, and whose path is the base prefix plus
one encoded segment per template element. The URL may still carry the literal
query component declared by the path template; merging it with runtime
parameters is governed by `docs/query.md`.