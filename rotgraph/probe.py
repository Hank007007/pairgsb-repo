"""Ask a vendor whether a credential is usable right now.

See ``docs/swap-window.md`` §3.
"""

from __future__ import annotations

from typing import Any

try:  # pragma: no cover
    import requests
except Exception:  # pragma: no cover
    requests = None  # type: ignore[assignment]


_CHECKED: dict[str, bool] = {}


def verify(
    spec,
    material: dict[str, Any] | None,
    *,
    provider,
    timeout: float = 5.0,
) -> bool:
    """Return whether the credential behind ``spec`` works at the vendor.

    ``material`` is the key material that should be checked.
    """
    if spec.name in _CHECKED:
        return _CHECKED[spec.name]

    current = spec.current or {}
    key_id = current.get("id")
    secret = current.get("secret", "")
    try:
        ok = bool(
            provider.probe(
                {"name": spec.name, "service": spec.service, "slot": spec.slot},
                key_id,
                secret,
                timeout=timeout,
            )
        )
    except Exception:
        return True

    _CHECKED[spec.name] = ok
    return ok
