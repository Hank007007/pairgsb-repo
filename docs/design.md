# Design

This note describes how a review travels through podgrated and what each layer
is responsible for. It describes responsibilities and contracts, not
implementation; the normative behaviour lives in the four specification
documents next to it.

## 1. The request path

```
AdmissionReview bytes
        │
        ▼
   DecodeReview        internal/admission/decode.go
        │
        ▼
   LoadPolicy          internal/policy/load.go
        │
        ▼
   BuildPatch          internal/mutate/patch.go
        │
        ▼
   apply in memory
        │
        ▼
   Evaluate            internal/policy/rules.go
        │
        ▼
   Handle → response   internal/admission/serve.go
```

The order is not incidental. The policy speaks about the Pod that will be
stored, so mutation happens before evaluation, and evaluation sees the mutated
object. Decoding comes first because everything downstream needs to know which
object the review is about.

`Handle` owns the sequencing and the translation between the layers' return
values and the admission response. It is the only place that knows about both
the wire format and the rule engine.

## 2. Decoding

`DecodeReview` turns one body into the typed view the rest of the service uses.
Its contract:

* enforce the size limit before any parsing work happens;
* pick the right object for the operation — the submitted object for CREATE
  and UPDATE, the stored object for DELETE;
* carry through every container list the Pod declares, not just the one the
  common case uses;
* surface the request metadata downstream layers need, in particular the
  namespace the review names, so policy selection can consult it.

Its output is a `Decoded` value plus an error. An error is a rejection: a body
that cannot be decoded into something the service understands is not a Pod the
service can vouch for.

## 3. Policy selection

Loading and selection are separate concerns that meet in one place.

`LoadPolicy` is responsible for turning the file at a path into a `Policy`, and
for deciding whether the file is usable at all. It is called on the request
path, so it also owns the reload decision: the file is only re-applied when its
`version` differs from the one in effect. A failed read or parse is reported as
an error, never as an empty policy.

`Policy` selection decides which values apply to a given review. The top level
of the file is the base; a namespace entry selected by label supplies the
values it sets and inherits the rest; the `default` entry is the fallback. The
selection itself is a label match against the Namespace object delivered with
the review, which is why `namespace` has to survive decoding.

The result of selection is what `Evaluate` and `BuildPatch` consume. Neither of
them re-reads the file or re-runs selection.

## 4. Rule evaluation

`Evaluate` is the policy itself. It takes the selected values and a Pod, and
returns whether the Pod is allowed plus the violations that fired.

Its contract:

* every rule that speaks about a container applies to every element of every
  container list;
* container-level security settings take precedence over Pod-level ones, field
  by field;
* path and image comparisons are structural, not textual — a path is compared
  by its normalized segments, an image by the registry segment of its
  reference;
* capability names are compared as names, after case folding and trimming;
* each violation carries the rule identifier and the subject it applies to, and
  does not repeat manifest text that the author controls.

`Evaluate` does not mutate its input and does not know about defaults. A Pod
that fails evaluation is rejected regardless of what a patch could have added.

## 5. Mutation

`BuildPatch` produces the JSONPatch for a Pod that has already been evaluated.
It is driven by the `defaults` section of the selected policy.

Its contract:

* emit an operation only for a leaf the Pod leaves unset;
* never replace an object that already exists with a value derived from
  defaults, because that discards what the author wrote;
* address containers by a pointer that resolves to them, escaping any token
  that needs it;
* append to arrays rather than overwriting positions;
* cover all three container lists, and emit nothing for a list that is empty.

The patch is applied to a copy of the decoded Pod before evaluation. Applying
it may change the outcome: a default that introduces a forbidden value is a
rejection like any other.

## 6. Response

The HTTP layer wraps `Handle`'s decision in an `AdmissionReview`.

* `.response.uid` is the request's uid.
* `.response.allowed` comes from the decision.
* `.response.status.message` explains a rejection, naming the rule and the
  subject but not the manifest values.
* `.response.warnings` lists the rules that fired and were waived.
* `.response.patch` and `.response.patchType` are set when there is a patch.

A decision, once reached, is always delivered as a well-formed review response
— including when the reason for the decision is that the service could not
read its own policy.

## 7. Failure policy

Everything in the path fails closed. A review that cannot be decoded, a policy
file that cannot be read, a patch that cannot be applied, a Pod that cannot be
evaluated — each is a rejection with a message. The only path that answers
`allowed: true` is one where a Pod was decoded, a policy was in effect, the
patch applied, and no rule fired.
