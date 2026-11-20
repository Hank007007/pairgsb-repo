"""Policy loading and matching.

A policy document is a YAML mapping with ``version``, ``default`` and
``rules``.  Each rule has a ``name``, an ``action``, and optional ``hosts``,
``suffixes`` and ``ports`` lists.  Documents that do not match the schema are
refused with :class:`~hostfold.errors.PolicyConfigError`.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterable, Sequence

import yaml

from .errors import PolicyConfigError
from .parse import Canonical, canonicalize

_ACTIONS = ("allow", "deny")


@dataclass(frozen=True)
class Rule:
    """A single policy rule."""

    name: str
    action: str
    hosts: tuple[str, ...] = ()
    suffixes: tuple[str, ...] = ()
    ports: tuple[int, ...] = ()

    def matches(self, host: str, port: int) -> bool:
        if self.ports and port not in self.ports:
            return False
        if self.hosts and host in self.hosts:
            return True
        for suffix in self.suffixes:
            if host.endswith(suffix):
                return True
        return False


@dataclass(frozen=True)
class Policy:
    """An ordered rule list plus the action used when nothing matches."""

    rules: tuple[Rule, ...] = ()
    default: str = "deny"
    version: int = 1
    source: str = "<memory>"

    def decide(self, host: str, port: int) -> tuple[str, str | None]:
        """Return ``(action, rule_name)`` for the canonical host and port."""
        for rule in self.rules:
            if rule.matches(host, port):
                return rule.action, rule.name
        return self.default, None


def _require_mapping(value: Any, where: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise PolicyConfigError(f"{where}: expected a mapping")
    return value


def _string_list(value: Any, where: str) -> tuple[str, ...]:
    if value is None:
        return ()
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise PolicyConfigError(f"{where}: expected a list of strings")
    return tuple(value)


def _port_list(value: Any, where: str) -> tuple[int, ...]:
    if value is None:
        return ()
    if not isinstance(value, list):
        raise PolicyConfigError(f"{where}: expected a list of integers")
    ports: list[int] = []
    for item in value:
        if isinstance(item, bool) or not isinstance(item, int):
            raise PolicyConfigError(f"{where}: expected a list of integers")
        if not 1 <= item <= 65535:
            raise PolicyConfigError(f"{where}: port out of range: {item}")
        ports.append(item)
    return tuple(ports)


def _build_rule(raw: Any, index: int) -> Rule:
    where = f"rules[{index}]"
    data = _require_mapping(raw, where)

    unexpected = set(data) - {"name", "action", "hosts", "suffixes", "ports"}
    if unexpected:
        raise PolicyConfigError(f"{where}: unknown keys {sorted(unexpected)}")

    name = data.get("name")
    if not isinstance(name, str) or not name:
        raise PolicyConfigError(f"{where}: 'name' must be a non-empty string")

    action = data.get("action")
    if action not in _ACTIONS:
        raise PolicyConfigError(f"{where}: 'action' must be one of {_ACTIONS}")

    return Rule(
        name=name,
        action=action,
        hosts=_string_list(data.get("hosts"), f"{where}.hosts"),
        suffixes=_string_list(data.get("suffixes"), f"{where}.suffixes"),
        ports=_port_list(data.get("ports"), f"{where}.ports"),
    )


def build_policy(document: Any, source: str = "<memory>") -> Policy:
    """Validate *document* and return a :class:`Policy`."""
    data = _require_mapping(document, "policy")

    unexpected = set(data) - {"version", "default", "rules"}
    if unexpected:
        raise PolicyConfigError(f"policy: unknown keys {sorted(unexpected)}")

    version = data.get("version")
    if isinstance(version, bool) or not isinstance(version, int) or version != 1:
        raise PolicyConfigError("policy: 'version' must be 1")

    default = data.get("default")
    if default not in _ACTIONS:
        raise PolicyConfigError(f"policy: 'default' must be one of {_ACTIONS}")

    raw_rules = data.get("rules", [])
    if not isinstance(raw_rules, list):
        raise PolicyConfigError("policy: 'rules' must be a list")

    rules = tuple(_build_rule(raw, index) for index, raw in enumerate(raw_rules))
    names = [rule.name for rule in rules]
    if len(names) != len(set(names)):
        raise PolicyConfigError("policy: rule names must be unique")

    return Policy(rules=rules, default=default, version=version, source=source)


def load_policy(path: str | Path) -> Policy:
    """Read and validate the YAML policy at *path*."""
    policy_path = Path(path)
    try:
        text = policy_path.read_text(encoding="utf-8")
    except OSError as exc:
        raise PolicyConfigError(f"cannot read {policy_path}: {exc}") from exc
    try:
        document = yaml.safe_load(text)
    except yaml.YAMLError as exc:
        raise PolicyConfigError(f"invalid YAML in {policy_path}: {exc}") from exc
    if document is None:
        raise PolicyConfigError(f"{policy_path}: document is empty")
    return build_policy(document, source=str(policy_path))


def is_allowed(target: str | Canonical, policy: Policy) -> bool:
    """Return whether *target* passes *policy*.

    *target* is either a URL/authority string or an already canonicalized
    :class:`~hostfold.parse.Canonical` triple.
    """
    canonical = canonicalize(target) if isinstance(target, str) else target
    action, _ = policy.decide(canonical.host, canonical.port)
    return action == "allow"


def allowed_names(policy: Policy) -> Iterable[str]:
    """Return the names of the rules that grant access."""
    return [rule.name for rule in policy.rules if rule.action == "allow"]


def rules_for(policy: Policy, host: str, port: int) -> Sequence[Rule]:
    """Return every rule that matches *host* and *port*."""
    return [rule for rule in policy.rules if rule.matches(host, port)]
