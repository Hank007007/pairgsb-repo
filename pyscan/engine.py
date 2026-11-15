"""污点分析引擎：只走 AST，不执行被扫代码。

整体是一条很直的路：按源码顺序过一遍，碰到赋值就把"这个变量脏了，脏从哪来"记下来，
碰到规则里的危险调用就看它实参脏不脏。命中就把来源和危险点拼成 trace。
"""

from __future__ import annotations

import ast
from pathlib import Path
from typing import Iterable, Sequence

from .model import Finding, Rule

__all__ = ["analyze_source", "analyze_paths"]


def _dotted(node: ast.AST) -> str | None:
    """把 ``a.b.c`` 这种纯名字链还原成字符串；不是纯名字链就返回 None。"""
    parts: list[str] = []
    while isinstance(node, ast.Attribute):
        parts.append(node.attr)
        node = node.value
    if isinstance(node, ast.Name):
        parts.append(node.id)
        return ".".join(reversed(parts))
    return None


def _matches(name: str, table: Sequence[str]) -> str | None:
    """name 与条目相等、是它的下级（``os.environ.get`` 对 ``os.environ``），
    或者条目以 ``name`` 结尾（``request.args`` 对 ``flask.request.args``），都算命中。
    """
    for entry in table:
        if name == entry or name.startswith(entry + "."):
            return entry
        if entry.endswith("." + name):
            return entry
    return None


class _Walker(ast.NodeVisitor):
    """按顺序走一遍模块，维护"哪些变量脏了"。

    每个函数单独走一遍，进函数时清空变量表。
    """

    def __init__(self, path: str, rule: Rule) -> None:
        self.path = path
        self.rule = rule
        self.sources = tuple(rule.sources)
        self.sinks = tuple(rule.sinks)
        self.sanitizers = tuple(rule.sanitizers)
        self.tainted: dict[str, tuple[str, int]] = {}
        self.findings: list[Finding] = []

    # ---- 污点来源 -------------------------------------------------

    def _source_hit(self, expr: ast.AST) -> tuple[str, int] | None:
        """表达式本身就在取污点。``request.args["q"]`` 这种下标也认。"""
        node = expr
        while isinstance(node, ast.Subscript):
            node = node.value
        if isinstance(node, ast.Call):
            name = _dotted(node.func)
            if name is not None:
                hit = _matches(name, self.sources)
                if hit is not None:
                    return f"{hit}()", node.lineno
            return None
        name = _dotted(node)
        if name is not None:
            hit = _matches(name, self.sources)
            if hit is not None:
                return hit, node.lineno
        return None

    def _sanitized(self, expr: ast.AST) -> bool:
        if not isinstance(expr, ast.Call):
            return False
        name = _dotted(expr.func)
        if name is None:
            return False
        tail = name.rsplit(".", 1)[-1]
        return any(spec.name.rsplit(".", 1)[-1] == tail for spec in self.sanitizers)

    def _resolve(self, expr: ast.AST) -> tuple[str, int] | None:
        """这个表达式脏不脏。脏就返回 (来源描述, 行号)。"""
        hit = self._source_hit(expr)
        if hit is not None:
            return hit
        if self._sanitized(expr):
            return None
        if isinstance(expr, ast.Name):
            return self.tainted.get(expr.id)
        return None

    # ---- 赋值 -----------------------------------------------------

    def _record(self, target: ast.AST, origin: tuple[str, int]) -> None:
        if isinstance(target, ast.Name):
            self.tainted[target.id] = origin
            return
        if isinstance(target, (ast.Tuple, ast.List)):
            for elt in target.elts:
                self._record(elt, origin)

    def visit_Assign(self, node: ast.Assign) -> None:
        origin = self._resolve(node.value)
        if origin is not None:
            for target in node.targets:
                self._record(target, origin)
        self.generic_visit(node)

    def visit_AnnAssign(self, node: ast.AnnAssign) -> None:
        if node.value is not None:
            origin = self._resolve(node.value)
            if origin is not None:
                self._record(node.target, origin)
        self.generic_visit(node)

    def visit_AugAssign(self, node: ast.AugAssign) -> None:
        origin = self._resolve(node.value) or self._resolve(node.target)
        if origin is not None:
            self._record(node.target, origin)
        self.generic_visit(node)

    # ---- 危险调用 -------------------------------------------------

    def _sink_args(self, call: ast.Call, arg_index: int) -> Iterable[ast.AST]:
        if arg_index >= 0:
            if arg_index < len(call.args):
                yield call.args[arg_index]
            return
        yield from call.args

    def visit_Call(self, node: ast.Call) -> None:
        name = _dotted(node.func)
        if name is not None:
            for spec in self.sinks:
                if spec.name != name:
                    continue
                for arg in self._sink_args(node, spec.arg):
                    origin = self._resolve(arg)
                    if origin is None:
                        continue
                    self.findings.append(
                        Finding(
                            path=self.path,
                            line=origin[1],
                            col=node.col_offset,
                            rule=self.rule.id,
                            sink=spec.name,
                            trace=(origin[0], spec.name),
                        )
                    )
                    return
        self.generic_visit(node)

    # ---- 作用域 ---------------------------------------------------

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        saved = self.tainted
        self.tainted = {}
        for stmt in node.body:
            self.visit(stmt)
        self.tainted = saved

    visit_AsyncFunctionDef = visit_FunctionDef


def analyze_source(source: str, path: str, rule: Rule) -> list[Finding]:
    """扫一段源码。语法错误直接抛 SyntaxError，由调用方决定怎么处理。"""
    tree = ast.parse(source, filename=path)
    walker = _Walker(path, rule)
    for stmt in tree.body:
        walker.visit(stmt)
    return walker.findings


def analyze_paths(paths: Iterable[str | Path], rule: Rule) -> list[Finding]:
    """扫一批文件。读不了或语法错的跳过，不让一个坏文件带走整轮扫描。"""
    findings: list[Finding] = []
    for raw in paths:
        path = Path(raw)
        if path.suffix != ".py" or not path.is_file():
            continue
        try:
            source = path.read_text(encoding="utf-8", errors="replace")
            findings.extend(analyze_source(source, str(path), rule))
        except SyntaxError:
            continue
    return findings
