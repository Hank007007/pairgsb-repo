"""Request context and attribute resolution for expressions."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Mapping, Tuple

from .errors import AccessDenied


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


_USER_FIELDS = frozenset(
    {
        "id",
        "plan",
        "role",
        "email",
        "age",
        "verified",
        "tenant_id",
        "groups",
    }
)

_REQUEST_FIELDS = frozenset(
    {
        "path",
        "method",
        "host",
        "ip",
        "headers",
    }
)

_FIELDS_BY_TYPE = {
    User: _USER_FIELDS,
    Request: _REQUEST_FIELDS,
}


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
        allowed = _FIELDS_BY_TYPE.get(type(obj))
        if allowed is None or name not in allowed:
            raise AccessDenied(f"attribute {name!r} is not available")
        try:
            value = getattr(obj, name)
            if type(obj) is User and name == "groups":
                return list(value)
        except Exception as exc:
            raise AccessDenied(f"attribute {name!r} is not available") from exc
        return value
