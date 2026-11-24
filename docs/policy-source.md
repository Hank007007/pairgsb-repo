# Policy source

podgrated reads its policy from a YAML file on disk. The path is supplied on
the command line. The file is re-read while the service runs; it is not a
startup-only artifact.

## 1. File format

```yaml
version: 7
allowedHostPaths:
  - /var/run
  - /var/log/pods
allowedRegistries:
  - registry.internal
allowedCapabilities:
  - CHOWN
  - NET_BIND_SERVICE
defaults:
  podSecurityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containerSecurityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
namespaces:
  - name: build-farm
    labels:
      tier: build
    enabled: true
    overrides:
      allowedRegistries:
        - registry.internal
        - registry.build.internal
  - name: partner-acme
    labels:
      tier: partner
    enabled: true
    overrides:
      allowedHostPaths:
        - /var/run
default:
  overrides:
    allowedHostPaths:
      - /var/run
```

`version` is a monotonically increasing integer chosen by whoever edits the
file. It is the only signal that the content changed.

`namespaces` is optional. Each entry selects namespaces by label (§2) and may
override any of `allowedHostPaths`, `allowedRegistries`, `allowedCapabilities`,
`defaults`. Unspecified keys fall through to the top-level values.

The top-level `default` entry carries the same `overrides` shape and is used
when no namespace entry matches.

## 2. Namespace selection

Namespace labels are not part of the Pod. They come from the `Namespace`
object that the `AdmissionRequest` names in `request.namespace`; podgrated
receives that object alongside the review (see `limits.md` §3 for the request
shape it is delivered in) and uses its `metadata.labels`.

Selection is by `namespaceSelector`-style label matching:

* Every label in a namespace entry's `labels` map must be present with the
  identical value on the Namespace object. A missing key or a different value
  means no match.
* When several entries match, the first one in file order wins.
* When no entry matches, the top-level `default` entry applies; when that is
  absent, the top-level values apply unchanged.

A namespace whose `AdmissionRequest` carries an empty `namespace` matches no
selector.

## 3. Reload

* The file is read when the service starts.
* On each review the service considers re-reading it. It re-reads only when the
  `version` in the file differs from the version currently in effect. A file
  whose `version` is unchanged is not re-applied, even if its bytes differ.
* After a successful read whose `version` differs, the new content takes effect
  for the very next review. An entry with `enabled: false` therefore stops
  applying from the next review onward, with no restart and no drain.
* `enabled: false` on a namespace entry removes that entry from selection; if it
  was the selected entry, selection falls back per §2.

Reload is a whole-file operation. Partial merges of an edited file are not
performed.

## 4. Failure

The policy file is the authority on what may enter the cluster. If it cannot be
read, parsed, or validated (unknown keys are tolerated; a `version` that is
absent or not an integer is not), the service has no policy to apply, and it
fails closed: every review is rejected until a readable policy is in effect. A
stale in-memory copy is not used in place of a failed reload, and an empty rule
set does not mean "allow everything".

Failures are reported as an admission rejection with a rule identifier, not as
a transport error; the apiserver is expected to receive a well-formed
`AdmissionReview` response.
