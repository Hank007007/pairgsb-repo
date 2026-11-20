"""A small reader for the subset of YAML used by policy files.

The policy format needs block mappings, block sequences, inline sequences,
quoted and plain scalars, and comments.  Anything outside that subset is
reported as an error instead of being guessed at.
"""

from __future__ import annotations

import json
import re


class YamlError(ValueError):
    """Raised when a document is not valid for the supported subset."""


_INT_RE = re.compile(r"-?\d+")
_FLOAT_RE = re.compile(r"-?(?:\d+\.\d*|\.\d+)(?:[eE][+-]?\d+)?")
_EXP_RE = re.compile(r"-?\d+[eE][+-]?\d+")


def _strip_comment(line: str) -> str:
    quote = None
    out = []
    for index, char in enumerate(line):
        if quote is not None:
            out.append(char)
            if char == quote:
                quote = None
            continue
        if char in ("'", '"'):
            quote = char
            out.append(char)
            continue
        if char == "#" and (index == 0 or line[index - 1].isspace()):
            break
        out.append(char)
    return "".join(out).rstrip()


def _split_key(text: str, lineno: int):
    quote = None
    depth = 0
    for index, char in enumerate(text):
        if quote is not None:
            if char == quote:
                quote = None
            continue
        if char in ("'", '"'):
            quote = char
            continue
        if char in "[{(":
            depth += 1
            continue
        if char in "]})":
            depth -= 1
            continue
        if char == ":" and depth == 0:
            key = text[:index].strip()
            rest = text[index + 1:].strip()
            if not key:
                raise YamlError("line %d: empty mapping key" % lineno)
            return key, rest
    raise YamlError("line %d: expected 'key: value'" % lineno)


def _looks_like_key(text: str) -> bool:
    try:
        _split_key(text, 0)
    except YamlError:
        return False
    return True


def _split_inline(text: str, lineno: int):
    inner = text[1:-1].strip()
    if not inner:
        return []
    parts = []
    buffer = []
    quote = None
    depth = 0
    for char in inner:
        if quote is not None:
            buffer.append(char)
            if char == quote:
                quote = None
            continue
        if char in ("'", '"'):
            quote = char
            buffer.append(char)
            continue
        if char in "[{(":
            depth += 1
        elif char in "]})":
            depth -= 1
        if char == "," and depth == 0:
            parts.append("".join(buffer).strip())
            buffer = []
            continue
        buffer.append(char)
    if quote is not None or depth != 0:
        raise YamlError("line %d: unterminated inline value" % lineno)
    parts.append("".join(buffer).strip())
    return [_scalar(part, lineno) for part in parts]


def _scalar(text: str, lineno: int):
    text = text.strip()
    if not text:
        return None
    if text[0] == "'":
        if len(text) < 2 or text[-1] != "'":
            raise YamlError("line %d: unterminated quoted string" % lineno)
        return text[1:-1].replace("''", "'")
    if text[0] == '"':
        if len(text) < 2 or text[-1] != '"':
            raise YamlError("line %d: unterminated quoted string" % lineno)
        try:
            return json.loads(text)
        except json.JSONDecodeError as exc:
            raise YamlError("line %d: invalid quoted string: %s" % (lineno, exc))
    if text[0] == "[":
        if text[-1] != "]":
            raise YamlError("line %d: unterminated inline sequence" % lineno)
        return _split_inline(text, lineno)
    if text[0] in "{":
        raise YamlError("line %d: inline mappings are not supported" % lineno)
    lowered = text.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    if lowered in ("null", "~"):
        return None
    if _INT_RE.fullmatch(text):
        return int(text)
    if _FLOAT_RE.fullmatch(text) or _EXP_RE.fullmatch(text):
        return float(text)
    return text


class _Parser:
    def __init__(self, text: str):
        self.lines = []
        for lineno, raw in enumerate(text.splitlines(), 1):
            prefix = raw[: len(raw) - len(raw.lstrip())]
            if "\t" in prefix:
                raise YamlError("line %d: tabs are not allowed for indentation" % lineno)
            stripped = _strip_comment(raw)
            if not stripped.strip():
                continue
            indent = len(stripped) - len(stripped.lstrip(" "))
            self.lines.append((indent, stripped.strip(), lineno))
        self.index = 0

    def parse(self):
        if not self.lines:
            return {}
        value = self._block(self.lines[0][0])
        if self.index != len(self.lines):
            _, _, lineno = self.lines[self.index]
            raise YamlError("line %d: unexpected content" % lineno)
        return value

    def _block(self, indent: int):
        if self.lines[self.index][0] != indent:
            lineno = self.lines[self.index][2]
            raise YamlError("line %d: unexpected indentation" % lineno)
        if self._is_sequence(self.lines[self.index][1]):
            return self._sequence(indent)
        return self._mapping(indent)

    @staticmethod
    def _is_sequence(content: str) -> bool:
        return content == "-" or content.startswith("- ")

    def _mapping(self, indent: int):
        result = {}
        while self.index < len(self.lines):
            line_indent, content, lineno = self.lines[self.index]
            if line_indent < indent:
                break
            if line_indent > indent:
                raise YamlError("line %d: unexpected indentation" % lineno)
            if self._is_sequence(content):
                break
            key, rest = _split_key(content, lineno)
            self.index += 1
            if rest:
                value = _scalar(rest, lineno)
            elif self.index < len(self.lines) and self.lines[self.index][0] > indent:
                value = self._block(self.lines[self.index][0])
            elif (
                self.index < len(self.lines)
                and self.lines[self.index][0] == indent
                and self._is_sequence(self.lines[self.index][1])
            ):
                value = self._sequence(indent)
            else:
                value = None
            if key in result:
                raise YamlError("line %d: duplicate key %r" % (lineno, key))
            result[key] = value
        return result

    def _sequence(self, indent: int):
        items = []
        while self.index < len(self.lines):
            line_indent, content, lineno = self.lines[self.index]
            if line_indent < indent:
                break
            if line_indent > indent:
                raise YamlError("line %d: unexpected indentation" % lineno)
            if not self._is_sequence(content):
                break
            rest = content[1:].lstrip()
            if not rest:
                self.index += 1
                if self.index < len(self.lines) and self.lines[self.index][0] > indent:
                    item = self._block(self.lines[self.index][0])
                else:
                    item = None
            elif _looks_like_key(rest):
                item_indent = indent + (len(content) - len(rest))
                item = self._mapping_item(rest, item_indent)
            else:
                self.index += 1
                item = _scalar(rest, lineno)
            items.append(item)
        return items

    def _mapping_item(self, first: str, item_indent: int):
        result = {}
        pending = first
        pending_lineno = self.lines[self.index][2]
        self.index += 1
        while True:
            key, rest = _split_key(pending, pending_lineno)
            if rest:
                value = _scalar(rest, pending_lineno)
            elif self.index < len(self.lines) and self.lines[self.index][0] > item_indent:
                value = self._block(self.lines[self.index][0])
            elif (
                self.index < len(self.lines)
                and self.lines[self.index][0] == item_indent
                and self._is_sequence(self.lines[self.index][1])
            ):
                value = self._sequence(item_indent)
            else:
                value = None
            if key in result:
                raise YamlError(
                    "line %d: duplicate key %r" % (pending_lineno, key)
                )
            result[key] = value
            if self.index >= len(self.lines):
                break
            next_indent, next_content, next_lineno = self.lines[self.index]
            if next_indent == item_indent and not self._is_sequence(next_content):
                pending = next_content
                pending_lineno = next_lineno
                self.index += 1
                continue
            if next_indent > item_indent:
                raise YamlError(
                    "line %d: unexpected indentation" % next_lineno
                )
            break
        return result


def loads(text: str):
    """Parse a policy document from a string."""
    return _Parser(text).parse()


def load(path: str):
    """Parse a policy document from a UTF-8 file."""
    with open(path, "r", encoding="utf-8") as handle:
        return loads(handle.read())
