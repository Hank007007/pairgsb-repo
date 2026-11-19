# sessforge

Server-side session management for web applications: session cookies, stored
session records, validity checks, logout, revocation and remember-me tokens.

The normative behaviour is in [`docs/session-policy.md`](docs/session-policy.md).
Background on why the policy looks the way it does is in
[`docs/incident-reports.md`](docs/incident-reports.md).

## Requirements

- Python 3.10+
- `pytest` for the test suite

No runtime dependencies outside the standard library.

## Install for development

```console
python -m pip install -e ".[test]"
```

## Run the tests

```console
python -m pytest -q -p no:cacheprovider
```

## Layout

| Path | Contents |
| --- | --- |
| `sessforge/store.py` | storage interface, record types and the in-memory implementation |
| `sessforge/lifecycle.py` | login, validation, renewal and revocation |
| `sessforge/cookie.py` | cookie attributes and logout cleanup |
| `sessforge/epoch.py` | per-user privilege epochs |
| `sessforge/remember.py` | long-lived remember-me tokens |
| `tests/fakes.py` | a fake-Redis adapter used to exercise the storage contract |

## Usage sketch

```python
import time

from sessforge import MemoryStore
from sessforge.cookie import session_cookie
from sessforge.lifecycle import login, validate
from sessforge.remember import issue, redeem

store = MemoryStore()
now = time.time()

record = login(store, user_id="alice", now=now, sid=request_sid)
set_cookie(session_cookie(record.sid, max_age=12 * 60 * 60).to_header())

result = validate(store, request_sid, time.time())
if not result.ok:
    redirect_to_login()

token = issue(store, "alice", time.time())
successor = redeem(store, token.token, time.time())
```

Applications decide when a credential or permission change warrants an epoch
advance; `sessforge.epoch.bump` performs it.
