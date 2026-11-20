# Secret resolution

This document is the specification for how `refgate` turns secret references
into values. It describes required behaviour only.

## 1. Reference syntax

A secret reference is a string with the form:

```
secret://<scheme>/<path>
```

- `<scheme>` is one of `kv`, `env`, `file`.
- `<path>` is non-empty and uses `/` as its separator.
- Whitespace around the whole reference is insignificant.

Anything that does not match this form is not a reference and is treated as a
literal configuration value.

## 2. Dispatch

References are resolved through the provider registry.

- §2.1 The scheme decides the provider. Only `kv`, `env` and `file` are valid
  schemes.
- §2.2 A reference whose scheme has no registered provider must fail with an
  error. The loader must not fall back to another provider and must not touch
  the filesystem for such a reference.
- §2.3 Resolution failures are errors. A reference that cannot be resolved
  must not be replaced by an empty string, a zero value or a placeholder; the
  load fails.

## 3. Providers

### 3.1 `kv`

Reads from a JSON firmware file of the shape
`{"<scope>": {"<path>": "<value>"}, ...}`. The requested scope is consulted
first, then `default`. A missing key is a resolution error.

### 3.2 `env`

Reads the process environment variable named by the path. An unset variable is
a resolution error. The value is used verbatim.

### 3.3 `file`

Reads a file relative to an explicitly configured whitelist of directories.

- §3.3.1 Only directories passed to the provider at construction time are
  readable. There is no implicit current-directory root.
- §3.3.2 A path that escapes every whitelisted directory — via `..`, an
  absolute path, a symlink, or any other means — is an error. The file must not
  be read.
- §3.3.3 The search order is the order the roots were configured. A missing
  file is a resolution error.

## 4. Interpolation

`${NAME}` placeholders are expanded using the `Vars` supplied to the load.
An unknown or unterminated placeholder is an error.

- §4.1 Interpolation applies to the raw documents only, exactly once.
- §4.2 A value produced by resolving a reference must never be interpolated.
  A `${...}` sequence that arrives as part of a resolved secret is preserved
  verbatim. This prevents a secret value from pulling unrelated configuration
  into a field it does not belong to.

## 5. Merge

The effective document is the base document with the environment overlay
applied on top.

- §5.1 The overlay has higher precedence. When both documents define a key,
  the overlay's value is the effective one, including when the overlay value is
  a reference.
- §5.2 An overlay that omits a key leaves the base value in place.
- §5.3 List-valued keys do not accumulate duplicates: applying the same
  overlay repeatedly, or an overlay that repeats a base element, yields a list
  in which each element appears once. Order follows first appearance: base
  elements first, then new overlay elements.
- §5.4 Keys present only in the overlay are added.

## 6. Dump

`Dump` produces a log-safe rendering of the effective configuration.

- §6.1 Every value that came from a secret reference is replaced by a
  placeholder, under any key name and in any scheme.
- §6.2 Values under conventionally sensitive keys — including `api_key`,
  `token`, `dsn`, `password`, `bearer`, `secret` and their dotted or
  underscore-suffixed variants — are replaced by a placeholder even when they
  were written literally.
- §6.3 No plaintext secret may be reachable through any accessor exposed for
  logging, including the raw document accessor. Access to plaintext is an
  explicit, separately named operation, not a default.
- §6.4 Non-sensitive literal values are rendered as-is.

## 7. Cache

The resolver may cache provider results.

- §7.1 Cache entries are keyed by everything that can change the result:
  the reference (scheme included) and the environment scope. Two scopes that
  contain the same reference text must not share a value.
- §7.2 Failures are not cached. After a failed resolution, the next attempt
  consults the provider again.
- §7.3 Entries expire after the configured TTL and are re-resolved on the next
  access. The TTL applies uniformly to every entry that is stored.
