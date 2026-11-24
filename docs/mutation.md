# Mutation

podgrated returns a JSONPatch alongside `.allowed` for accepted reviews. The
patch is produced from the submitted object and the defaulting section of the
policy source. The apiserver applies it in the same step that decides whether
the object is stored, so it is the last opportunity to shape a Pod before it
exists.

## 1. Defaulting semantics

A default is applied only where the field is absent. Concretely:

* If the Pod already sets `spec.securityContext.<field>`, the Pod default for
  that field is not emitted. A Pod-level default does not overwrite a value the
  author wrote.
* If a container already sets `securityContext.<field>`, the container default
  for that field is not emitted, even when the Pod-level default applies to
  sibling containers.
* When the containing object (`securityContext`, `capabilities`, …) is absent,
  it may be created, but only with the fields the default is actually
  supplying. Creating an empty parent that displaces nothing is acceptable;
  creating a parent that replaces existing content is not.
* Defaulting is per missing leaf. A container that has
  `securityContext.runAsUser: 1000` keeps that value and still receives the
  `seccompProfile` default.
* An explicit `false`, `0`, or empty list is a value, not an absence.

Defaults come from `defaults` in the policy source (§1 of
`policy-source.md`). Two groups exist: Pod-level fields
(`spec.securityContext.*`) and container-level fields
(`spec.containers[*].securityContext.*`, and likewise for the other two
container lists).

## 2. Patch format

The response is an RFC 6902 JSONPatch document encoded as a JSON array of
operations.

### 2.1 Pointer escaping

Operation paths are RFC 6901 JSON Pointers. Any reference token that itself
contains `~` or `/` is escaped: `~` becomes `~0`, `/` becomes `~1`. This applies
to every token taken from user-controlled data, including container names,
annotation keys, and label keys. Array elements are addressed by their decimal
index in the array.

### 2.2 Array operations

New array elements are appended with the `-` token: `add` to
`/spec/containers/app/securityContext/capabilities/drop/-` appends one entry to
that container's drop list. Appending by numeric index places the entry at that
position in the existing array.

Every emitted path resolves against the object the patch is applied to. The
service emits operations only against lists that have a corresponding element.

### 2.3 Coverage

Defaulting applies to all three container lists, as in §2.1 of
`admission-policy.md`. A Pod that has containers and init containers receives
defaults for both; an absent or empty list receives nothing.

Containers whose effective value already satisfies a default are left out
entirely; they do not receive a no-op operation.

## 3. Validation happens after mutation

The policy in `admission-policy.md` is a statement about the Pod that will be
stored, not about the Pod that was submitted. Therefore:

1. Decode the review.
2. Compute the patch.
3. Apply the patch to the submitted object in memory.
4. Evaluate the policy against the mutated object.

A default that would introduce a forbidden value — for example a policy whose
`defaults` carry `privileged: true`, or a capability outside the allow-list —
is rejected exactly as if the author had written it. A Pod that is rejected on
its submitted form is not rescued by mutation; the order above rejects it
either way.

If the patch cannot be applied in memory, the review is rejected.

## 4. Response

The response mirrors the request:

* `.response.uid` equals `.request.uid` exactly. The apiserver discards a
  response whose uid does not match.
* `.response.patchType` is `JSONPatch` when a non-empty patch is returned.
* `.response.allowed` reflects §3.
* `.response.status.message` is set on rejection.
* `.response.warnings` carries one entry per rule that fired and was waived.
