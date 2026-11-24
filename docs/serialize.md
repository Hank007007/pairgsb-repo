# Serializing the header

The compiler produces at most one `Content-Security-Policy` header per
response. This document describes how the merged policy becomes that header.

## 1. One header

The merged policy is serialized into a single header value. Directives are
joined with `"; "` (semicolon followed by a single space). Within a directive,
the name and its sources are joined with single spaces.

## 2. Directive order

Directives are emitted in a fixed order, independent of the order in which the
tenant or the baseline defined them. The order is:

1. `default-src`
2. `script-src`, `script-src-elem`, `script-src-attr`
3. `style-src`, `style-src-elem`, `style-src-attr`
4. `img-src`, `font-src`, `connect-src`, `media-src`
5. `object-src`, `frame-src`, `child-src`, `worker-src`, `manifest-src`,
   `prefetch-src`
6. `base-uri`, `form-action`, `frame-ancestors`
7. `sandbox`, `upgrade-insecure-requests`, `block-all-mixed-content`
8. `require-trusted-types-for`, `trusted-types`
9. `report-uri`, `report-to`

`default-src` is always first. Directives not named in the list are emitted
after the known ones, in ascending name order, so the output is deterministic.

## 3. Size and character limits

* A single header value must not exceed 4096 bytes when encoded as UTF-8. If
  the serialized policy would exceed that, the compiler refuses to produce a
  header and reports the oversize policy instead of truncating it.
* A header value may not contain control characters. The only characters
  permitted between tokens are the space, the semicolon and the comma; every
  other C0 control (`U+0000`–`U+001F`), `U+007F` DEL, and in particular
  carriage return and line feed, is stripped before the value leaves the
  serializer.
* The serializer is the last line of defence: even if a source expression
  reached it with a stray `;` or a newline inside, the emitted value must not
  contain it.

## 4. What the header does not include

The `Reporting-Endpoints` header is a separate header. It is built from the
validated endpoint, not from the policy text, and is described in
`docs/reports.md`.
