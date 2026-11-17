# sidemount mount rules

This document is normative. It states what a conforming `sidemount` instance
must do. It deliberately does not prescribe an implementation; any design that
satisfies every clause below is acceptable.

## §1 — Scope

`sidemount` is an injection sidecar that runs next to a workload container.
It obtains a set of secrets from a credential broker on behalf of a service
identity, writes them under a target directory, and keeps them current for as
long as the process lives. The workload reads the resulting files and never
talks to the broker itself.

The broker is the only source of secret material. A bundle consists of a lease
identifier, an optional lease lifetime, and a mapping from secret name to
secret value.

## §2 — Names

A secret name is admissible only if it matches:

```
[a-z][a-z0-9_]{0,63}
```

That is: a lowercase ASCII letter, followed by zero to sixty-three lowercase
ASCII letters, digits, or underscores; at most 64 characters in total.

Names that do not match are skipped. A skipped name must not produce a file,
must not replace or remove an existing file, and must not abort the rest of the
bundle. Every admissible name maps to exactly one file whose basename is the
name itself.

## §3 — Permissions

- The target directory must be mode `0700` (owner read/write/execute only).
- Every materialized file must be mode `0400` (owner read only).
- The target directory is dedicated to `sidemount`; no other process writes
  into it.
- Permissions must be the effective mode of the file or directory as observed
  on disk. Modes must not be left to the ambient umask, and files must not be
  left writable by any principal.
- Changing the secret value must not widen permissions.

## §4 — Target path integrity and materialization

For every admissible name, the final path is `<target-dir>/<name>`.

- Before a secret is placed at its final path, the process must establish that
  the path is either absent or a regular file. If the path exists and is a
  symbolic link, a directory, a device node, a FIFO, a socket, or any other
  non-regular file, the whole materialization pass fails.
- Failing the pass means no file in that pass is created, replaced, removed,
  or modified — not the offending entry and not any other entry. In
  particular, no byte may be written through a symbolic link: the object the
  link resolves to must be left exactly as it was.
- A secret value must be written to a temporary file in the target directory
  and then moved into place with a single atomic rename. A reader that opens
  the path must see either the previous complete value or the new complete
  value, never a partial write.
- Temporary files must not remain in the target directory after a pass, and
  must never be visible under an admissible name.
- On a failed pass, any temporary file created during that pass is removed.

## §5 — Lease renewal

A lease is renewed before it expires, and a renewal may also be requested
explicitly. Renewal semantics:

- A renewed bundle replaces the contents of the mounted files. Names that are
  present in both the previous and the renewed bundle keep their file name.
- A name that was mounted from an earlier bundle and is absent from the
  renewed bundle is removed from the target directory before the renewal is
  considered complete.
- Names skipped under §2 are treated as absent for this purpose: they are not
  created, and no existing file with such a name may be touched.
- If renewal fails, the previously mounted set must remain intact and usable.
  A failed renewal must not leave the target directory partially updated or
  with temporary files behind.
- After a renewal, the in-memory set of mounted secret names equals the set of
  admissible names in the bundle that was accepted.

## §6 — Process lifecycle

The sidecar reads newline-delimited commands from stdin:

- `renew` — fetch a fresh bundle and apply it under §5. A success or failure
  line is written to stdout.
- `stop` — finish: the process must shut down through the shutdown sequence
  described below, not through a separate path.
- Any other non-empty line is ignored with a diagnostic on stdout.

Lease lifetime:

- If the granted bundle carries a lifetime, the sidecar renews on its own when
  that lifetime has elapsed, then arms the next renewal from the lifetime of
  the bundle it receives. Self-renewal uses the same path as an explicit
  `renew`.
- Self-renewal continues for the lifetime of the process.

Shutdown:

- On `stop`, on `SIGTERM`, and on stdin EOF, the process performs the same
  shutdown sequence exactly once.
- Shutdown removes every regular file in the target directory, so that a
  subsequent process, or a volume reused by another container, does not
  inherit secret material. If an entry is not a regular file, it must not be
  followed or written through.
- The process exits only after cleanup has completed, with status `0` for
  `stop` and stdin EOF. An unrecoverable startup failure exits non-zero.
- Files are not removed on a transient broker error; a failed renewal is
  reported and the process stays up.

## §7 — Observability

Startup, successful renewal, failed renewal, and shutdown are reported on the
process streams. Diagnostics must not include secret values.
