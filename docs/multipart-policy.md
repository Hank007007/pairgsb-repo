# Multipart parsing policy

This document is normative. It describes what the gateway accepts, how part
metadata is normalised, and what happens when a request is outside the policy.
It deliberately contains no implementation guidance.

## §1 Scope and disposition types

1.1 A request is only treated as multipart if it carries a `multipart/form-data`
media type with a `boundary` parameter.

1.2 Each part must carry a `Content-Disposition` header. Its disposition type
must be exactly `form-data` or `file` (case-insensitive). Any other type is a
client error.

1.3 The disposition parameters `name` and `filename` are the only metadata the
gateway forwards. A file part is a part whose disposition carries a `filename`
parameter; a part without one is a plain field and has no filename.

## §2 Part header blocks

2.1 A part header block is the byte sequence between the header delimiter that
follows the boundary line and the empty line that ends it. It consists of
`name: value` lines.

2.2 Header field names are case-insensitive and are normalised to lower case.
A field name is a non-empty run of visible ASCII characters excluding `:`,
`(`, `)`, `,`, `/`, `;`, `<`, `=`, `>`, `?`, `@`, `[`, `\`, `]`, `{`, `}`,
space and tab. A line whose name does not satisfy this is not a valid header
line.

2.3 Obsolete line folding is supported: a header line beginning with a space
or a horizontal tab is a continuation of the preceding header line. Folding is
unfolded by replacing the line break and the leading whitespace with a single
space, so that a folded header is reconstructed as one complete value. A
continuation line that does not follow a header line is malformed.

2.4 Field values must not contain bare CR, bare LF, or NUL. A header line with
an empty name, a missing colon, or a forbidden character is malformed.

2.5 A malformed header block rejects the whole request with a client error.
The gateway does not silently drop malformed lines.

## §3 Disposition parameter grammar

3.1 The disposition value is `type` followed by zero or more
`; name=value` parameters.

3.2 A parameter value is either a token or a quoted-string. Inside a
quoted-string, `;` and `=` are ordinary characters and do not terminate the
parameter. Backslash escapes an immediately following character inside a
quoted-string. Splitting the value on `;` or `=` without regard to quoting is
not permitted.

3.3 Parameter names are case-insensitive. When a parameter occurs more than
once the last occurrence wins, except that an extended parameter (`name*`)
takes precedence over its plain counterpart (`name`) when both are present.

3.4 Values are otherwise preserved byte for byte, with one exception: a
`filename` value is always percent-decoded, whichever parameter form supplied
it, before it reaches path normalisation. A client that sends
`filename="a%2fb.png"` must not end up with a literal `%2f` in the stored name,
and `%00` must be decoded to NUL so that the filename rules in §5 can reject
it. A `%` that is not followed by two hexadecimal digits is malformed. The
`name` parameter is not percent-decoded.

## §4 Extended parameter values (`filename*`, `name*`)

4.1 An extended parameter value has the form
`charset'language'percent-encoded-value`.

4.2 `charset` is `UTF-8` or `ISO-8859-1` (case-insensitive). `language` may be
empty and is only informational. Unknown charsets reject the part with a
client error.

4.3 The encoded value is decoded with percent-decoding and then interpreted
using the declared charset. A percent escape must be two hexadecimal digits;
a malformed escape rejects the part.

4.4 The decoded value is subject to the same restrictions as any other
disposition parameter. The extended form is what the client intends the
filename to be; it is the value that reaches path normalisation.

## §5 Filename normalisation and the storage path

5.1 A client filename is untrusted. The gateway derives a tenant-relative
storage path from it and never returns a path that could escape the tenant
directory.

5.2 Both `/` and `\` are path separators for this purpose, as are the Unicode
division slash (U+2215) and fullwidth solidus (U+FF0F). A filename is
rejected if, after normalisation, it contains a `..` path segment, is absolute
(leading separator, leading drive letter such as `C:`, or UNC prefix), or
contains a NUL byte.

5.3 Control characters, NUL in particular, are never allowed in a filename. A
NUL must not be used to truncate a name into something shorter than what the
client sent.

5.4 Windows reserved device names (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`,
`LPT1`–`LPT9`, including a trailing extension or trailing dot/space) are
rejected on every platform, because storage is shared.

5.5 Empty names, names consisting only of separators, and names whose final
segment is `.` or `..` are rejected.

5.6 Rejection is reported to the caller as an error carrying the reason; the
gateway does not invent a replacement name.

## §6 Framing

6.1 The first delimiter in the body is `--boundary`. Each subsequent delimiter
is `CRLF--boundary`. A `--boundary` character sequence that is not preceded by
CRLF (or, for the first one, at the very start of the body) is body content,
not a delimiter. Body content may legitimately contain the boundary substring.

6.2 A delimiter is followed either by `CRLF`, which begins the next part, or by
`--`, which closes the multipart body. The closing delimiter may be followed by
optional whitespace and then `CRLF`; anything after that is epilogue.

6.3 Epilogue is discarded. It is never part of any part's body.

6.4 The body of a part is exactly the bytes between the line break that
follows its header block and the CRLF that precedes the next delimiter.

6.5 A request that ends without a closing delimiter is truncated and is
rejected.

## §7 Boundary validation

7.1 The boundary is 1 to 70 characters long. Empty or longer boundaries are a
client error.

7.2 Boundary characters are the RFC 2046 `bcharsnospace` set: digits, letters,
and `` '()+_,-./:=? `` plus space, provided the boundary neither begins nor
ends with a space. A boundary containing other characters — or one quoted in
the header in a way that does not match this set — is a client error.

7.3 Boundary validation happens before any part is produced, so an invalid
boundary yields no parts.

## §8 Request budgets

8.1 A single request may carry at most the configured number of parts
(default 64). Exceeding it rejects the whole request with HTTP 413.

8.2 The total number of bytes read for a request may not exceed the configured
total (default 8 MiB). Exceeding it rejects the whole request with HTTP 413.

8.3 The sum of all part header blocks may not exceed the configured header
budget (default 16 KiB). Exceeding it rejects the whole request with HTTP 413.

8.4 A single part body may not exceed the configured per-part ceiling
(default 4 MiB). Exceeding it rejects the whole request with HTTP 413.

8.5 Budgets are hard: a rejected request produces an empty `parts` array. No
part that caused or followed a budget breach is returned to the caller. The
error carries the 413 status and a reason code.

8.6 A request rejected by any rule in this document returns no parts. There is
no partial success.
