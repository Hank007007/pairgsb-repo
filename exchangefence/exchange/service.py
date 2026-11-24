"""Token exchange core: request parameters in, issued token out."""

import time

from .. import policy, providers
from ..errors import InvalidRequest, InvalidScope, InvalidToken
from ..grants.store import GrantStore
from ..scopes import canon, registry
from ..tokens import codec
from ..tokens.claims import build

_DEFAULT_STORE = GrantStore()


def _default_store():
    return _DEFAULT_STORE


def _clock(now):
    return int(time.time()) if now is None else int(now)


def _validate_subject(claims, client, now):
    """Check that *claims* may act as the subject of an exchange."""
    if claims.get("iss") != policy.ISSUER:
        raise InvalidToken("subject token issuer is not accepted")
    if claims.get("aud") != client.get("audience"):
        raise InvalidToken("subject token audience is not accepted")
    expires_at = claims.get("exp")
    if not isinstance(expires_at, (int, float)) or int(expires_at) <= now:
        raise InvalidToken("subject token is expired")
    if not claims.get("sub"):
        raise InvalidToken("subject token has no subject")


def _validate_actor(claims, now):
    """Check that *claims* may act as the actor of an exchange."""
    if claims.get("iss") != policy.ISSUER:
        raise InvalidToken("actor token issuer is not accepted")
    expires_at = claims.get("exp")
    if not isinstance(expires_at, (int, float)) or int(expires_at) <= now:
        raise InvalidToken("actor token is expired")
    if not claims.get("sub"):
        raise InvalidToken("actor token has no subject")


def _scope_entries(value):
    if value is None:
        return []
    if isinstance(value, str):
        return value.split(" ")
    entries = []
    for item in value:
        entries.extend(str(item).split(" "))
    return entries


def _result_scope(params, subject_claims, client):
    """Return the scope entries to place on the issued token."""
    requested = params.get("requested_scope")
    if requested is None or not str(requested).strip():
        requested = subject_claims.get("scope")
    entries = _scope_entries(requested)
    allowed = registry.registered_for_client(client, canon.normalize(entries))
    return [scope for scope in entries if scope in allowed]


def _requested_ttl(params):
    value = params.get("ttl")
    if value is None or value == "":
        return None
    try:
        ttl = int(value)
    except (TypeError, ValueError):
        raise InvalidRequest("ttl must be an integer")
    if ttl <= 0:
        raise InvalidRequest("ttl must be positive")
    return ttl


def exchange(params, *, store=None, now=None):
    """Perform a token exchange and return the token response fields."""
    now = _clock(now)
    client = providers.get_client(params.get("client_id"))
    if client is None:
        raise InvalidRequest("unknown client")

    subject_claims = codec.decode(params.get("subject_token"))
    _validate_subject(subject_claims, client, now)

    actor_claims = None
    actor_token = params.get("actor_token")
    if actor_token:
        actor_claims = codec.decode(actor_token)
        _validate_actor(actor_claims, now)

    scopes = _result_scope(params, subject_claims, client)
    ttl = _requested_ttl(params)
    claims = build(
        subject_claims,
        scopes,
        audience=client.get("audience"),
        client_id=params.get("client_id"),
        ttl=ttl,
        actor_claims=actor_claims,
        now=now,
    )
    token = codec.encode(claims)

    if store is None:
        store = _default_store()
    lifetime = max(1, int(claims["exp"]) - now)
    refresh_handle = store.create(
        subject=claims.get("sub"),
        client_id=params.get("client_id"),
        audience=client.get("audience"),
        scopes=canon.normalize(subject_claims.get("scope")),
        ttl=lifetime,
        downscoped_scope=scopes,
    )

    return {
        "access_token": token,
        "issued_token_type": policy.ACCESS_TOKEN_TYPE,
        "token_type": "Bearer",
        "expires_in": lifetime,
        "scope": " ".join(scopes),
        "refresh_token": refresh_handle,
        "claims": claims,
    }


def refresh(params, *, store=None, now=None):
    """Redeem a refresh grant and return the token response fields."""
    now = _clock(now)
    handle = params.get("refresh_token")
    if not handle:
        raise InvalidRequest("refresh_token is required")
    if store is None:
        store = _default_store()

    record = store.lookup(handle)
    client = providers.get_client(params.get("client_id") or record.get("client_id"))
    if client is None:
        raise InvalidRequest("unknown client")

    requested = params.get("requested_scope")
    if requested is None or not str(requested).strip():
        scopes = canon.normalize(record.get("scope"))
    else:
        scopes = canon.normalize(requested)
        if not canon.covers(record.get("scope"), scopes):
            raise InvalidScope("requested scope exceeds the grant")

    claims = build(
        {
            "sub": record.get("subject"),
            "scope": " ".join(scopes),
            "exp": record.get("expires_at"),
        },
        scopes,
        audience=record.get("audience"),
        client_id=record.get("client_id"),
        ttl=None,
        now=now,
    )
    token = codec.encode(claims)
    new_handle = store.rotate(handle, scopes=scopes)
    lifetime = max(1, int(claims["exp"]) - now)

    return {
        "access_token": token,
        "issued_token_type": policy.ACCESS_TOKEN_TYPE,
        "token_type": "Bearer",
        "expires_in": lifetime,
        "scope": " ".join(scopes),
        "refresh_token": new_handle,
        "claims": claims,
    }
