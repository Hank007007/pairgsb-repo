"""A tiny reader for the block-style YAML subset used by policy files.

Only the constructs that appear under ``grantable.yaml`` and ``policy/`` are
supported: nested mappings, block sequences of scalars, comments, integers,
booleans and plain strings.  This keeps the runtime free of third-party
dependencies; it is not a general-purpose YAML implementation.
"""

from __future__ import annotations

from typing import Any


def loads(text: str) -> Any:
    """Parse ``text`` and return the document as Python containers."""

    lines = _significant_lines(text)
    if not lines:
        return {}
    value, index = _parse_block(lines, 0, lines[0][0])
    if index != len(lines):
        raise ValueError(f"unexpected content on line {lines[index][2]}")
    return value


def _significant_lines(text: str) -> list[tuple[int, str, int]]:
    result: list[tuple[int, str, int]] = []
    for number, raw in enumerate(text.splitlines(), start=1):
        line = _strip_comment(raw).rstrip()
        if not line.strip():
            continue
        prefix = line[: len(line) - len(line.lstrip(" \t"))]
        if "\t" in prefix:
            raise ValueError(f"tab indentation on line {number}")
        indent = len(prefix)
        result.append((indent, line.strip(), number))
    return result


def _strip_comment(line: str) -> str:
    quote: str | None = None
    for position, character in enumerate(line):
        if quote is not None:
            if character == quote:
                quote = None
        elif character in "\"'":
            quote = character
        elif character == "#" and (position == 0 or line[position - 1] in " \t"):
            return line[:position]
    return line


def _parse_block(
    lines: list[tuple[int, str, int]], start: int, indent: int
) -> tuple[Any, int]:
    content = lines[start][1]
    if content.startswith("- "):
        return _parse_sequence(lines, start, indent)
    return _parse_mapping(lines, start, indent)


def _parse_mapping(
    lines: list[tuple[int, str, int]], start: int, indent: int
) -> tuple[dict[str, Any], int]:
    mapping: dict[str, Any] = {}
    index = start
    while index < len(lines):
        line_indent, content, number = lines[index]
        if line_indent < indent:
            break
        if line_indent > indent:
            raise ValueError(f"unexpected indentation on line {number}")
        if content.startswith("- "):
            break

        key, separator, remainder = content.partition(":")
        key = key.strip()
        if not separator or not key:
            raise ValueError(f"expected 'key: value' on line {number}")
        remainder = remainder.strip()
        index += 1

        if remainder:
            mapping[key] = _scalar(remainder)
        elif index < len(lines) and lines[index][0] > indent:
            mapping[key], index = _parse_block(lines, index, lines[index][0])
        elif (
            index < len(lines)
            and lines[index][0] == indent
            and lines[index][1].startswith("- ")
        ):
            mapping[key], index = _parse_sequence(lines, index, indent)
        else:
            mapping[key] = None
    return mapping, index


def _parse_sequence(
    lines: list[tuple[int, str, int]], start: int, indent: int
) -> tuple[list[Any], int]:
    items: list[Any] = []
    index = start
    while index < len(lines):
        line_indent, content, number = lines[index]
        if line_indent < indent:
            break
        if line_indent > indent:
            raise ValueError(f"unexpected indentation on line {number}")
        if not content.startswith("- "):
            break

        item = content[2:].strip()
        index += 1
        if not item:
            if index < len(lines) and lines[index][0] > indent:
                value, index = _parse_block(lines, index, lines[index][0])
                items.append(value)
            else:
                items.append(None)
        elif _looks_like_mapping(item):
            key, _, remainder = item.partition(":")
            entry: dict[str, Any] = {}
            remainder = remainder.strip()
            entry[key.strip()] = _scalar(remainder) if remainder else None
            if index < len(lines) and lines[index][0] > indent:
                extra, index = _parse_block(lines, index, lines[index][0])
                if isinstance(extra, dict):
                    entry.update(extra)
            items.append(entry)
        else:
            items.append(_scalar(item))
    return items, index


def _looks_like_mapping(text: str) -> bool:
    return ": " in text or text.endswith(":")


def _scalar(text: str) -> Any:
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "\"'":
        return text[1:-1]

    lowered = text.lower()
    if lowered in {"true", "false"}:
        return lowered == "true"
    if lowered in {"null", "~"}:
        return None

    try:
        return int(text)
    except ValueError:
        pass
    try:
        return float(text)
    except ValueError:
        pass
    return text
