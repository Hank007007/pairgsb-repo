"""Template source parsing."""

from __future__ import annotations

import re

from .errors import TemplateSyntaxError
from .expr import parse_expression

_TAG_RE = re.compile(r"(\{\{.*?\}\}|\{%.*?%\}|\{#.*?#\})", re.DOTALL)

_TEXT = "text"
_OUTPUT = "output"
_TAG = "tag"


class Template:
    """A parsed template ready to be rendered."""

    def __init__(self, source: str, name: str = "<string>") -> None:
        self.source = source
        self.name = name
        self.nodes = TemplateParser(source, name).parse()
        self.extends: tuple | None = None
        self.blocks: dict[str, list] = {}
        self.loader = None
        _collect(self.nodes, self.blocks, self)

    def __repr__(self) -> str:
        return f"Template({self.name!r}, nodes={len(self.nodes)})"


def _split(source: str, name: str) -> list[tuple[str, str]]:
    """Split source into (kind, payload) items."""

    items: list[tuple[str, str]] = []
    for piece in _TAG_RE.split(source):
        if not piece:
            continue
        if piece.startswith("{#"):
            continue
        if piece.startswith("{{"):
            body = piece[2:-2].strip()
            if not body:
                raise TemplateSyntaxError(f"{name}: empty output tag")
            items.append((_OUTPUT, body))
        elif piece.startswith("{%"):
            body = piece[2:-2].strip()
            if not body:
                raise TemplateSyntaxError(f"{name}: empty tag")
            items.append((_TAG, body))
        else:
            items.append((_TEXT, piece))
    return items


class TemplateParser:
    """Builds a node list from template source."""

    def __init__(self, source: str, name: str = "<string>") -> None:
        self.source = source
        self.name = name
        self.items = _split(source, name)
        self.index = 0

    def parse(self) -> list:
        nodes = self._parse_nodes(())
        if self.index < len(self.items):
            kw = self.items[self.index][1].partition(" ")[0]
            raise self._error(f"unexpected {kw!r}")
        return nodes

    def _error(self, message: str) -> TemplateSyntaxError:
        return TemplateSyntaxError(f"{self.name}: {message}")

    def _parse_nodes(self, terminators: tuple[str, ...]) -> list:
        """Parse until one of ``terminators`` is reached.

        Returns the node list and leaves ``self.index`` pointing at the
        terminating tag, which the caller consumes.
        """

        nodes: list = []
        while self.index < len(self.items):
            kind, payload = self.items[self.index]
            if kind == _TEXT:
                nodes.append(("text", payload))
                self.index += 1
                continue
            if kind == _OUTPUT:
                nodes.append(("output", parse_expression(payload)))
                self.index += 1
                continue
            keyword, _, rest = payload.partition(" ")
            keyword = keyword.strip()
            rest = rest.strip()
            if keyword in terminators:
                return nodes
            self.index += 1
            if keyword == "for":
                nodes.append(self._parse_for(rest))
            elif keyword == "if":
                nodes.append(self._parse_if(rest))
            elif keyword == "include":
                nodes.append(self._parse_include(rest))
            elif keyword == "extends":
                nodes.append(("extends", parse_expression(rest)))
            elif keyword == "block":
                body = self._parse_nodes(("endblock",))
                if self.index >= len(self.items):
                    raise self._error("unclosed block")
                self.index += 1
                nodes.append(("block", rest, body))
            else:
                raise self._error(f"unknown tag {keyword!r}")
        return nodes

    def _parse_for(self, rest: str) -> tuple:
        target, sep, iterable = rest.partition(" in ")
        if not sep:
            raise self._error("for tag requires 'in'")
        targets = tuple(part.strip() for part in target.split(",") if part.strip())
        if not targets:
            raise self._error("for tag requires a loop target")
        body = self._parse_nodes(("else", "endfor"))
        else_body: list = []
        if self.index < len(self.items) and _keyword(self.items[self.index]) == "else":
            self.index += 1
            else_body = self._parse_nodes(("endfor",))
        if self.index >= len(self.items):
            raise self._error("unclosed for tag")
        self.index += 1
        return ("for", targets, parse_expression(iterable), body, else_body)

    def _parse_if(self, rest: str) -> tuple:
        branches = []
        condition = parse_expression(rest)
        body = self._parse_nodes(("elif", "else", "endif"))
        branches.append((condition, body))
        else_body: list = []
        while self.index < len(self.items):
            keyword = _keyword(self.items[self.index])
            if keyword == "elif":
                rest = self.items[self.index][1][len("elif"):].strip()
                self.index += 1
                condition = parse_expression(rest)
                body = self._parse_nodes(("elif", "else", "endif"))
                branches.append((condition, body))
            elif keyword == "else":
                self.index += 1
                else_body = self._parse_nodes(("endif",))
            else:
                break
        if self.index >= len(self.items):
            raise self._error("unclosed if tag")
        self.index += 1
        return ("if", branches, else_body)

    def _parse_include(self, rest: str) -> tuple:
        if not rest:
            raise self._error("include tag requires a template name")
        marker = rest.rpartition(" ")
        if marker[1] == " " and marker[2].strip() == "missing" and marker[0].strip().endswith("ignore"):
            name_expr = marker[0].strip()[: -len("ignore")].strip()
            return ("include", parse_expression(name_expr), True)
        return ("include", parse_expression(rest), False)


def _keyword(item: tuple[str, str]) -> str:
    kind, payload = item
    if kind != _TAG:
        return ""
    return payload.partition(" ")[0].strip()


def _collect(nodes: list, blocks: dict[str, list], template: Template) -> None:
    for node in nodes:
        kind = node[0]
        if kind == "block":
            blocks[node[1]] = node[2]
            _collect(node[2], blocks, template)
        elif kind == "for":
            _collect(node[3], blocks, template)
            _collect(node[4], blocks, template)
        elif kind == "if":
            for _, branch in node[1]:
                _collect(branch, blocks, template)
            _collect(node[2], blocks, template)
        elif kind == "extends":
            template.extends = node[1]
