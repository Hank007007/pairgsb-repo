# Limits and transport

## 1. Endpoint

podgrated exposes one HTTP endpoint, `POST /admit`, which accepts an
`AdmissionReview` in the `admission.k8s.io/v1` wire format and returns an
`AdmissionReview`. The listening address is a command-line flag; see the
README.

The server answers within the apiserver's admission timeout. It performs no
I/O beyond reading the policy file and the namespace object supplied with the
review.

## 2. Request size

A single review body is limited to **1 MiB** (1,048,576 bytes).

* The limit applies to the body as received. A body that exceeds the limit is
  rejected without being decoded; the point of the limit is to bound the work
  spent on hostile input.
* The rejection is an `AdmissionReview` response with `.allowed: false`, a
  message naming the limit, and the request's uid. It is not an HTTP 413,
  because the apiserver needs a well-formed review response to record the
  failure.
* A body of exactly 1 MiB is accepted; 1 MiB + 1 byte is rejected.

## 3. Review delivery

The review body is a JSON object of this shape:

```json
{
  "apiVersion": "admission.k8s.io/v1",
  "kind": "AdmissionReview",
  "request": {
    "uid": "0b0d6a3e-...",
    "kind": {"group": "", "version": "v1", "kind": "Pod"},
    "namespace": "build-farm",
    "operation": "CREATE",
    "object": {"...": "the submitted Pod"},
    "oldObject": {"...": "the previously stored Pod, on UPDATE and DELETE"}
  },
  "namespaceObject": {
    "metadata": {"name": "build-farm", "labels": {"tier": "build"}}
  }
}
```

`namespaceObject` is the `Namespace` object for `request.namespace`; it is
delivered in the same body. It may be absent.

### 3.1 Operations

* `CREATE` carries the submitted Pod in `request.object` and no
  `request.oldObject`. The Pod to inspect is `request.object`.
* `UPDATE` carries the new Pod in `request.object` and the stored Pod in
  `request.oldObject`. The Pod to inspect is `request.object`; the policy
  statement is about what will be stored.
* `DELETE` carries the stored Pod in `request.oldObject` and an empty or absent
  `request.object`. The Pod to inspect is `request.oldObject`.
* `CONNECT` does not apply to Pods and is answered as an allow with no patch.

An operation that cannot be decoded into a Pod is rejected, not allowed.

## 4. Failure modes

| Situation | Answer |
| --- | --- |
| Body over 1 MiB | reject (§2) |
| Body is not valid JSON or not an AdmissionReview | reject |
| `request` absent | reject |
| operation not applicable to a Pod | allow, no patch |
| policy file unavailable (§4 of `policy-source.md`) | reject |

Every rejection carries `.response.uid` equal to the request's uid, and every
rejection carries a message.
