# tokengrit

Internal library for API token issuance, verification, rotation and
revocation. Pure Python, SQLite storage, no third-party runtime dependencies.

```
tg_<id8>_<secret>
```

The plaintext token is returned once, at issuance. Only a salted scrypt
digest is stored.

## Requirements

* Python 3.11+
* pytest (tests only)

## Usage

```python
from tokengrit import Store, verify_token, rotate

store = Store("tokens.sqlite3")

issued = store.issue(scopes=["read", "write"])
result = verify_token(store, issued.raw, required_scope="read")
assert result.ok

successor = rotate(store, issued.raw)
store.revoke(successor.raw)
```

## Documents

* `docs/token-format.md` — token grammar and public verification outcomes
* `docs/hashing.md` — secret hashing and comparison
* `docs/design.md` — required behaviour of the library

## Tests

```
python -m pytest -q -p no:cacheprovider
```
