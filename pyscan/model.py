"""分析结果的对外数据结构。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Sequence


@dataclass(frozen=True)
class SinkSpec:
    """一个危险调用点。

    name 是点分全名，例如 ``os.system``、``cursor.execute``。
    arg 是参数下标（0 起），``-1`` 表示任意位置参数都算。
    """

    name: str
    arg: int = 0


@dataclass(frozen=True)
class SanitizerSpec:
    """一个净化函数。

    name 是点分全名；arg 是被净化的参数下标，``-1`` 表示任意位置参数。
    """

    name: str
    arg: int = 0


@dataclass(frozen=True)
class Rule:
    """一组规则：哪些值算脏，哪些调用算危险，哪些函数能洗干净。"""

    id: str
    name: str
    sources: Sequence[str] = field(default_factory=tuple)
    sinks: Sequence[SinkSpec] = field(default_factory=tuple)
    sanitizers: Sequence[SanitizerSpec] = field(default_factory=tuple)


@dataclass(frozen=True)
class Finding:
    """一条命中。trace 是从污染源到危险点经过的语句，给人看。"""

    path: str
    line: int
    col: int
    rule: str
    sink: str
    trace: tuple[str, ...] = ()
