"""Ask a vendor whether a credential is usable right now.

See ``docs/swap-window.md`` §3.
"""

from __future__ import annotations

from typing import Any

try:  # pragma: no cover
    import requests
except Exception:  # pragma: no cover
    requests = None  # type: ignore[assignment]


# Retained for compatibility with older test setup/teardown.  It is intentionally
# never read: verification must make a fresh external call every time.
_CHECKED: dict[str, Any] = {}


def _spec_name(spec: Any) -> str:
    if isinstance(spec, dict):
        return str(spec.get("name", ""))
    return str(getattr(spec, "name", ""))


def _spec_service(spec: Any) -> str:
    if isinstance(spec, dict):
        return str(spec.get("service", ""))
    return str(getattr(spec, "service", ""))


def _spec_slot(spec: Any) -> str:
    if isinstance(spec, dict):
        return str(spec.get("slot", ""))
    return str(getattr(spec, "slot", ""))


def verify_credential(
    provider: Any,
    spec: Any,
    key_id: str | None,
    secret: str | None,
    *,
    timeout: float = 5.0,
) -> bool:
    """Make one fresh live provider probe for one credential."""

    if not key_id:
        return False
    request_spec = {
        "name": _spec_name(spec),
        "service": _spec_service(spec),
        "slot": _spec_slot(spec),
    }
    try:
        result = provider.probe(
            request_spec,
            key_id,
            secret or "",
            timeout=timeout,
        )
    except Exception:
        return False

    if isinstance(result, dict):
        if "ok" in result:
            return bool(result["ok"])
        state = str(result.get("state", "")).lower()
        if state:
            return state == "active"
    return bool(result)


def verify(
    spec: Any = None,
    material: dict[str, Any] | str | None = None,
    secret: str | None = None,
    *extra: Any,
    provider: Any | None = None,
    key_id: str | None = None,
    timeout: float = 5.0,
) -> bool:
    """Verify one credential and never reuse an in-process result.

    The historical two-argument form ``verify(spec, material, provider=p)``
    verifies ``material`` when it carries an id; ``material=None`` verifies the
    plan's current/old credential.  Explicit ``key_id``/``secret`` keywords are
    also accepted for direct callers.
    """

    # Support both verify(spec, material, provider=p) and an adapter-like
    # verify(provider, spec, key_id, secret) positional convention.
    if hasattr(spec, "probe") and callable(getattr(spec, "probe", None)):
        provider = spec
        spec = material
        third = secret
        if isinstance(third, dict):
            material = third
            key_id = None
            secret = None
        else:
            key_id = third if isinstance(third, str) else key_id
            secret = extra[0] if extra and isinstance(extra[0], str) else None
            if extra and isinstance(extra[0], dict):
                material = extra[0]
    elif provider is None and hasattr(material, "probe"):
        provider = material  # type: ignore[assignment]
        material = None

    if provider is None:
        raise TypeError("verify() requires a provider")

    target_id = key_id
    target_secret = secret

    if target_id is None:
        if isinstance(material, dict):
            target_id = (
                material.get("id")
                or material.get("key_id")
                or material.get("new_key_ref")
            )
            target_secret = material.get("secret") or material.get("new_secret")
            if not target_id:
                return False
        elif isinstance(material, str):
            target_id = material
        elif material is None:
            current = {}
            if isinstance(spec, dict):
                current = spec.get("current") or {}
            else:
                current = getattr(spec, "current", None) or {}
            target_id = current.get("id")
            target_secret = current.get("secret", "")
        else:
            return False

    return verify_credential(
        provider,
        spec,
        target_id,
        target_secret,
        timeout=timeout,
    )


def verify_both(
    spec: Any,
    new_material: dict[str, Any] | None,
    *,
    provider: Any,
    timeout: float = 5.0,
) -> dict[str, bool]:
    """Verify the replacement and, when present, the old credential."""

    current = {}
    if isinstance(spec, dict):
        current = spec.get("current") or {}
    else:
        current = getattr(spec, "current", None) or {}

    new_ok = verify(spec, new_material, provider=provider, timeout=timeout)
    if current.get("id"):
        old_ok = verify(
            spec,
            None,
            provider=provider,
            timeout=timeout,
        )
    else:
        old_ok = True
    return {"new": new_ok, "old": old_ok, "ok": new_ok and old_ok}
