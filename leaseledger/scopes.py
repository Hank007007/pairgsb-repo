"""scope 文本的解析与规范化。

scope 是一个以 ``:`` 分隔的层级名字，例如 ``read``、``read:admin``、
``billing:invoice:export``。policy 里还可以使用末尾的 ``*`` 表示整棵子树。
"""

from __future__ import annotations

import re
from typing import Iterable

from .errors import InvalidScope

_SEGMENT_RE = re.compile(r"^[a-z][a-z0-9_-]*$")


def _split(text: str) -> list[str]:
    return text.split(":")


def parse_scope(value: str) -> str:
    """解析单个 scope，返回规范化文本。

    允许末尾出现一个 ``*`` 段（只用于 policy 规则）；其余段必须是以小写字母
    开头的 ``[a-z0-9_-]`` 串。
    """

    if not isinstance(value, str):
        raise InvalidScope(f"scope 必须是字符串: {value!r}")
    text = value.strip()
    if not text:
        raise InvalidScope("scope 不能为空")
    segments = _split(text)
    for index, segment in enumerate(segments):
        if not segment:
            raise InvalidScope(f"scope 含空段: {value!r}")
        if segment == "*":
            if index != len(segments) - 1:
                raise InvalidScope(f"通配符只能出现在末尾: {value!r}")
            continue
        if not _SEGMENT_RE.match(segment):
            raise InvalidScope(f"非法 scope 段 {segment!r}: {value!r}")
    return ":".join(segments)


def is_wildcard(scope: str) -> bool:
    return scope == "*" or scope.endswith(":*")


def is_concrete(scope: str) -> bool:
    return "*" not in scope


def parse_scopes(value: str | Iterable[str] | None) -> tuple[str, ...]:
    """解析 scope 集合。

    字符串按逗号切分；其它可迭代对象逐个读取。结果去重并保持首次出现顺序。
    """

    if value is None:
        raise InvalidScope("scope 集合不能为空")
    if isinstance(value, str):
        items: Iterable[str] = value.split(",")
    else:
        items = value
    result: list[str] = []
    for item in items:
        scope = parse_scope(item)
        if scope not in result:
            result.append(scope)
    if not result:
        raise InvalidScope("scope 集合不能为空")
    return tuple(result)


def ensure_concrete(scopes: Iterable[str]) -> tuple[str, ...]:
    """确认一组 scope 都是具体 scope（不含通配符）。"""

    result = tuple(scopes)
    for scope in result:
        if not is_concrete(scope):
            raise InvalidScope(f"此处不接受通配 scope: {scope!r}")
    return result


def scope_depth(scope: str) -> int:
    """scope 的段数；``*`` 不算作一层。"""

    if scope == "*":
        return 0
    segments = scope.split(":")
    if segments and segments[-1] == "*":
        return len(segments) - 1
    return len(segments)
