"""租约策略：按 scope 配置单次签发允许的最长有效期。

策略文件是 JSON，形如::

    {
      "max_ttl_seconds": 3600,
      "default": {"max_ttl_seconds": 600},
      "rules": [
        {"scope": "read", "max_ttl_seconds": 3600},
        {"scope": "read:admin", "max_ttl_seconds": 300},
        {"scope": "billing:*", "max_ttl_seconds": 120}
      ]
    }

规则里的 ``scope`` 是 scope 模式，可以以 ``:*`` 结尾表示整棵子树。
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

from .errors import InvalidTTL, UnknownScope
from .scopes import parse_scope


def _positive_int(value: Any, field: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise InvalidTTL(f"{field} 必须是整数秒: {value!r}")
    if value <= 0:
        raise InvalidTTL(f"{field} 必须为正数: {value!r}")
    return value


@dataclass(frozen=True)
class PolicyRule:
    """一条 scope 规则。"""

    scope: str
    max_ttl_seconds: int


@dataclass(frozen=True)
class Policy:
    """一组规则加上全局 TTL 硬上限。"""

    rules: tuple[PolicyRule, ...]
    max_ttl_seconds: int
    default: PolicyRule | None = None

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "Policy":
        if not isinstance(data, Mapping):
            raise TypeError("policy 必须是对象")
        raw_rules = data.get("rules", [])
        if not isinstance(raw_rules, list):
            raise TypeError("rules 必须是数组")
        rules: list[PolicyRule] = []
        seen: set[str] = set()
        for raw in raw_rules:
            if not isinstance(raw, Mapping):
                raise TypeError("每条 rule 必须是对象")
            scope = parse_scope(raw.get("scope", ""))
            if scope in seen:
                raise ValueError(f"重复的 scope 规则: {scope!r}")
            seen.add(scope)
            rules.append(
                PolicyRule(
                    scope=scope,
                    max_ttl_seconds=_positive_int(
                        raw.get("max_ttl_seconds"), "max_ttl_seconds"
                    ),
                )
            )
        default_raw = data.get("default")
        default: PolicyRule | None = None
        if default_raw is not None:
            if not isinstance(default_raw, Mapping):
                raise TypeError("default 必须是对象")
            default = PolicyRule(
                scope="*",
                max_ttl_seconds=_positive_int(
                    default_raw.get("max_ttl_seconds"), "default.max_ttl_seconds"
                ),
            )
        return cls(
            rules=tuple(rules),
            max_ttl_seconds=_positive_int(
                data.get("max_ttl_seconds"), "max_ttl_seconds"
            ),
            default=default,
        )

    @classmethod
    def load(cls, path: str | Path) -> "Policy":
        with open(path, "r", encoding="utf-8") as handle:
            return cls.from_dict(json.load(handle))

    def lookup(self, scope: str) -> PolicyRule:
        """返回适用于该 scope 的规则。"""

        text = parse_scope(scope)
        for rule in self.rules:
            if text.startswith(rule.scope):
                return rule
        if self.default is not None:
            return self.default
        raise UnknownScope(f"policy 中没有适用于 {scope!r} 的规则")

    def max_ttl_for(self, scope: str) -> int:
        """该 scope 单次签发允许的最长 TTL。"""

        return self.lookup(scope).max_ttl_seconds
