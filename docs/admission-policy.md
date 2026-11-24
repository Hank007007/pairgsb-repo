# Admission policy

This document defines the policy rules podgrated applies to every Pod the
kube-apiserver submits for admission. The rules are normative: a Pod that
violates any rule in §3 is rejected, and the rejection carries the identifier
of the rule that produced it.

Rule identifiers are stable strings of the form `podgrate/<area>/<name>`. They
are surfaced in the admission response and in the server log, and tooling
treats them as an API.

## 1. Scope

Every review whose object is a Pod is subject to these rules, regardless of
the operation (CREATE, UPDATE, DELETE) and regardless of which workload
controller produced the Pod template.

## 2. What is inspected

### 2.1 Container lists

A Pod specification has three container lists. All three are part of the Pod
that will actually run:

* `spec.containers`
* `spec.initContainers`
* `spec.ephemeralContainers`

Every rule in §3 that speaks about "a container" applies to each element of
each of the three lists. A violation in any one of them rejects the Pod. Lists
that are absent or empty contribute nothing.

### 2.2 Effective security context

`securityContext` exists at two levels: on the Pod (`spec.securityContext`)
and on each container (the container's own `securityContext`). For every field
that exists at both levels, the container-level value wins for that container
when it is present. When the container does not set the field, the Pod-level
value applies.

Rules are therefore evaluated per container against the *effective* value: the
container-level setting if present, otherwise the Pod-level setting. A
Pod-level default does not excuse a container that overrides it, and a
container-level override does not leak to sibling containers.

## 3. Rejection rules

### 3.1 Host path volumes

A volume of type `hostPath` exposes a path on the node. Only paths inside the
allow-listed directory trees may be mounted.

Allowed trees are declared in the policy source (§1 of `policy-source.md`), for
example `/var/run` and `/var/log/pods`.

Path comparison is performed on normalized paths, segment by segment:

1. The path is normalized before comparison: `.` and `..` segments are
   resolved, repeated separators collapsed, and a trailing separator removed.
   `/var/run` and `/var/run/x` denote directories; a path that normalizes to
   `/var/run` is the tree root itself.
2. A path is inside an allowed tree when, after normalization, it is equal to
   the tree root or its segment sequence begins with the tree root's segment
   sequence followed by at least one more segment.

The consequences below follow from this definition and are load-bearing:

* `/var/run` is inside `/var/run`.
* `/var/run/x` and `/var/run/x/y` are inside `/var/run`.
* `/var/run-evil` is not inside `/var/run`; the first differing segment is
  `run-evil`, not `run`.
* `/var/run/../../etc` normalizes to `/etc` and is not inside `/var/run`.

A `hostPath` volume that is declared but not mounted by any container is still
inspected: the volume is part of the Pod spec and may be mounted by a later
revision of the Pod.

### 3.2 Host namespaces and privilege

Any of the following is rejected outright; there is no allow-list and no
namespace-based exemption:

| Field | Violating value |
| --- | --- |
| `spec.hostNetwork` | `true` |
| `spec.hostPID` | `true` |
| `spec.hostIPC` | `true` |
| container `securityContext.privileged` | `true` |
| container `securityContext.allowPrivilegeEscalation` | `true` |

`privileged` and `allowPrivilegeEscalation` are evaluated per container using
the effective value defined in §2.2. An absent value is not a violation; it is
a candidate for defaulting (§1 of `mutation.md`).

### 3.3 Capabilities

Container capabilities are declared as `securityContext.capabilities.add`. The
following are rejected:

* any entry equal to `ALL`, compared case-insensitively and ignoring
  surrounding whitespace (`ALL`, `all`, ` all ` are the same entry);
* any entry, after the same case folding and trimming, that is not in the
  policy's capability allow-list.

Entries are compared as capability names, not as raw strings. The allow-list is
declared in the policy source (§1 of `policy-source.md`). An empty or absent
`add` list is not a violation.

A `drop` list does not make an entry in `add` acceptable; the two lists are
evaluated independently.

### 3.4 Image provenance

Every container image must come from an allowed registry. The allowed
registries are declared in the policy source (§1 of `policy-source.md`).

The registry of an image reference is determined by splitting the reference
into repository and tag/digest, then taking the first path segment of the
repository:

* `registry.internal/team/app:1.2` → `registry.internal`
* `evil.example/registry.internal/x:1` → `evil.example`

An image is accepted only when its registry segment matches an allowed registry
exactly. A reference without a registry-looking first segment (for example
`nginx:1.25`, which denotes a Docker Hub library image) does not resolve to an
allowed registry and is rejected.

The check applies to all three container lists.

## 4. Messages and warnings

Rejected reviews carry a message that identifies the rule and the subject
(container name or volume name). Messages do not quote manifest values such as
image references or host paths; those are reported by position or by a short
descriptor instead.

Accepted reviews may carry warnings. A warning names a rule that fired and was
waived, together with the subject it applied to. Warnings are advisory and do
not change `.allowed`.
