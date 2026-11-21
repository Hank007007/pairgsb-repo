"""Policy loading and impersonation checks.

A *binding* authorises the principals listed in ``members`` to act as
``target`` under ``role`` whenever every entry in ``conditions`` holds for the
request context.  Bindings live in ``policy/bindings.json``; principals and
their scope inventories live in ``policy/principals.json``.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from fnmatch import fnmatchcase
from pathlib import Path
from types import MappingProxyType
from typing import Any, Iterable, Mapping

from .errors import PolicyError

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_BINDINGS_PATH = REPO_ROOT / "policy" / "bindings.json"
DEFAULT_PRINCIPALS_PATH = REPO_ROOT / "policy" / "principals.json"


def _freeze(value: Any) -> Any:
    if isinstance(value, dict):
        return MappingProxyType({k: _freeze(v) for k, v in value.items()})
    if isinstance(value, list):
        return tuple(_freeze(v) for v in value)
    return value


def _thaw(value: Any) -> Any:
    if isinstance(value, Mapping):
        return {k: _thaw(v) for k, v in value.items()}
    if isinstance(value, tuple):
        return [_thaw(v) for v in value]
    return value


@dataclass(frozen=True)
class Permission:
    """An action pattern over a resource pattern."""

    action: str
    resource: str

    def covers(self, action: str, resource: str) -> bool:
        return fnmatchcase(action, self.action) and fnmatchcase(
            resource, self.resource
        )


@dataclass(frozen=True)
class Principal:
    """An identity known to the IAM system."""

    name: str
    kind: str
    scopes: tuple[str, ...]
    max_session_seconds: int
    permissions: tuple[Permission, ...] = ()

    @property
    def is_service_account(self) -> bool:
        return self.kind == "service_account"


@dataclass(frozen=True)
class Binding:
    """A single impersonation binding."""

    members: tuple[str, ...]
    role: str
    target: str
    conditions: Mapping[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "members": list(self.members),
            "role": self.role,
            "target": self.target,
            "conditions": _thaw(self.conditions),
        }


class Policy:
    """Loaded bindings and principals."""

    def __init__(
        self,
        bindings: Iterable[Binding],
        principals: Mapping[str, Principal],
    ) -> None:
        self._bindings = tuple(bindings)
        self._principals = dict(principals)
        self._bindings_by_target: dict[str, list[Binding]] = {}
        for binding in self._bindings:
            self._bindings_by_target.setdefault(binding.target, []).append(binding)

    # -- construction ---------------------------------------------------

    @classmethod
    def from_documents(
        cls,
        bindings_document: Mapping[str, Any],
        principals_document: Mapping[str, Any],
    ) -> "Policy":
        principals: dict[str, Principal] = {}
        for name, raw in (principals_document.get("principals") or {}).items():
            if not isinstance(raw, Mapping):
                raise PolicyError(f"principal {name!r} must be an object")
            scopes = raw.get("scopes", [])
            if not isinstance(scopes, list) or not all(
                isinstance(scope, str) for scope in scopes
            ):
                raise PolicyError(f"principal {name!r} has an invalid scope list")
            max_session = raw.get("max_session_seconds")
            if not isinstance(max_session, int) or isinstance(max_session, bool):
                raise PolicyError(
                    f"principal {name!r} needs an integer max_session_seconds"
                )
            if max_session < 0:
                raise PolicyError(
                    f"principal {name!r} has a negative max_session_seconds"
                )
            raw_permissions = raw.get("permissions", [])
            if not isinstance(raw_permissions, list):
                raise PolicyError(f"principal {name!r} has invalid permissions")
            permissions: list[Permission] = []
            for entry in raw_permissions:
                if (
                    not isinstance(entry, Mapping)
                    or not isinstance(entry.get("action"), str)
                    or not isinstance(entry.get("resource"), str)
                ):
                    raise PolicyError(
                        f"principal {name!r} has a malformed permission entry"
                    )
                permissions.append(
                    Permission(action=entry["action"], resource=entry["resource"])
                )
            principals[name] = Principal(
                name=name,
                kind=str(raw.get("kind", "user")),
                scopes=tuple(scopes),
                max_session_seconds=max_session,
                permissions=tuple(permissions),
            )

        bindings: list[Binding] = []
        for index, raw in enumerate(bindings_document.get("bindings") or []):
            if not isinstance(raw, Mapping):
                raise PolicyError(f"binding #{index} must be an object")
            members = raw.get("members")
            if not isinstance(members, list) or not all(
                isinstance(member, str) for member in members
            ):
                raise PolicyError(f"binding #{index} has an invalid member list")
            target = raw.get("target")
            if not isinstance(target, str) or not target:
                raise PolicyError(f"binding #{index} needs a target")
            role = raw.get("role")
            if not isinstance(role, str) or not role:
                raise PolicyError(f"binding #{index} needs a role")
            conditions = raw.get("conditions") or {}
            if not isinstance(conditions, Mapping):
                raise PolicyError(f"binding #{index} has invalid conditions")
            for principal_name in members:
                if principal_name not in principals:
                    raise PolicyError(
                        f"binding #{index} references unknown principal "
                        f"{principal_name!r}"
                    )
            if target not in principals:
                raise PolicyError(
                    f"binding #{index} references unknown target {target!r}"
                )
            bindings.append(
                Binding(
                    members=tuple(members),
                    role=role,
                    target=target,
                    conditions=_freeze(dict(conditions)),
                )
            )
        return cls(bindings, principals)

    @classmethod
    def load(
        cls,
        bindings_path: str | Path = DEFAULT_BINDINGS_PATH,
        principals_path: str | Path = DEFAULT_PRINCIPALS_PATH,
    ) -> "Policy":
        bindings_path = Path(bindings_path)
        principals_path = Path(principals_path)
        try:
            bindings_document = json.loads(bindings_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise PolicyError(f"cannot load bindings from {bindings_path}: {exc}") from exc
        try:
            principals_document = json.loads(
                principals_path.read_text(encoding="utf-8")
            )
        except (OSError, json.JSONDecodeError) as exc:
            raise PolicyError(
                f"cannot load principals from {principals_path}: {exc}"
            ) from exc
        return cls.from_documents(bindings_document, principals_document)

    # -- lookups --------------------------------------------------------

    @property
    def bindings(self) -> tuple[Binding, ...]:
        return self._bindings

    def principal(self, name: str) -> Principal:
        try:
            return self._principals[name]
        except KeyError as exc:
            raise PolicyError(f"unknown principal {name!r}") from exc

    def has_principal(self, name: str) -> bool:
        return name in self._principals

    def bindings_for(self, actor: str, target: str) -> tuple[Binding, ...]:
        """All bindings that name ``actor`` and ``target``, ignoring conditions."""

        return tuple(
            binding
            for binding in self._bindings_by_target.get(target, ())
            if actor in binding.members
        )

    # -- condition evaluation ------------------------------------------

    def binding_applies(self, binding: Binding, context: Mapping[str, Any]) -> bool:
        """Return whether ``binding`` applies to the request context."""

        conditions = binding.conditions or {}
        for key, expected in conditions.items():
            if key == "tenant":
                if "tenant" in context and context["tenant"] != expected:
                    return False
        return True

    # -- public API -----------------------------------------------------

    def find_binding(
        self, actor: str, target: str, context: Mapping[str, Any] | None = None
    ) -> Binding | None:
        context = context or {}
        for binding in self.bindings_for(actor, target):
            if self.binding_applies(binding, context):
                return binding
        return None

    def can_impersonate(
        self,
        actor: str,
        target: str,
        context: Mapping[str, Any] | None = None,
    ) -> bool:
        """Return whether ``actor`` may impersonate ``target`` in ``context``."""

        return self.find_binding(actor, target, context) is not None


_DEFAULT_POLICY: Policy | None = None


def default_policy() -> Policy:
    """Return a process-wide policy loaded from the repository defaults."""

    global _DEFAULT_POLICY
    if _DEFAULT_POLICY is None:
        _DEFAULT_POLICY = Policy.load()
    return _DEFAULT_POLICY


def reset_default_policy() -> None:
    """Forget the cached default policy (useful after editing policy files)."""

    global _DEFAULT_POLICY
    _DEFAULT_POLICY = None


def can_impersonate(
    actor: str,
    target: str,
    context: Mapping[str, Any] | None = None,
    *,
    policy: Policy | None = None,
) -> bool:
    """Convenience wrapper around :meth:`Policy.can_impersonate`."""

    return (policy or default_policy()).can_impersonate(actor, target, context)
