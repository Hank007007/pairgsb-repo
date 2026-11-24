"""Token exchange core: request parameters in, issued token out."""

import time

from .. import policy, providers
from ..errors import AccessDenied, InvalidRequest, InvalidScope, InvalidToken
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


def _check_token_type(params, key, *, required):
    token_type = params.get(key)
    if required and not token_type:
        raise InvalidRequest(key + " is required")
    if token_type is not None and token_type not in policy.ALLOWED_TOKEN_TYPES:
        raise InvalidRequest("unrecognised " + key)


def _resolve_mode(params, actor_token):
    mode = params.get("mode")
    if mode is not None and mode not in ("delegation", "impersonation"):
        raise InvalidRequest("unsupported mode")
    if mode == "delegation" and not actor_token:
        raise InvalidRequest("delegation mode requires an actor_token")
    if mode == "impersonation" and actor_token:
        raise InvalidRequest("actor_token is not allowed in impersonation mode")
    return actor_token is not None or mode == "delegation"


def _may_act_allows(subject_claims, actor_claims):
    may_act = subject_claims.get("may_act")
    if not may_act:
        return False
    entries = may_act if isinstance(may_act, list) else [may_act]
    actor_sub = actor_claims.get("sub")
    for entry in entries:
        if not isinstance(entry, dict):
            continue
        if entry.get("sub") == actor_sub:
            return True
        inner = entry.get("act")
        if isinstance(inner, dict) and inner.get("sub") == actor_sub:
            return True
    return False


def _result_scope(params, subject_claims, client):
    """Intersect requested, subject, and registered scope sets.

    Every requested scope must be available from both the subject grant and
    the client registration; the request is rejected rather than silently
    narrowed. Wildcard requests expand to the concrete family members
    available on both sides; the wildcard itself is never issued.
    """
    requested = params.get("requested_scope")
    if requested is None or not str(requested).strip():
        requested = subject_claims.get("scope")
    requested_scopes = canon.normalize(requested)
    if not requested_scopes:
        raise InvalidScope("no scope can be issued")

    subject_scopes = canon.normalize(subject_claims.get("scope"))
    registered_scopes = registry.registered_for_client(
        client, client.get("scopes", "")
    )

    issued = set()
    for scope in requested_scopes:
        if canon.is_wildcard(scope):
            namespace = canon.wildcard_namespace(scope)
            candidates = set(subject_scopes) | set(registered_scopes)
            members = {
                candidate
                for candidate in candidates
                if not canon.is_wildcard(candidate)
                and candidate.startswith(namespace + ":")
            }
            selected = {
                member
                for member in members
                if canon.covers(subject_scopes, member)
                and canon.covers(registered_scopes, member)
            }
            if not selected:
                raise InvalidScope("requested scope is not available")
            issued.update(selected)
            continue
        if not canon.covers(subject_scopes, scope):
            raise InvalidScope("subject token does not hold the requested scope")
        if not canon.covers(registered_scopes, scope):
            raise InvalidScope("client registration does not allow the requested scope")
        issued.add(scope)
    return sorted(issued)


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

    if not params.get("subject_token"):
        raise InvalidRequest("subject_token is required")
    _check_token_type(params, "subject_token_type", required=True)

    subject_claims = codec.decode(params.get("subject_token"))
    _validate_subject(subject_claims, client, now)

    actor_token = params.get("actor_token")
    _resolve_mode(params, actor_token)
    if actor_token:
        _check_token_type(params, "actor_token_type", required=False)
        actor_claims = codec.decode(actor_token)
        _validate_actor(actor_claims, now)
        if not _may_act_allows(subject_claims, actor_claims):
            raise AccessDenied("the subject token does not permit this actor")
    else:
        actor_claims = None

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
        scopes=scopes,
        ttl=lifetime,
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
