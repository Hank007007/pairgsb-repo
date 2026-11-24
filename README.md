# noncebind

`noncebind` is the shared OpenID Connect login component for the internal
back-office applications. It builds authorization requests, handles the
provider callback, exchanges the authorization code, validates the ID token,
and maintains the session that follows a successful login.

Four providers are registered out of the box:

| name | issuer | algorithms |
|---|---|---|
| `okta-dev` | `https://idp.example.com/oauth2/default` | RS256 |
| `azure-ad` | `https://login.microsoftonline.com/.../v2.0` | RS256 |
| `keycloak-eu` | `https://sso.eu.example.com/realms/internal` | RS256, ES256 |
| `auth0-partner` | `https://partner.eu.auth0.com/` | ES256 |

## Layout

```
noncebind/
  providers.py          provider registry (issuer, audience, algorithms, endpoints)
  oidc/
    jwt.py              compact JWT decoding
    keys.py             JWK -> public key, JWS signature verification
    jwks.py             JWKS retrieval, cache, kid selection
    token.py            token endpoint client
    verify.py           ID token validation
  login/
    transaction.py      login transaction lifecycle
    authorize.py        authorization URL construction
    callback.py         callback orchestration
  session/
    store.py            anonymous and authenticated sessions
docs/
  oidc-profile.md       signature algorithm and claim rules
  jwks.md               key retrieval, caching, and selection rules
  login-binding.md      state, nonce, redirect binding, and session transition
DESIGN.md               layer responsibilities and intended behaviour
```

## Usage

The library is framework-neutral. A typical integration looks like this:

```python
from noncebind.login.authorize import build_authorization_url
from noncebind.login.callback import CallbackRequest, handle_callback
from noncebind.login.transaction import TransactionStore
from noncebind.providers import get_provider
from noncebind.session.store import SessionStore

provider = get_provider("okta-dev")
transactions = TransactionStore()
sessions = SessionStore()

anonymous = sessions.create_anonymous()
transaction = transactions.issue(provider, provider.redirect_uri)
url = build_authorization_url(provider, transaction)
```

When the browser returns:

```python
response = handle_callback(
    CallbackRequest(
        state=state,
        code=code,
        redirect_uri=provider.redirect_uri,
        session_id=anonymous.id,
    ),
    provider=provider,
    transactions=transactions,
    sessions=sessions,
)
```

`response.status`, `response.headers`, and `response.cookies` are what an HTTP
adapter needs to write back to the browser.

Every function that consults the clock or talks to the network accepts an
override (`now=`, `transport=`, `fetcher=`), so the flow can be exercised
without sockets or wall-clock waits.

## Development

```
python -m pytest -q -p no:cacheprovider
```

Requires Python 3.10 or newer, `pytest`, and `cryptography`. Nothing else.
