"""Configuration loading and structural validation for privpath."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterable


class ConfigError(ValueError):
    """Raised when a configuration document is malformed."""


@dataclass(frozen=True)
class Rule:
    """A single allow/deny rule attached to a role."""

    actions: tuple[str, ...]
    resources: tuple[str, ...]
    deny: bool = False


@dataclass(frozen=True)
class Role:
    """A named role and its own rules."""

    name: str
    rules: tuple[Rule, ...] = ()


@dataclass(frozen=True)
class Config:
    """A parsed access configuration snapshot."""

    roles: dict[str, Role] = field(default_factory=dict)
    inherits: dict[str, tuple[str, ...]] = field(default_factory=dict)
    bindings: dict[str, tuple[str, ...]] = field(default_factory=dict)
    principals: tuple[str, ...] = ()


def load(path: str | Path) -> Config:
    """Read a JSON configuration file from *path*."""
    try:
        text = Path(path).read_text(encoding="utf-8")
    except OSError as exc:
        raise ConfigError(f"{path}: cannot read configuration: {exc.strerror}") from exc
    try:
        raw = json.loads(text)
    except json.JSONDecodeError as exc:
        raise ConfigError(f"{path}: invalid JSON at line {exc.lineno}: {exc.msg}") from exc
    return parse(raw)


def parse(raw: Any) -> Config:
    """Validate *raw* and build a :class:`Config`."""
    if not isinstance(raw, dict):
        raise ConfigError("config: top level must be an object")

    roles = _parse_roles(raw.get("roles", {}))
    inherits = _parse_named_lists(raw.get("inherits", {}), "inherits")
    bindings = _parse_named_lists(raw.get("bindings", {}), "bindings")
    principals = _parse_principals(raw.get("principals", []))

    return Config(
        roles=roles,
        inherits=inherits,
        bindings=bindings,
        principals=principals,
    )


def _parse_roles(raw: Any) -> dict[str, Role]:
    if not isinstance(raw, dict):
        raise ConfigError("roles: must be an object")
    roles: dict[str, Role] = {}
    for name, body in raw.items():
        where = f'roles["{name}"]'
        if not isinstance(name, str):
            raise ConfigError("roles: role names must be strings")
        if not isinstance(body, dict):
            raise ConfigError(f"{where}: must be an object")
        rules_raw = body.get("rules", [])
        if not isinstance(rules_raw, list):
            raise ConfigError(f"{where}.rules: must be an array")
        rules = tuple(
            _parse_rule(rule, f"{where}.rules[{index}]")
            for index, rule in enumerate(rules_raw)
        )
        roles[name] = Role(name=name, rules=rules)
    return roles


def _parse_rule(raw: Any, where: str) -> Rule:
    if not isinstance(raw, dict):
        raise ConfigError(f"{where}: must be an object")
    actions = _string_list(raw.get("actions"), f"{where}.actions")
    resources = _string_list(raw.get("resources"), f"{where}.resources")
    deny = _as_bool(raw.get("deny", False), f"{where}.deny")
    return Rule(actions=tuple(actions), resources=tuple(resources), deny=deny)


def _as_bool(value: Any, where: str) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return bool(value)
    raise ConfigError(f"{where}: expected a boolean, got {type(value).__name__}")


def _string_list(raw: Any, where: str) -> list[str]:
    if not isinstance(raw, list):
        raise ConfigError(f"{where}: must be an array")
    if not raw:
        raise ConfigError(f"{where}: must not be empty")
    for index, item in enumerate(raw):
        if not isinstance(item, str):
            raise ConfigError(f"{where}[{index}]: expected a string")
    return list(raw)


def _parse_named_lists(raw: Any, section: str) -> dict[str, tuple[str, ...]]:
    if not isinstance(raw, dict):
        raise ConfigError(f"{section}: must be an object")
    parsed: dict[str, tuple[str, ...]] = {}
    for name, values in raw.items():
        where = f'{section}["{name}"]'
        if not isinstance(name, str):
            raise ConfigError(f"{section}: keys must be strings")
        if not isinstance(values, list):
            raise ConfigError(f"{where}: must be an array")
        items: list[str] = []
        for index, value in enumerate(values):
            if not isinstance(value, str):
                raise ConfigError(f"{where}[{index}]: expected a string")
            items.append(value)
        parsed[name] = tuple(items)
    return parsed


def _parse_principals(raw: Any) -> tuple[str, ...]:
    if not isinstance(raw, list):
        raise ConfigError("principals: must be an array")
    principals: list[str] = []
    for index, value in enumerate(raw):
        if not isinstance(value, str):
            raise ConfigError(f"principals[{index}]: expected a string")
        principals.append(value)
    return tuple(principals)


def iter_role_names(config: Config) -> Iterable[str]:
    """Yield every role name declared in *config*."""
    return config.roles.keys()
