# caveatchain

Offline capability tokens with attenuating caveat chains. A root token is
signed by an issuer; any holder can append caveats that only narrow the
token; a verifier holding the root key can validate the whole chain locally,
without calling back to the issuer. Revocation is handled by short lived
revocation lists compared locally.

Standard library only. Python 3.9+.

```bash
python -m pytest -q -p no:cacheprovider
```

## Layout

| path | role |
|---|---|
| `caveatchain/token.py` | wire format, encode/decode, attenuation |
| `caveatchain/sign.py` | root and chain HMAC computation |
| `caveatchain/caveats.py` | caveat types and satisfaction rules |
| `caveatchain/verify.py` | the full verification pipeline |
| `caveatchain/revoke.py` | revocation lists |
| `caveatchain/store.py` | in-memory root id / key registry |
| `docs/capability-chain.md` | normative specification |
| `tests/` | test suite |

## Example

```python
import time
from caveatchain import (
    CaveatContext, KeyStore, issue_token, verify_encoded,
)

store = KeyStore()
root_id = store.create()
key = store.key_for(root_id)

token = issue_token(root_id, key)
token = token.attenuate(key, {"type": "path", "prefix": "/docs"})
token = token.attenuate(key, {"type": "expiry", "exp": int(time.time()) + 600})

result = verify_encoded(token.encode(), store, CaveatContext(path="/docs/a.txt"))
assert result.ok, result.reason
```

## Status

Version 1. `docs/capability-chain.md` is the normative description of the
required behaviour; the implementation is expected to conform to it.
