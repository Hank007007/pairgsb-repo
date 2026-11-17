"""Render a parsed template with a set of context variables."""

from __future__ import annotations

import time

from .errors import TemplateError
from .escape import escape
from .filters import apply_filter
from .limits import DEFAULT_LIMITS, Limits
from .loader import TemplateLoader
from .parser import Template
from .sandbox import SafeContext, resolve_attr, wrap

_GLOBALS = {
    "range": range,
    "len": len,
}


class Renderer:
    """Walks the node tree and produces the final document."""

    def __init__(self, loader: TemplateLoader | None = None, limits: Limits | None = None) -> None:
        self.loader = loader
        self.limits = limits or DEFAULT_LIMITS
        self.deadline = time.monotonic() + self.limits.max_render_seconds

    # -- entry points ----------------------------------------------------
    def render(self, template: Template, context: dict | SafeContext) -> str:
        scope = context.all() if isinstance(context, SafeContext) else dict(context)
        return self._render_template(template, scope, {})

    def _render_template(
        self,
        template: Template,
        scope: dict,
        overrides: dict[str, list],
    ) -> str:
        if template.extends is not None:
            parent_name = self.eval_expr(template.extends, scope)
            if self.loader is None:
                raise TemplateError("extends requires a loader")
            parent = self.loader.get_template(str(parent_name))
            merged = dict(overrides)
            merged.update(template.blocks)
            return self._render_template(parent, scope, merged)

        pieces: list[str] = []
        self.render_nodes(template.nodes, scope, pieces, overrides)
        return "".join(pieces)

    def render_nodes(
        self,
        nodes: list,
        scope: dict,
        out: list[str],
        overrides: dict[str, list],
    ) -> None:
        for node in nodes:
            kind = node[0]
            if kind == "text":
                out.append(node[1])
            elif kind == "output":
                value = self.eval_expr(node[1], scope)
                out.append(escape(value))
            elif kind == "for":
                self.render_for(node, scope, out, overrides)
            elif kind == "if":
                self.render_if(node, scope, out, overrides)
            elif kind == "include":
                self.render_include(node, scope, out, overrides)
            elif kind == "block":
                name, body = node[1], node[2]
                body = overrides.get(name, body)
                self.render_nodes(body, scope, out, overrides)
            elif kind == "extends":
                continue
            else:
                raise TemplateError(f"cannot render node {kind!r}")

    def render_for(self, node: tuple, scope: dict, out: list[str], overrides: dict) -> None:
        _, targets, iterable_expr, body, else_body = node
        iterable = self.eval_expr(iterable_expr, scope)
        try:
            iterator = iter(iterable)
        except TypeError as exc:
            raise TemplateError(f"value is not iterable: {exc}") from exc

        rendered_any = False
        for item in iterator:
            rendered_any = True
            inner = dict(scope)
            if len(targets) == 1:
                inner[targets[0]] = item
            else:
                values = list(item)
                for index, target in enumerate(targets):
                    inner[target] = values[index] if index < len(values) else None
            self.render_nodes(body, inner, out, overrides)
        if not rendered_any:
            self.render_nodes(else_body, scope, out, overrides)

    def render_if(self, node: tuple, scope: dict, out: list[str], overrides: dict) -> None:
        _, branches, else_body = node
        for condition, body in branches:
            if self.eval_expr(condition, scope):
                self.render_nodes(body, scope, out, overrides)
                return
        self.render_nodes(else_body, scope, out, overrides)

    def render_include(self, node: tuple, scope: dict, out: list[str], overrides: dict) -> None:
        _, name_expr, ignore_missing = node
        name = self.eval_expr(name_expr, scope)
        if self.loader is None:
            raise TemplateError("include requires a loader")
        try:
            included = self.loader.get_template(str(name))
        except TemplateError:
            if ignore_missing:
                return
            raise
        out.append(self._render_template(included, scope, {}))

    # -- expression evaluation ------------------------------------------
    def eval_expr(self, node: tuple, scope: dict) -> object:
        kind = node[0]
        if kind == "lit":
            return node[1]
        if kind == "name":
            return self.lookup(node[1], scope)
        if kind == "attr":
            return resolve_attr(self.eval_expr(node[1], scope), node[2])
        if kind == "index":
            target = self.eval_expr(node[1], scope)
            index = self.eval_expr(node[2], scope)
            try:
                return target[index]  # type: ignore[index]
            except (KeyError, IndexError, TypeError) as exc:
                raise TemplateError(f"cannot index value: {exc}") from exc
        if kind == "list":
            return [self.eval_expr(item, scope) for item in node[1]]
        if kind == "call":
            func = self.eval_expr(node[1], scope)
            args = [self.eval_expr(arg, scope) for arg in node[2]]
            return self.call(func, args)
        if kind == "filter":
            value = self.eval_expr(node[1], scope)
            args = [self.eval_expr(arg, scope) for arg in node[3]]
            return apply_filter(node[2], value, args)
        if kind == "unop":
            value = self.eval_expr(node[2], scope)
            if node[1] == "not":
                return not value
            if node[1] == "-":
                return -value  # type: ignore[operator]
            raise TemplateError(f"unknown unary operator {node[1]!r}")
        if kind == "binop":
            return self.eval_binop(node[1], node[2], node[3], scope)
        raise TemplateError(f"cannot evaluate expression {kind!r}")

    def eval_binop(self, op: str, left_node: tuple, right_node: tuple, scope: dict) -> object:
        if op == "and":
            left = self.eval_expr(left_node, scope)
            return self.eval_expr(right_node, scope) if left else left
        if op == "or":
            left = self.eval_expr(left_node, scope)
            return left if left else self.eval_expr(right_node, scope)

        left = self.eval_expr(left_node, scope)
        right = self.eval_expr(right_node, scope)
        if op == "==":
            return left == right
        if op == "!=":
            return left != right
        if op == "<":
            return left < right  # type: ignore[operator]
        if op == ">":
            return left > right  # type: ignore[operator]
        if op == "<=":
            return left <= right  # type: ignore[operator]
        if op == ">=":
            return left >= right  # type: ignore[operator]
        if op == "in":
            return left in right  # type: ignore[operator]
        if op == "not in":
            return left not in right  # type: ignore[operator]
        if op == "+":
            return left + right  # type: ignore[operator]
        if op == "-":
            return left - right  # type: ignore[operator]
        if op == "*":
            return left * right  # type: ignore[operator]
        if op == "/":
            return left / right  # type: ignore[operator]
        if op == "//":
            return left // right  # type: ignore[operator]
        if op == "%":
            return left % right  # type: ignore[operator]
        raise TemplateError(f"unknown operator {op!r}")

    def lookup(self, name: str, scope: dict) -> object:
        if name in scope:
            return wrap(scope[name])
        if name in _GLOBALS:
            return _GLOBALS[name]
        raise TemplateError(f"undefined variable: {name}")

    def call(self, func: object, args: list) -> object:
        if not callable(func):
            raise TemplateError(f"value of type {type(func).__name__} is not callable")
        try:
            return func(*args)
        except Exception as exc:
            raise TemplateError(f"call failed: {exc}") from exc


def render(
    template: Template,
    context: dict | SafeContext,
    loader: TemplateLoader | None = None,
    limits: Limits | None = None,
) -> str:
    """Render ``template`` with ``context``.

    ``limits`` carries the resource bounds for this render.
    """

    limits = limits or DEFAULT_LIMITS
    max_loop_iterations = limits.max_loop_iterations
    max_output_bytes = limits.max_output_bytes
    max_render_seconds = limits.max_render_seconds
    deadline = time.monotonic() + max_render_seconds

    if loader is None:
        loader = getattr(template, "loader", None)
    renderer = Renderer(loader=loader, limits=limits)
    renderer.deadline = deadline
    return renderer.render(template, context)
