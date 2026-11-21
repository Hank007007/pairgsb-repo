"""Line-oriented parser for the manifest subset.

The parser turns manifest text into a tree of :mod:`anchorveil.nodes` plus a
table of anchors.  Aliases are kept symbolic; merge keys stay in the mapping
as a plain ``"<<"`` entry.  Both are resolved by later stages.
"""

from __future__ import annotations

from dataclasses import dataclass

from .errors import ParseError
from .nodes import AliasNode, Node, ScalarNode, TaggedNode


@dataclass(slots=True)
class _Line:
    indent: int
    text: str
    lineno: int


def parse(text: str) -> tuple[Node, dict[str, Node]]:
    """Parse ``text`` and return ``(root, anchors)``."""
    lines = _prepare(text)
    anchors: dict[str, Node] = {}
    if not lines:
        return ScalarNode("", False, 1), anchors
    if lines[0].indent != 0:
        raise ParseError("document must start at column 0", lines[0].lineno)
    first = lines[0]
    if not (first.text == "-" or first.text.startswith("- ")) and _split_key(first.text) is None:
        if len(lines) > 1:
            raise ParseError("unexpected content after root scalar", lines[1].lineno)
        root, _ = _parse_value(
            first.text, lines, 1, 0, first.lineno, anchors, allow_block=False
        )
        return root, anchors
    root, index = _parse_block(lines, 0, 0, anchors)
    if index < len(lines):
        raise ParseError("unexpected indentation", lines[index].lineno)
    return root, anchors


def _prepare(text: str) -> list[_Line]:
    lines: list[_Line] = []
    for number, raw in enumerate(text.splitlines(), start=1):
        stripped = _strip_comment(raw).rstrip()
        if not stripped.strip():
            continue
        indent = 0
        while indent < len(stripped) and stripped[indent] == " ":
            indent += 1
        if indent < len(stripped) and stripped[indent] == "\t":
            raise ParseError("tab characters are not allowed for indentation", number)
        lines.append(_Line(indent, stripped[indent:], number))
    return lines


def _strip_comment(line: str) -> str:
    quote: str | None = None
    i = 0
    while i < len(line):
        ch = line[i]
        if quote is None:
            if ch in ("'", '"'):
                quote = ch
            elif ch == "#" and (i == 0 or line[i - 1] in " \t"):
                return line[:i]
        else:
            if ch == "\\" and quote == '"':
                i += 2
                continue
            if ch == quote:
                quote = None
        i += 1
    return line


def _parse_block(
    lines: list[_Line], index: int, indent: int, anchors: dict[str, Node]
) -> tuple[Node, int]:
    if lines[index].text == "-" or lines[index].text.startswith("- "):
        return _parse_sequence(lines, index, indent, anchors)
    return _parse_mapping(lines, index, indent, anchors)


def _parse_sequence(
    lines: list[_Line], index: int, indent: int, anchors: dict[str, Node]
) -> tuple[Node, int]:
    items: list[Node] = []
    while index < len(lines) and lines[index].indent == indent:
        line = lines[index]
        if not (line.text == "-" or line.text.startswith("- ")):
            raise ParseError("expected a sequence entry", line.lineno)
        rest = line.text[1:].lstrip()
        if rest == "":
            index += 1
            if index < len(lines) and lines[index].indent > indent:
                value, index = _parse_block(lines, index, lines[index].indent, anchors)
            else:
                value = ScalarNode("", False, line.lineno)
            items.append(value)
            continue
        if rest == "-" or rest.startswith("- "):
            lines[index] = _Line(indent + 2, rest, line.lineno)
            value, index = _parse_sequence(lines, index, indent + 2, anchors)
            items.append(value)
            continue
        if _split_key(rest) is not None:
            lines[index] = _Line(indent + 2, rest, line.lineno)
            value, index = _parse_mapping(lines, index, indent + 2, anchors)
            items.append(value)
            continue
        index += 1
        value, index = _parse_value(
            rest, lines, index, indent, line.lineno, anchors, allow_block=True
        )
        items.append(value)
    return items, index


def _parse_mapping(
    lines: list[_Line], index: int, indent: int, anchors: dict[str, Node]
) -> tuple[Node, int]:
    mapping: dict[str, Node] = {}
    while index < len(lines) and lines[index].indent == indent:
        line = lines[index]
        if line.text == "-" or line.text.startswith("- "):
            raise ParseError("cannot mix mapping keys and sequence entries", line.lineno)
        split = _split_key(line.text)
        if split is None:
            raise ParseError("expected 'key: value'", line.lineno)
        key_text, rest = split
        key = _key_text(key_text, line.lineno)
        index += 1
        value, index = _parse_value(
            rest, lines, index, indent, line.lineno, anchors, allow_block=True
        )
        mapping[key] = value
    return mapping, index


def _parse_value(
    rest: str,
    lines: list[_Line],
    index: int,
    parent_indent: int,
    lineno: int,
    anchors: dict[str, Node],
    *,
    allow_block: bool,
) -> tuple[Node, int]:
    rest = rest.strip()
    anchor = None
    tag = None

    while rest:
        if rest[0] == "&":
            name, _, remainder = rest[1:].partition(" ")
            if not name:
                raise ParseError("anchor name is required", lineno)
            if anchor is not None:
                raise ParseError("only one anchor may be attached to a node", lineno)
            anchor = name
            rest = remainder.strip()
            continue
        if rest[0] == "!":
            tag_text, _, remainder = rest.partition(" ")
            if tag is not None:
                raise ParseError("only one tag may be attached to a node", lineno)
            tag = tag_text
            rest = remainder.strip()
            continue
        break

    if rest == "":
        if allow_block and index < len(lines) and lines[index].indent > parent_indent:
            value, index = _parse_block(lines, index, lines[index].indent, anchors)
        else:
            value = ScalarNode("", False, lineno)
    elif rest[0] == "*":
        name = rest[1:].strip()
        if not name or " " in name:
            raise ParseError("invalid alias", lineno)
        value = AliasNode(name, lineno)
        index = _expect_no_block(lines, index, parent_indent, lineno)
    else:
        value = _scalar(rest, lineno)
        index = _expect_no_block(lines, index, parent_indent, lineno)

    if tag is not None:
        value = TaggedNode(tag, value, lineno)
    if anchor is not None:
        anchors[anchor] = value
    return value, index


def _expect_no_block(
    lines: list[_Line], index: int, parent_indent: int, lineno: int
) -> int:
    if index < len(lines) and lines[index].indent > parent_indent:
        raise ParseError("unexpected indented block", lines[index].lineno)
    return index


def _scalar(text: str, lineno: int) -> ScalarNode:
    if text[0] in ("'", '"'):
        quote = text[0]
        if len(text) < 2 or text[-1] != quote:
            raise ParseError("unterminated quoted scalar", lineno)
        return ScalarNode(text, True, lineno)
    return ScalarNode(text, False, lineno)


def _key_text(text: str, lineno: int) -> str:
    text = text.strip()
    if text and text[0] in ("'", '"'):
        if len(text) < 2 or text[-1] != text[0]:
            raise ParseError("unterminated quoted key", lineno)
        body = text[1:-1]
        return body.replace("''", "'") if text[0] == "'" else body
    if not text:
        raise ParseError("mapping key is required", lineno)
    return text


def _split_key(text: str) -> tuple[str, str] | None:
    quote: str | None = None
    i = 0
    while i < len(text):
        ch = text[i]
        if quote is None:
            if ch in ("'", '"'):
                quote = ch
            elif ch == ":":
                after = text[i + 1 : i + 2]
                if after in ("", " "):
                    return text[:i], text[i + 1 :]
        else:
            if ch == "\\" and quote == '"':
                i += 2
                continue
            if ch == quote:
                quote = None
        i += 1
    return None
