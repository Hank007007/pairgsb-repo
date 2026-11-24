# podgrate

podgrate is the admission policy service for our self-hosted multi-tenant
cluster. The kube-apiserver posts every Pod CREATE, UPDATE and DELETE to it as
an `AdmissionReview`; the service decides whether the Pod may land, and for
accepted Pods returns a JSONPatch that fills in the cluster's security
defaults.

The cluster runs internal build pipelines and a number of partner namespaces.
Pod manifests are written by other teams, so this service is the single gate
between a manifest and a running container.

## Layout

```
cmd/podgrated        entry point and flags
internal/admission   review decoding, the request path, the HTTP handler
internal/policy      policy file loading, namespace selection, the rules
internal/mutate      JSONPatch construction and JSON Pointer escaping
internal/kube        wire types for AdmissionReview and Pod
docs/                the normative specification
testdata/            sample policy and reviews
```

## Specification

The behaviour of the service is specified by four documents. They are the
authority on what the code should do; where they and the code disagree, the
documents win.

| Document | Covers |
| --- | --- |
| `docs/admission-policy.md` | which Pods are rejected, and what is inspected |
| `docs/mutation.md` | the defaults patch and the order of validation |
| `docs/policy-source.md` | the policy file, namespace selection, reload, failure |
| `docs/limits.md` | endpoint, request size limit, operations, failure modes |

`docs/design.md` describes how the request path is split across the packages.

## Running

```sh
go build ./cmd/podgrated
./podgrated -listen :8443 -policy testdata/policy.yaml
```

Then post a review:

```sh
curl -s localhost:8443/admit --data-binary @testdata/reviews/create-web.json
```

`GET /healthz` returns `ok` while the process is up. It says nothing about the
state of the policy file; review answers are the signal for that.

Flags:

* `-listen` — address to listen on (default `:8443`)
* `-policy` — path to the policy file (default `testdata/policy.yaml`)

The process exits with status 2 when `-listen` is empty, and status 1 when the
listener fails.

## Tests

```sh
go test ./...
```

The tests exercise the request path against the sample policy and reviews in
`testdata/`. They cover the plain single-container flow.
