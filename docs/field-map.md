# Field map

Only the paths in this table are readable from an expression. Any other path
is denied (`AccessDenied`). Paths are case-sensitive. Fields not listed here
are internal even when they happen to exist on the underlying object.

## `user`

| Path | Type | Meaning |
|---|---|---|
| `user.id` | string | stable user identifier |
| `user.plan` | string | subscription plan slug |
| `user.role` | string | primary role slug |
| `user.email` | string | e-mail address |
| `user.age` | integer | age in years |
| `user.verified` | boolean | whether the account is verified |
| `user.tenant_id` | string | tenant that owns the account |
| `user.groups` | list of strings | group slugs the user belongs to |

## `request`

| Path | Type | Meaning |
|---|---|---|
| `request.path` | string | request path, beginning with `/` |
| `request.method` | string | upper-case HTTP method |
| `request.host` | string | `Host` header value |
| `request.ip` | string | client IP address |
| `request.headers` | map of string to string | request headers |

## Notes

* There is no index or subscript syntax, so entries of `request.headers`
  cannot be read individually. Use `contains` on a value that is already a
  scalar field instead.
* A path with more than the listed segments, an empty segment, or an
  underscore-prefixed segment is never valid.
* The list of allowed paths is closed. Adding a path requires a change to
  this document.
