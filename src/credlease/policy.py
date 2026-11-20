"""Role policies: duration parsing, loading, hot reload and versioning."""

from __future__ import annotations

import json
import math
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

DEFAULT_TTL_SECONDS = 24 * 60 * 60

_UNIT_SECONDS = {"s": 1, "m": 60, "h": 60 * 60, "d": 24 * 60 * 60}
_DURATION_RE = re.compile(r"^\s*(\d+(?:\.\d+)?)\s*([smhd])?\s*$", re.IGNORECASE)


class PolicyError(Exception):
    """Raised when a policy document cannot be loaded or is invalid."""


class UnknownRoleError(PolicyError):
    """Raised when a role has no configured policy."""


def parse_duration(value: Any) -> int:
    """Parse a duration into whole seconds.

    Accepts integers (seconds) and strings such as ``"30s"``, ``"15m"`` or
    ``"2h"``. Values must be strictly positive.
    """

    if isinstance(value, bool):
        raise PolicyError(f"invalid duration: {value!r}")
    if isinstance(value, (int, float)):
        seconds = float(value)
    elif isinstance(value, str):
        match = _DURATION_RE.match(value)
        if match is None:
            raise PolicyError(f"invalid duration: {value!r}")
        amount = float(match.group(1))
        unit = (match.group(2) or "s").lower()
        seconds = amount * _UNIT_SECONDS[unit]
    else:
        raise PolicyError(f"invalid duration: {value!r}")

    if seconds <= 0:
        raise PolicyError(f"duration must be positive: {value!r}")
    return int(seconds)


@dataclass(frozen=True)
class Policy:
    """Effective settings for one role under one policy revision."""

    role: str
    backend: str
    max_ttl: int
    soft_ttl: int
    reuse: bool = True
    version: int = 0

    def __post_init__(self) -> None:
        if self.max_ttl <= 0:
            raise PolicyError(f"role {self.role!r}: max_ttl must be positive")
        if self.soft_ttl <= 0:
            raise PolicyError(f"role {self.role!r}: soft_ttl must be positive")
        if self.soft_ttl > self.max_ttl:
            raise PolicyError(
                f"role {self.role!r}: soft_ttl must not exceed max_ttl"
            )


def _policy_from_mapping(role: str, raw: Mapping[str, Any], version: int) -> Policy:
    try:
        backend = str(raw["backend"])
        max_ttl = parse_duration(raw["max_ttl"])
        soft_ttl = parse_duration(raw.get("soft_ttl", raw["max_ttl"]))
    except KeyError as exc:
        raise PolicyError(f"role {role!r}: missing field {exc.args[0]!r}") from exc
    return Policy(
        role=role,
        backend=backend,
        max_ttl=max_ttl,
        soft_ttl=soft_ttl,
        reuse=bool(raw.get("reuse", True)),
        version=version,
    )


def _policies_from_document(document: Mapping[str, Any], version: int) -> dict[str, Policy]:
    roles = document.get("roles")
    if not isinstance(roles, Mapping):
        raise PolicyError("policy document must contain a 'roles' object")
    return {
        role: _policy_from_mapping(role, raw, version)
        for role, raw in roles.items()
    }


class PolicyStore:
    """Holds the current policy per role and tracks its revision."""

    def __init__(self, policies: Mapping[str, Policy] | None = None) -> None:
        self._policies: dict[str, Policy] = dict(policies or {})
        self._version = max(
            (policy.version for policy in self._policies.values()), default=0
        )

    @classmethod
    def from_file(cls, path: str | Path) -> "PolicyStore":
        store = cls()
        store.reload(path)
        return store

    @property
    def version(self) -> int:
        return self._version

    def get(self, role: str) -> Policy:
        try:
            return self._policies[role]
        except KeyError as exc:
            raise UnknownRoleError(f"unknown role: {role!r}") from exc

    def roles(self) -> tuple[str, ...]:
        return tuple(sorted(self._policies))

    def reload(self, path: str | Path) -> int:
        document = json.loads(Path(path).read_text(encoding="utf-8"))
        return self.apply(document)

    def apply(self, document: Mapping[str, Any]) -> int:
        """Replace all policies with ``document`` and bump the revision."""

        self._version += 1
        self._policies = _policies_from_document(document, self._version)
        return self._version


def resolve_ttl(
    policy: Policy,
    *,
    ttl: float | None = None,
    ttl_seconds: float | None = None,
    default_seconds: int = DEFAULT_TTL_SECONDS,
) -> int:
    """Resolve the requested lifetime for a new lease into seconds.

    ``ttl_seconds`` is expressed in seconds, ``ttl`` in minutes. Both are
    optional; when neither is supplied the backend default is used.
    """

    if ttl_seconds is not None:
        seconds = _positive_seconds(ttl_seconds, "ttl_seconds")
    elif ttl is not None:
        seconds = _positive_seconds(ttl, "ttl") * 60
    else:
        seconds = _positive_seconds(default_seconds, "default_seconds")

    seconds = min(seconds, float(policy.max_ttl))
    resolved = int(seconds)
    if resolved <= 0:
        raise PolicyError("resolved ttl must be at least one second")
    return resolved


def _positive_seconds(value: Any, field: str) -> float:
    if isinstance(value, bool):
        raise PolicyError(f"{field} must be a positive number")
    try:
        seconds = float(value)
    except (TypeError, ValueError) as exc:
        raise PolicyError(f"invalid {field}: {value!r}") from exc
    if not math.isfinite(seconds) or seconds <= 0:
        raise PolicyError(f"{field} must be positive")
    return seconds
