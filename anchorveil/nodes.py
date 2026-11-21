"""Parse-tree node types shared by the parser and the later stages."""

from __future__ import annotations

from dataclasses import dataclass

class _Unset:
    """Sentinel for a scalar whose value has already been resolved."""

    __slots__ = ()

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return "<unset>"

    def __copy__(self) -> "_Unset":
        return self

    def __deepcopy__(self, memo: dict[int, object]) -> "_Unset":
        return self


UNSET = _Unset()


@dataclass(slots=True)
class ScalarNode:
    text: str
    quoted: bool = False
    line: int = 0
    value: object = UNSET


@dataclass(slots=True)
class AliasNode:
    name: str
    line: int = 0


@dataclass(slots=True)
class TaggedNode:
    tag: str
    content: "Node"
    line: int = 0


Node = ScalarNode | AliasNode | TaggedNode | dict[str, "Node"] | list["Node"]
