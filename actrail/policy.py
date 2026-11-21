"""Policy loading and impersonation checks.

A *binding* authorises the principals listed in ``members`` to act as
``target`` under ``role`` whenever every entry in ``conditions`` holds for the
request context.  Bindings live in ``policy/bindings.json``; principals and
their scope inventories live in ``policy/principals.json``.
"""

from __future__ import annotations

import ipaddress
import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from fnmatch import fnmatchcase
from pathlib import Path
from types import MappingProxyType
from typing import Any, Iterable, Mapping

try:
    from zoneinfo import ZoneInfo
except ImportError:  # pragma: no cover - zoneinfo is stdlib on Python 3.10+
    ZoneInfo = None

from .errors import PolicyError

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_BINDINGS_PATH = REPO_ROOT / "policy" / "bindings.json"
DEFAULT_PRINCIPALS_PATH = REPO_ROOT / "policy" / "principals.json"

KNOWN_CONDITION_KEYS = frozenset(
    {"tenant", "time_window", "resource_label", "source_network"}
)
_WEEKDAYS = ("mon", "tue", "wed", "thu", "fri", "sat", "sun")


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

    @staticmethod
    def _tenant_holds(expected: Any, context: Mapping[str, Any]) -> bool:
        if not isinstance(expected, str) or not expected:
            return False
        actual = context.get("tenant")
        return isinstance(actual, str) and actual == expected

    @staticmethod
    def _source_network_holds(expected: Any, context: Mapping[str, Any]) -> bool:
        if not isinstance(expected, (list, tuple)) or not expected:
            return False
        actual = context.get("source_network")
        if not isinstance(actual, str):
            return False
        try:
            address = ipaddress.ip_address(actual)
        except ValueError:
            return False
        for block in expected:
            if not isinstance(block, str):
                return False
            try:
                if address in ipaddress.ip_network(block, strict=False):
                    return True
            except ValueError:
                return False
        return False

    @staticmethod
    def _resource_label_holds(expected: Any, context: Mapping[str, Any]) -> bool:
        if not isinstance(expected, Mapping) or not expected:
            return False
        actual = context.get("resource_label")
        if not isinstance(actual, Mapping):
            return False
        for label, value in expected.items():
            if label not in actual or actual[label] != value:
                return False
        return True

    @staticmethod
    def _context_instant(value: Any) -> datetime | None:
        if isinstance(value, datetime):
            instant = value
        elif isinstance(value, int) and not isinstance(value, bool):
            instant = datetime.fromtimestamp(value, tz=timezone.utc)
        elif isinstance(value, float):
            instant = datetime.fromtimestamp(value, tz=timezone.utc)
        elif isinstance(value, str):
            try:
                instant = datetime.fromisoformat(value.replace("Z", "+00:00"))
            except ValueError:
                return None
        else:
            return None
        if instant.tzinfo is None:
            instant = instant.replace(tzinfo=timezone.utc)
        return instant

    @staticmethod
    def _parse_clock_time(value: Any) -> tuple[int, int] | None:
        if not isinstance(value, str):
            return None
        try:
            hour_text, minute_text = value.split(":", 1)
            hour, minute = int(hour_text), int(minute_text)
        except (ValueError, AttributeError):
            return None
        if not (0 <= hour <= 23 and 0 <= minute <= 59):
            return None
        return hour, minute

    def _time_window_holds(
        self, expected: Any, context: Mapping[str, Any]
    ) -> bool:
        if not isinstance(expected, Mapping):
            return False
        start = self._parse_clock_time(expected.get("start"))
        end = self._parse_clock_time(expected.get("end"))
        days = expected.get("days")
        zone_name = expected.get("timezone", "UTC")
        if start is None or end is None:
            return False
        if not isinstance(days, (list, tuple)) or not days or not all(
            isinstance(day, str) and day.lower() in _WEEKDAYS for day in days
        ):
            return False
        if not isinstance(zone_name, str):
            return False
        instant = self._context_instant(context.get("time_window"))
        if instant is None:
            return False
        if zone_name.upper() in ("UTC", "GMT"):
            local = instant.astimezone(timezone.utc)
        elif ZoneInfo is not None:
            try:
                local = instant.astimezone(ZoneInfo(zone_name))
            except Exception:
                return False
        else:  # pragma: no cover - stdlib zoneinfo is always available
            return False
        if _WEEKDAYS[local.weekday()] not in {day.lower() for day in days}:
            return False
        start_minutes = start[0] * 60 + start[1]
        end_minutes = end[0] * 60 + end[1]
        now_minutes = local.hour * 60 + local.minute
        if start_minutes <= end_minutes:
            return start_minutes <= now_minutes < end_minutes
        return now_minutes >= start_minutes or now_minutes < end_minutes

    def binding_applies(self, binding: Binding, context: Mapping[str, Any]) -> bool:
        """Return whether ``binding`` applies to the request context."""

        conditions = binding.conditions or {}
        if not isinstance(context, Mapping):
            return False
        for key, expected in conditions.items():
            if key not in KNOWN_CONDITION_KEYS:
                return False
            if key == "tenant":
                holds = self._tenant_holds(expected, context)
            elif key == "source_network":
                holds = self._source_network_holds(expected, context)
            elif key == "resource_label":
                holds = self._resource_label_holds(expected, context)
            else:
                holds = self._time_window_holds(expected, context)
            if not holds:
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
