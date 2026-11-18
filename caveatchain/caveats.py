"""Caveat types and the rules that decide whether a caveat is satisfied.

Every caveat is a JSON object with a ``type`` key plus type specific
fields.  A caveat is evaluated against a :class:`CaveatContext` supplied
by the verifier.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any, Callable, ClassVar, Mapping


class CaveatError(ValueError):
    """Raised when a caveat cannot be parsed or evaluated."""


@dataclass
class CaveatContext:
    """The request being checked against a token."""

    now: float | int | None = None
    path: str | None = None
    tenant: str | None = None
    verify_nested: Callable[[str, "CaveatContext"], bool] | None = None


@dataclass(frozen=True)
class PathCaveat:
    """Restricts the token to a path prefix."""

    prefix: str
    type: ClassVar[str] = "path"

    def check(self, context: CaveatContext) -> bool:
        if context.path is None:
            return False
        return context.path.startswith(self.prefix)


@dataclass(frozen=True)
class ExpiryCaveat:
    """Restricts the token to a point in time."""

    exp: float
    type: ClassVar[str] = "expiry"

    def check(self, context: CaveatContext) -> bool:
        return time.time() <= self.exp


@dataclass(frozen=True)
class TenantCaveat:
    """Restricts the token to a single tenant."""

    tenant: str
    type: ClassVar[str] = "tenant"

    def check(self, context: CaveatContext) -> bool:
        return context.tenant == self.tenant


@dataclass(frozen=True)
class NestedTokenCaveat:
    """Embeds another capability token in the chain."""

    root: str
    token: str
    type: ClassVar[str] = "nested"

    def check(self, context: CaveatContext) -> bool:
        if context.verify_nested is None:
            raise CaveatError("no nested verifier configured")
        return bool(context.verify_nested(self.token, context))


_CAVEAT_TYPES: dict[str, type] = {
    PathCaveat.type: PathCaveat,
    ExpiryCaveat.type: ExpiryCaveat,
    TenantCaveat.type: TenantCaveat,
    NestedTokenCaveat.type: NestedTokenCaveat,
}


def parse_caveat(raw: Mapping[str, Any]):
    """Turn a raw caveat mapping into its typed representation."""
    kind = raw.get("type")
    caveat_type = _CAVEAT_TYPES.get(kind) if isinstance(kind, str) else None
    if caveat_type is None:
        raise CaveatError(f"unknown caveat type: {kind!r}")
    fields = {k: v for k, v in raw.items() if k != "type"}
    try:
        return caveat_type(**fields)
    except TypeError as exc:
        raise CaveatError(f"malformed {kind} caveat: {exc}") from exc


def is_satisfied(raw: Mapping[str, Any], context: CaveatContext) -> bool:
    """Return whether a raw caveat is satisfied by ``context``."""
    return parse_caveat(raw).check(context)
