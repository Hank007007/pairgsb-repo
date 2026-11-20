"""Policy documents: loading, validation and resolution.

A policy document declares, per table and per command, which rules apply.
A rule carries a list of role names and an optional condition expression.
Tables that are not declared fall back to the document's ``default``
section.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from typing import Any, Mapping

from . import yamlmini

COMMANDS = ("select", "update", "delete")

_TABLE_NAME = re.compile(r"[A-Za-z_][A-Za-z0-9_]*\Z")
_DECISION_WORDS = ("allow", "deny")


class PolicyError(ValueError):
    """Raised when a policy document is malformed."""


@dataclass(frozen=True)
class Context:
    """The subject on whose behalf a statement is executed."""

    role: str | None = None
    attributes: Mapping[str, Any] = field(default_factory=dict)

    def attribute(self, name: str) -> Any:
        """Return the value of a subject attribute.

        ``role`` and every key in ``attributes`` are addressable as
        ``subject.<name>`` from a policy condition.
        """
        if name == "role":
            return self.role
        if name in self.attributes:
            return self.attributes[name]
        raise KeyError("subject attribute %r is not set" % name)


@dataclass(frozen=True)
class Rule:
    """One rule attached to a table and command."""

    name: str
    roles: tuple
    when: str | None

    def matches(self, ctx: Context) -> bool:
        """Return whether this rule applies to ``ctx``."""
        if ctx.role is None:
            return False
        return any(ctx.role.startswith(required) for required in self.roles)


@dataclass(frozen=True)
class TablePolicy:
    table: str
    rules: Mapping[str, tuple]


@dataclass(frozen=True)
class DefaultEntry:
    """What to do for a command when the table itself has no rules."""

    decision: str | None = None
    rules: tuple = ()

    @property
    def is_decision(self) -> bool:
        return self.decision is not None


@dataclass(frozen=True)
class PolicySet:
    version: int
    tables: Mapping[str, TablePolicy]
    default: Mapping[str, DefaultEntry]
    revision: str


@dataclass(frozen=True)
class ResolvedPolicy:
    """The rules that apply to one table, command and subject."""

    table: str
    command: str
    rules: tuple = ()
    decision: str = "deny"
    defaulted: bool = False


def _where(*parts) -> str:
    return ".".join(str(part) for part in parts)


def _mapping(value, where):
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise PolicyError("%s must be a mapping" % where)
    return value


def _string(value, where):
    if not isinstance(value, str) or not value:
        raise PolicyError("%s must be a non-empty string" % where)
    return value


def _parse_rule(item, where) -> Rule:
    data = _mapping(item, where)
    unknown = set(data) - {"name", "roles", "when"}
    if unknown:
        raise PolicyError(
            "%s has unknown field(s): %s" % (where, ", ".join(sorted(unknown)))
        )
    if "roles" not in data:
        raise PolicyError("%s is missing required field 'roles'" % where)
    roles = data["roles"]
    if not isinstance(roles, list) or not roles:
        raise PolicyError("%s.roles must be a non-empty list" % where)
    parsed_roles = []
    for index, role in enumerate(roles):
        parsed_roles.append(_string(role, "%s.roles[%d]" % (where, index)))
    name = data.get("name", "")
    if name is None:
        name = ""
    if not isinstance(name, str):
        raise PolicyError("%s.name must be a string" % where)
    when = data.get("when")
    if when is not None:
        if isinstance(when, bool):
            when = "true" if when else "false"
        when = _string(when, "%s.when" % where)
    return Rule(name=name, roles=tuple(parsed_roles), when=when)


def _parse_rule_list(value, where) -> tuple:
    if not isinstance(value, list):
        raise PolicyError("%s must be a list of rules" % where)
    return tuple(
        _parse_rule(item, "%s[%d]" % (where, index))
        for index, item in enumerate(value)
    )


def _parse_default(value, where) -> dict:
    data = _mapping(value, where)
    parsed = {}
    for command, entry in data.items():
        location = _where(where, command)
        if command not in COMMANDS:
            raise PolicyError(
                "%s: unknown command %r (expected one of %s)"
                % (where, command, ", ".join(COMMANDS))
            )
        if isinstance(entry, str):
            decision = entry.strip().lower()
            if decision not in _DECISION_WORDS:
                raise PolicyError(
                    "%s must be 'allow' or 'deny'" % location
                )
            parsed[command] = DefaultEntry(decision=decision)
        elif isinstance(entry, list):
            parsed[command] = DefaultEntry(
                rules=_parse_rule_list(entry, location)
            )
        else:
            raise PolicyError(
                "%s must be 'allow', 'deny' or a list of rules" % location
            )
    return parsed


def _parse_tables(value, where) -> dict:
    data = _mapping(value, where)
    parsed = {}
    for table, body in data.items():
        location = _where(where, table)
        if not isinstance(table, str) or not _TABLE_NAME.match(table):
            raise PolicyError("%s: invalid table name %r" % (where, table))
        if table in parsed:
            raise PolicyError("%s: duplicate table %r" % (where, table))
        body = _mapping(body, location)
        rules = {}
        for command, entry in body.items():
            if command not in COMMANDS:
                raise PolicyError(
                    "%s: unknown command %r (expected one of %s)"
                    % (location, command, ", ".join(COMMANDS))
                )
            rules[command] = _parse_rule_list(
                entry, _where(location, command)
            )
        parsed[table] = TablePolicy(table=table, rules=rules)
    return parsed


def load_policy_set(path: str) -> PolicySet:
    """Load and validate a policy document from ``path``."""
    stat = os.stat(path)
    with open(path, "r", encoding="utf-8") as handle:
        text = handle.read()
    revision = "%s:%s:%s:%s" % (
        os.path.basename(path),
        stat.st_mtime_ns,
        stat.st_size,
        stat.st_ino,
    )
    return loads_policy_set(text, revision=revision)


def loads_policy_set(text: str, revision: str | None = None) -> PolicySet:
    """Load and validate a policy document from ``text``."""
    try:
        data = yamlmini.loads(text)
    except yamlmini.YamlError as exc:
        raise PolicyError(str(exc))

    data = _mapping(data, "document")
    if "version" not in data:
        raise PolicyError("document is missing required field 'version'")
    version = data["version"]
    if not isinstance(version, int) or isinstance(version, bool) or version < 1:
        raise PolicyError("version must be a positive integer")

    unknown = set(data) - {"version", "tables", "default"}
    if unknown:
        raise PolicyError(
            "document has unknown field(s): %s" % ", ".join(sorted(unknown))
        )

    tables = _parse_tables(data.get("tables"), "tables")
    default = _parse_default(data.get("default"), "default")

    if revision is None:
        revision = "version:%d" % version

    return PolicySet(
        version=version, tables=tables, default=default, revision=revision
    )


def _matching_rules(rules, ctx):
    return tuple(rule for rule in rules if rule.matches(ctx))


def resolve_policy(
    policy_set: PolicySet, table: str, command: str, ctx: Context
) -> ResolvedPolicy:
    """Return the policy that governs ``table`` and ``command`` for ``ctx``."""
    if command not in COMMANDS:
        raise PolicyError(
            "unknown command %r (expected one of %s)"
            % (command, ", ".join(COMMANDS))
        )

    table_policy = policy_set.tables.get(table)
    if table_policy is not None:
        rules = table_policy.rules.get(command, ())
        return ResolvedPolicy(
            table=table,
            command=command,
            rules=_matching_rules(rules, ctx),
            decision="deny",
            defaulted=False,
        )

    entry = policy_set.default.get(command)
    if entry is None:
        return ResolvedPolicy(
            table=table,
            command=command,
            rules=(),
            decision="allow",
            defaulted=True,
        )
    if entry.is_decision:
        return ResolvedPolicy(
            table=table,
            command=command,
            rules=(),
            decision=entry.decision,
            defaulted=True,
        )
    return ResolvedPolicy(
        table=table,
        command=command,
        rules=_matching_rules(entry.rules, ctx),
        decision="deny",
        defaulted=True,
    )
