# filesentry broker — design

Status: normative. This document describes how the broker is supposed to behave.
It does not describe how the behaviour is implemented.

## §1 Purpose and scope

`filesentry` is a privileged daemon that performs file operations on behalf of
unprivileged business processes. It listens on a Unix domain socket, accepts a
single request per connection, executes the request with the daemon's own
credentials, and writes a single response back.

The broker exists so that business processes never need `CAP_DAC_OVERRIDE`,
`CAP_CHOWN`, or root. Everything the broker does on a caller's behalf is
bounded by the policy in the access file; nothing else in the system is in
scope.

Two words are used with precise meaning throughout this document:

* **caller** — the process at the other end of a connection.
* **target** — the file, directory, or symlink that a request names.

## §2 Trust model

* Business processes are untrusted. They control every byte they send: the
  operation name, the path, the payload, and the numeric ids in a `chown`
  request.
* The broker is trusted. It runs with elevated privileges and is assumed to be
  correctly configured by the operator.
* The **only** acceptable evidence of a caller's identity is the credential the
  kernel attaches to the connected socket. That credential is authoritative:
  it cannot be produced, altered, or influenced by the caller.
* Identity MUST NOT be taken from anything the caller or its environment can
  control, including but not limited to: the request payload, environment
  variables, command line arguments, file contents, or the socket path.
* If the kernel credential is unavailable for a connection, the broker MUST
  refuse the connection. It MUST NOT guess, default, or continue with a
  partially established identity.
* The access file, the socket path, and the daemon's startup environment are
  operator-controlled inputs. They are trusted for configuration, but they are
  never evidence about a caller.

## §3 Transport and wire protocol

The transport is a Unix domain stream socket. One request and one response are
exchanged per connection; the connection is closed by the broker after the
response is written.

Messages are JSON objects encoded as a single line terminated by `\n`. The
field names and their order in the Go structs are part of the protocol and are
frozen.

### Request

| field     | type              | meaning                                              |
|-----------|-------------------|------------------------------------------------------|
| `version` | integer           | protocol version, currently `1`                      |
| `op`      | string            | `read`, `write`, `list`, or `chown`                   |
| `path`    | string            | absolute target path                                 |
| `data`    | base64 / bytes    | payload for `write`                                  |
| `mode`    | integer           | permission bits for a newly created file             |
| `uid`     | integer           | desired owner for `chown`                            |
| `gid`     | integer           | desired group for `chown`                            |

### Response

| field     | type              | meaning                                              |
|-----------|-------------------|------------------------------------------------------|
| `version` | integer           | protocol version of the responder                    |
| `ok`      | boolean           | `true` if and only if the operation completed        |
| `data`    | base64 / bytes    | file contents for `read`                             |
| `entries` | array of entries  | directory entries for `list`                         |
| `reason`  | string            | machine-readable reason code (see §8)                |
| `error`   | string            | human-readable detail, never machine-parsed          |

An entry has `name`, `is_dir`, `size`, `mode`, `uid`, and `gid`.

A request whose `version` is not supported, whose `op` is unknown, or whose
framing cannot be decoded MUST be answered with `ok=false` and
`reason=invalid_request`, and the connection MUST be closed.

## §4 Configuration

The access file is a JSON document containing a single `rules` array. Each
element has exactly:

```json
{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "list"]}
```

* `uid` is the numeric uid of the caller the rule applies to.
* `paths` is a list of absolute path patterns (§5).
* `ops` is a list of operation names from `read`, `write`, `list`, `chown`, or
  the single special value `deny`. `deny` means "nothing below this pattern is
  permitted to this uid, under any circumstance".

Rules from different uids never interact. Rules are additive within a uid: a
rule naming several patterns applies all of them, and a rule naming several ops
grants all of them.

The access file is read once at startup. A malformed file, an unknown op, a
non-absolute pattern, or a pattern containing `.` or `..` segments is a startup
error; the daemon MUST NOT start with a partially loaded policy.

## §5 Path patterns and rule selection

### §5.1 Patterns

Patterns are absolute, slash-separated, and matched against a lexically
normalised request path:

* A request path is normalised before matching: repeated slashes collapse,
  `.` segments are removed, `..` segments are resolved lexically, and a
  trailing slash is dropped. A path that is not absolute after normalisation
  is rejected as `invalid_request`.
* Matching is case-sensitive and compares whole path segments. A pattern never
  matches a name merely because the name begins with the same characters:
  `/srv/tenants/acme/**` MUST NOT match `/srv/tenants/acme-evil/x` or
  `/srv/tenants/acmex`.
* `*` matches exactly one path segment, possibly empty. It MUST NOT match a
  `/`. `**` matches zero or more consecutive segments.
* A pattern that ends in a literal segment names both that directory and
  everything beneath it. `/srv/tenants/acme` therefore means the same set of
  paths as `/srv/tenants/acme/**`.
* `..` in a request path is resolved before matching, so
  `/srv/tenants/acme/../beta/x` is evaluated as `/srv/tenants/beta/x` and is
  never covered by a rule for `/srv/tenants/acme/**`.
* Wildcards are matched against the normalised request path only. They are not
  matched against resolved symlink targets; containment after resolution is
  governed by §6.

### §5.2 Selection

For a `(uid, path)` pair the broker considers every rule of that uid whose
patterns match the path and picks exactly one, in this order:

1. **Longest literal prefix.** The literal prefix of a pattern is the part
   before its first wildcard, with the trailing `/**` or directory expansion
   of §5.1 removed. A longer literal prefix is more specific.
2. **More segments.** If two matching patterns have equally long literal
   prefixes, the one with more segments is more specific.
3. **Deny wins.** If a match is still ambiguous, any matching `deny` rule wins.
4. **Stable final order.** If two matching patterns are equal on all of the
   above, the lexicographically smaller pattern string is selected.

Selection is therefore a total order. Two requests with the same
`(uid, path)` select the same rule regardless of the order in which rules
appear in the file, the order in which they were loaded, or how many times the
daemon has been restarted. In particular, a `deny` on a subdirectory always
beats a grant on a parent directory, on every run.

### §5.3 Authorisation

A request is authorised only if:

* the selected rule does not contain `deny`, and
* the selected rule contains the requested op.

Otherwise the request is refused with `reason=denied` and not executed. A rule
that grants `read` does not imply `list`, `write`, or `chown`; each op stands on
its own. Note that §6 may impose further restrictions on an authorised op.

## §6 Path resolution and containment

Normalisation (§5.1) is purely lexical. It does not make a request safe: a
symlink inside an allowed directory can point anywhere on the filesystem.

For every operation the broker MUST establish, on the object it actually uses,
that the final target lies inside the directory tree named by the selected
rule. Concretely:

* All intermediate symlinks are followed, and the resulting final location is
  checked for containment inside the selected rule's root.
* A symlink whose final target leaves that root MUST be refused with
  `reason=path_escape` — regardless of what the target is, whether the caller
  may read it, and whether the target exists. `path_escape` is distinct from
  `denied`: it says the request was well-formed and policy-relevant, but its
  evaluation would leave the sandbox.
* A symlink that stays inside the same root is allowed and behaves as the file
  it names.
* The check and the operation MUST apply to the same object. Re-resolving the
  path after containment has been established is a violation: between the two
  resolutions the object can be replaced.
* Containment is evaluated against the rule root, not against the caller's
  whole rule set. A path matched by a narrower rule is contained by that
  narrower rule's root.
* A request that does not exist is reported as `not_found`; the broker MUST NOT
  create intermediate directories.

## §7 Operations

### §7.1 `read`

Requires `read`. The target must be a regular file. The response carries the
file's bytes in `data`. Files larger than the configured read limit are refused
with `reason=too_large`; the default limit is 8 MiB.

### §7.2 `write`

Requires `write`. If the file exists it is truncated and replaced; if it does
not exist it is created with the mode from the request (permission bits outside
`0777` are ignored, and the process umask applies). Parent directories are
never created. The write is applied to the object that passed §6, not to a path
that is resolved a second time.

### §7.3 `list`

Requires `list`. The target must be a directory. Entries are returned in name
order and include only `name`, `is_dir`, `size`, `mode`, `uid`, and `gid` —
never file contents. `list` does not recurse.

### §7.4 `chown`

`chown` is deliberately more restrictive than the other operations, because it
transfers ownership rather than data.

* The `chown` op must be granted by §5.3 like any other op.
* The target MUST lie inside the caller's **tenant root**: the longest
  non-wildcard directory prefix among the paths of the rules that apply to the
  caller's uid, restricted to rules that grant `chown`. A caller cannot change
  ownership of anything outside the subtree it has been granted.
* The target MUST pass §6 containment with respect to that tenant root.
* The requested `uid` and `gid` MUST each be either the caller's own uid/gid or
  a negative value meaning "leave unchanged". A caller may take ownership of an
  in-tenant object for itself; it may never hand an object to a third party,
  and in particular a non-root caller may never make itself the owner of a root
  file or make a root file owned by anyone else.
* A caller with no rule granting `chown` for the target is refused: a missing
  rule is a refusal, not a permission. This holds even when the caller has
  rules for other paths, and even when the target exists and the caller could
  reach it by other means.
* A violation of the tenant-root rule is reported as `denied`; a violation of
  §6 is reported as `path_escape`.

## §8 Reason codes

| code              | meaning                                                        |
|-------------------|----------------------------------------------------------------|
| `denied`          | policy refuses the operation                                   |
| `path_escape`     | the resolved target leaves the root it was authorised against  |
| `not_found`       | the target does not exist                                      |
| `invalid_request` | malformed framing, unknown op, bad path, or wrong object type  |
| `too_large`       | the result exceeds the configured limit                        |
| `uid_unavailable` | the kernel credential for the connection could not be obtained|
| `io_error`        | the filesystem reported an error for an authorised operation   |

`ok=true` responses leave `reason` empty. `error` is diagnostic only.

## §9 Deployment

* The daemon runs as root and owns a Unix socket whose mode is `0660` and
  whose group is the one shared with business processes.
* The daemon never executes caller-supplied programs, never changes its own
  identity, and never reads a configuration file named by a request.
* Startup fails closed: if the policy cannot be loaded, the socket is not
  created.

## §10 Conformance checklist

The following statements must hold for every build and are the acceptance
criteria for the broker:

1. A caller cannot influence its own identity. Setting any environment
   variable, request field, or command-line flag has no effect on the uid the
   broker attributes to it.
2. A connection whose kernel credential cannot be read is refused; no
   operation is executed for it.
3. `/srv/tenants/acme-evil/x` is not matched by `/srv/tenants/acme/**`, and a
   `*` pattern does not cross a `/`.
4. Lexical `..` cannot move a request out of the pattern that matches it.
5. A symlink inside an allowed directory that points outside the rule root is
   refused with `path_escape`, and no bytes of the outside target are returned.
6. When a parent pattern grants and a child pattern denies, the deny always
   wins, on every run.
7. A caller cannot change the ownership of an object outside its tenant root,
   and cannot assign ownership to a uid/gid other than its own.
8. An operation is executed only when §5.3 authorises it; `deny` always
   refuses.
