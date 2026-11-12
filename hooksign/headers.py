"""解析 ``X-Signature`` 头。"""

from __future__ import annotations

from dataclasses import dataclass

HEADER_NAME = "X-Signature"


@dataclass(frozen=True)
class ParsedSignature:
    """头里解析出来的时间戳与签名列表。"""

    timestamp: int | None
    signatures: tuple[str, ...]


def parse_signature_header(value: str) -> ParsedSignature:
    """把 ``t=...,v1=...,v1=...`` 解析成结构化数据。

    条目之间用 ``,`` 分隔，条目内部的空白会被去掉；不认识的条目直接忽略。
    时间戳不是合法整数时抛 :class:`ValueError`。
    """
    timestamp: int | None = None
    signatures: list[str] = []

    for item in value.split(","):
        item = item.strip()
        if not item:
            continue
        name, sep, raw = item.partition("=")
        if not sep:
            continue
        name = name.strip()
        raw = raw.strip()
        if name == "t":
            timestamp = int(raw)
        elif name == "v1" and raw:
            signatures.append(raw)

    return ParsedSignature(timestamp=timestamp, signatures=tuple(signatures))
