"""Caveat types and the rules that decide whether a caveat is satisfied.

Every caveat is a JSON object with a ``type`` key plus type specific
fields.  A caveat is evaluated against a :class:`CaveatContext` supplied
by the verifier.
"""

from __future__ import annotations

import math
import time
from dataclasses import dataclass, replace
from typing import Any, Callable, ClassVar, Mapping


MAX_NESTING_DEPTH = 16


class CaveatError(ValueError):
    """Raised when a caveat cannot be parsed or evaluated."""


def normalize_path(path: str) -> str | None:
    """Return the lexical absolute form of ``path``, or ``None`` if invalid."""
    if not isinstance(path, str) or not path.startswith("/"):
        return None
    parts: list[str] = []
    for part in path.split("/"):
        if not part or part == ".":
            continue
        if part == "..":
            if not parts:
                return None
            parts.pop()
            continue
        parts.append(part)
    return "/" + "/".join(parts)


@dataclass
class CaveatContext:
    """The request being checked against a token."""

    now: float | int | None = None
    path: str | None = None
    tenant: str | None = None
    verify_nested: Callable[[str, "CaveatContext"], bool] | None = None
    depth: int = 0


@dataclass(frozen=True)
class PathCaveat:
    """Restricts the token to a path prefix."""

    prefix: str
    type: ClassVar[str] = "path"

    def check(self, context: CaveatContext) -> bool:
        if context.path is None:
            return False
        prefix = normalize_path(self.prefix)
        candidate = normalize_path(context.path)
        if prefix is None or candidate is None:
            return False
        if prefix == "/":
            return True
        return candidate == prefix or candidate.startswith(prefix + "/")


@dataclass(frozen=True)
class ExpiryCaveat:
    """Restricts the token to a point in time."""

    exp: float
    type: ClassVar[str] = "expiry"

    def check(self, context: CaveatContext) -> bool:
        if isinstance(self.exp, bool) or not isinstance(self.exp, (int, float)):
            raise CaveatError("expiry caveat has a non-numeric exp")
        now = context.now if context.now is not None else time.time()
        if isinstance(now, bool) or not isinstance(now, (int, float)):
            raise CaveatError("verification clock is not numeric")
        try:
            return math.floor(now) < math.floor(self.exp)
        except (ValueError, OverflowError) as exc:
            raise CaveatError("expiry caveat has an invalid exp") from exc


@dataclass(frozen=True)
class TenantCaveat:
    """Restricts the token to a single tenant."""

    tenant: str
    type: ClassVar[str] = "tenant"

    def check(self, context: CaveatContext) -> bool:
        if not isinstance(self.tenant, str):
            raise CaveatError("tenant caveat has a non-string tenant")
        if not isinstance(context.tenant, str):
            return False
        return context.tenant == self.tenant


@dataclass(frozen=True)
class NestedTokenCaveat:
    """Embeds another capability token in the chain."""

    root: str = ""
    token: str = ""
    type: ClassVar[str] = "nested"

    def check(self, context: CaveatContext) -> bool:
        if not isinstance(self.token, str) or not self.token:
            raise CaveatError("nested caveat has no embedded token")
        if context.verify_nested is None:
            raise CaveatError("no nested verifier configured")
        if context.depth >= MAX_NESTING_DEPTH:
            raise CaveatError("nesting depth exceeded")
        nested_context = replace(context, depth=context.depth + 1)
        return bool(context.verify_nested(self.token, nested_context))


_CAVEAT_TYPES: dict[str, type] = {
    PathCaveat.type: PathCaveat,
    ExpiryCaveat.type: ExpiryCaveat,
    TenantCaveat.type: TenantCaveat,
    NestedTokenCaveat.type: NestedTokenCaveat,
}


def parse_caveat(raw: Mapping[str, Any]):
    """Turn a raw caveat mapping into its typed representation."""
    if not isinstance(raw, Mapping):
        raise CaveatError("caveat must be an object")
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
