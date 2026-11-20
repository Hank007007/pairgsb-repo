# Design

`mimesieve` sits between the upload gateway and object storage.  One call
decides whether the bytes may be stored; the policy it enforces is written in
[`image-intake-policy.md`](image-intake-policy.md).

## Data flow

```
gateway ──inspect(data, declared_type)──▶ verify
                                           │
                        ┌──────────────────┼──────────────────┐
                        ▼                  ▼                  ▼
                     sniff.detect      walkers            policy.allow
                 (container id)   (structure walk)     (family whitelist)
```

1. **`sniff.detect`** looks at the leading bytes and returns the container
   type.  The declared Content-Type is normalised and kept alongside, because
   the gate must compare the two.
2. **A walker** per container reads only the framing needed to reach the
   dimension header and the terminator.  Each walker returns a
   `model.Header(width, height, end)` where `end` is the offset just past the
   container's terminator, or raises `errors.IntakeError` with one of the
   `errors.Reason` codes.
3. **`verify.inspect`** calls the sniffer, dispatches to the matching walker,
   checks the container tail and the dimension limits, and folds everything
   into a `model.Result`.

`inspect` never lets an exception escape: a malformed upload is a refusal with
a reason code, not a crash in the gateway.

## Module responsibilities

| module | responsibility |
|---|---|
| `errors.py` | the rejection codes and `IntakeError` |
| `model.py` | `Header` (walker output) and `Result` (gateway output) |
| `sniff.py` | container identification and Content-Type normalisation |
| `png.py` | PNG chunk framing and `IHDR` |
| `jpeg.py` | JPEG marker framing, frame header, end of image |
| `gif.py` | GIF logical screen descriptor and block stream |
| `webp.py` | RIFF/WebP chunk framing and canvas size |
| `policy.py` | the supported container list and the dimension limits |
| `verify.py` | orchestration: sniff → walk → tail → limits → policy |
| `tools/intake.py` | human-readable report over a directory of samples |

## Invariants

* The walkers never trust a length field; every length is bounded by the bytes
  actually present (policy §3.1).
* A container has exactly one dimension header (policy §3.2).
* Metadata payloads are opaque: markers inside a segment body are data, and an
  embedded image never supplies the main image's size (policy §3.3).
* The terminator position returned by a walker is compared with the real file
  length (policy §3.4).
* Limits are evaluated on exact integers (policy §4).

## Testing

`tests/` builds each container in memory and asserts the admitted results;
`testdata/images/` holds a broader corpus, and each sample's declared
Content-Type lives in a `.ctype` sidecar beside it.  `tools/intake.py` prints
the verdict for that corpus.
