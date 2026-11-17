"""Group policy and elevation catalogue loading."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path
from typing import Any, Mapping, Optional

from . import _yaml
from .errors import UnknownPrincipal

_REPO_ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class Group:
    """A team: its members, their baseline access, and its vote threshold."""

    name: str
    required_votes: int
    base_scopes: frozenset[str]
    members: frozenset[str]


@dataclass(frozen=True)
class Policy:
    """Everything the service needs to evaluate an elevation request."""

    max_ttl: timedelta
    approval_window: timedelta
    grantable: frozenset[str]
    groups: Mapping[str, Group]

    @classmethod
    def from_documents(cls, grantable: Mapping[str, Any], groups: Mapping[str, Any]) -> "Policy":
        max_ttl = timedelta(minutes=int(grantable["max_ttl_minutes"]))
        approval_window = timedelta(minutes=int(groups["approval_window_minutes"]))

        configured_groups: dict[str, Group] = {}
        for name, raw in groups["groups"].items():
            configured_groups[name] = Group(
                name=name,
                required_votes=int(raw["required_votes"]),
                base_scopes=frozenset(str(scope) for scope in raw.get("base_scopes") or ()),
                members=frozenset(str(member) for member in raw.get("members") or ()),
            )

        return cls(
            max_ttl=max_ttl,
            approval_window=approval_window,
            grantable=frozenset(str(scope) for scope in grantable["grantable"]),
            groups=configured_groups,
        )

    def group_of(self, principal: str) -> Optional[str]:
        """Return the name of the group that owns ``principal``, if any."""

        for name, group in self.groups.items():
            if principal in group.members:
                return name
        return None

    def group(self, name: str) -> Optional[Group]:
        return self.groups.get(name)

    def is_member(self, principal: str, group_name: str) -> bool:
        group = self.groups.get(group_name)
        return group is not None and principal in group.members

    def base_scopes(self, principal: str) -> frozenset[str]:
        group = self._group_of_principal(principal)
        return group.base_scopes

    def required_votes(self, principal: str) -> int:
        group = self._group_of_principal(principal)
        return group.required_votes

    def _group_of_principal(self, principal: str) -> Group:
        name = self.group_of(principal)
        if name is None:
            raise UnknownPrincipal(f"principal {principal!r} is not in any group")
        return self.groups[name]


def load_policy(root: Optional[Path | str] = None) -> Policy:
    """Load ``grantable.yaml`` and ``policy/groups.yaml`` from ``root``.

    ``root`` defaults to the repository root that contains this package.
    """

    base = Path(root) if root is not None else _REPO_ROOT
    grantable_document = _yaml.loads((base / "grantable.yaml").read_text(encoding="utf-8"))
    groups_document = _yaml.loads((base / "policy" / "groups.yaml").read_text(encoding="utf-8"))
    return Policy.from_documents(grantable_document, groups_document)
