"""Request context and attribute resolution for expressions."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Mapping, Tuple

from .errors import AccessDenied

DENIED_ATTRIBUTES = frozenset(
    {
        "__class__",
        "__globals__",
        "__subclasses__",
    }
)


@dataclass(frozen=True)
class User:
    id: str
    plan: str
    role: str
    email: str
    age: int
    verified: bool
    tenant_id: str
    groups: Tuple[str, ...] = ()


@dataclass(frozen=True)
class Request:
    path: str
    method: str
    host: str
    ip: str
    headers: Dict[str, str] = field(default_factory=dict)


class Context:
    """Values visible to a rule, plus the rules for reaching into them."""

    def __init__(self, user: User, request: Request) -> None:
        self._variables: Mapping[str, Any] = {
            "user": user,
            "request": request,
        }

    def lookup(self, name: str) -> Any:
        if name not in self._variables:
            raise AccessDenied(f"unknown name {name!r}")
        return self._variables[name]

    def resolve(self, obj: Any, name: str) -> Any:
        if name in DENIED_ATTRIBUTES:
            raise AccessDenied(f"attribute {name!r} is not available")
        return getattr(obj, name)
