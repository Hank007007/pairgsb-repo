"""污点分析引擎：只走 AST，不执行被扫代码。

两遍：先扫一遍本文件自己的 ``import`` 和顶层函数/类，建立别名表与函数索引；
再按源码顺序走，维护"哪些名字/属性脏了，脏从哪来"，碰到规则里的危险调用就
看它实参脏不脏。命中的行号列号取危险调用本身，trace 保留来源与危险点两段。

追踪范围按 docs/rule-semantics.md：单文件、一层直接调用，不做递归、多层调用
链、方解析和包解析。
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
    """归一化后的点分全名按组件边界比较：相等或是条目的下级才算命中。

    ``os.environ.get`` 命中 ``os.environ``；``request.args`` 不命中
    ``flask.request.args``，不做后缀匹配。
    """
    for entry in table:
        if name == entry or name.startswith(entry + "."):
            return entry
    return None


def _origin(desc: str, node: ast.AST) -> tuple[str, int]:
    return desc, node.lineno


class _Walker(ast.NodeVisitor):
    """按顺序走一遍模块，维护"哪些名字/属性脏了"。

    模块级名字表在整个文件内可见；每个函数另起一张局部名字表，进函数清空，
    赋值不外溢。类的实例方法共用一张 ``self.*`` 属性表，普通属性按模块级处理。
    """

    def __init__(self, path: str, rule: Rule, tree: ast.Module) -> None:
        self.path = path
        self.rule = rule
        self.sources = tuple(rule.sources)
        self.sinks = tuple(rule.sinks)
        self.sanitizers = tuple(rule.sanitizers)
        # 别名绑定：本地名字 -> 归一化点分全名
        self.aliases: dict[str, str] = {}
        # 本文件顶层函数：名字 -> FunctionDef
        self.funcs: dict[str, ast.FunctionDef] = {}
        # 类名 -> self.* 属性污点表
        self.class_attrs: dict[str, dict[str, tuple[str, int]]] = {}
        # 函数节点 id -> 所属类名（模块级函数为 None）
        self.enclosing_class: dict[int, str | None] = {}
        # 函数节点 id -> 返回值污点
        self.return_taint: dict[int, tuple[str, int]] = {}
        # 模块级名字 / 非 self 属性 / 下标的污点表
        self.names: dict[str, tuple[str, int]] = {}
        self.attrs: dict[str, tuple[str, int]] = {}
        self.subscripts: dict[str, tuple[str, int]] = {}
        # 当前函数的局部名字表、self 属性表、函数节点 id
        self.locals: dict[str, tuple[str, int]] | None = None
        self.self_table: dict[str, tuple[str, int]] | None = None
        self.current_function: int | None = None
        self.seed_self_params: bool = False
        self.current_args: list[ast.arg] = []
        self.findings: list[Finding] = []
        self.reported: set[int] = set()
        self._collect_index(tree)

    # ---- 预备：别名与函数索引 -------------------------------------

    def _collect_index(self, tree: ast.Module) -> None:
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name == "*":
                        continue
                    bound = alias.asname or alias.name.split(".")[0]
                    full = alias.name if alias.asname else alias.name.split(".")[0]
                    self.aliases[bound] = full
            elif isinstance(node, ast.ImportFrom):
                if node.module is None:
                    continue
                for alias in node.names:
                    if alias.name == "*":
                        continue
                    bound = alias.asname or alias.name
                    self.aliases[bound] = f"{node.module}.{alias.name}"
        for stmt in tree.body:
            if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
                self.funcs[stmt.name] = stmt
                self.enclosing_class[id(stmt)] = None
            elif isinstance(stmt, ast.ClassDef):
                for item in stmt.body:
                    if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                        self.enclosing_class[id(item)] = stmt.name

    def _normalize(self, node: ast.AST) -> str | None:
        """把纯名字链按本文件 import 别名还原成点分全名。"""
        dotted = _dotted(node)
        if dotted is None:
            return None
        head, _, rest = dotted.partition(".")
        root = self.aliases.get(head)
        if root is None:
            return dotted
        return f"{root}.{rest}" if rest else root

    def _resolve_name(self, node: ast.AST) -> str | None:
        """规则匹配用的全名：多段名字的根必须是本文件的 import 绑定。

        没 import 过的 ``shlex.quote`` 不能靠"尾巴像"命中规则全名；单段名字是
        ``eval`` / ``int`` 这类内建，没有 import 也按全名处理。
        """
        dotted = _dotted(node)
        if dotted is None:
            return None
        head, _, rest = dotted.partition(".")
        root = self.aliases.get(head)
        if root is not None:
            return f"{root}.{rest}" if rest else root
        return dotted if not rest else None

    # ---- 污点来源与净化器 -----------------------------------------

    def _source_hit(self, expr: ast.AST) -> tuple[str, int] | None:
        """表达式本身就在取污点。``request.args["q"]`` 这种下标也认。"""
        node = expr
        while isinstance(node, ast.Subscript):
            node = node.value
        if isinstance(node, ast.Call):
            name = self._resolve_name(node.func)
            if name is not None:
                hit = _matches(name, self.sources)
                if hit is not None:
                    return _origin(f"{hit}()", node)
            return None
        name = self._resolve_name(node)
        if name is not None:
            hit = _matches(name, self.sources)
            if hit is not None:
                return _origin(hit, node)
        return None

    def _sanitizer_hit(self, call: ast.Call) -> bool:
        name = self._resolve_name(call.func)
        return (
            name is not None
            and _matches(name, [spec.name for spec in self.sanitizers]) is not None
        )

    # ---- 表达式污点解析 -------------------------------------------

    def _resolve(
        self, expr: ast.AST | None, *, sink_arg: bool = False
    ) -> tuple[str, int] | None:
        """这个表达式脏不脏。脏就返回 (来源描述, 来源行号)。

        sink_arg=True 表示它正作为危险调用的被查实参：裸集合字面量此时不算脏，
        保持"容器里的元素脏不等于容器实参脏"的口径（例如 subprocess.run 的命令
        数组）；容器被下标、拼接或包一层之后再传则照常传播。
        """
        if expr is None:
            return None

        hit = self._source_hit(expr)
        if hit is not None:
            return hit

        if isinstance(expr, ast.Call):
            return self._resolve_call(expr)

        if isinstance(expr, ast.Name):
            if self.locals is not None and expr.id in self.locals:
                return self.locals[expr.id]
            return self.names.get(expr.id)

        if isinstance(expr, ast.Attribute):
            if self._is_self(expr.value):
                assert self.self_table is not None
                return self.self_table.get(expr.attr)
            dotted = self._normalize(expr)
            if dotted is not None:
                return self.attrs.get(dotted)
            return None

        if isinstance(expr, ast.Subscript):
            key = self._subscript_key(expr)
            if key is not None and key in self.subscripts:
                return self.subscripts[key]
            return self._resolve(expr.value) or self._resolve(expr.slice)

        if sink_arg and isinstance(expr, (ast.List, ast.Tuple, ast.Set, ast.Dict)):
            return None

        if isinstance(
            expr,
            (
                ast.BinOp,
                ast.BoolOp,
                ast.UnaryOp,
                ast.Compare,
                ast.IfExp,
                ast.JoinedStr,
                ast.FormattedValue,
                ast.Starred,
                ast.Await,
                ast.ListComp,
                ast.SetComp,
                ast.GeneratorExp,
                ast.DictComp,
                ast.List,
                ast.Tuple,
                ast.Set,
                ast.Dict,
            ),
        ):
            return self._first_tainted(self._child_nodes(expr))

        return None

    @staticmethod
    def _child_nodes(expr: ast.AST) -> list[ast.AST]:
        children: list[ast.AST] = []
        for field_name in expr._fields:
            value = getattr(expr, field_name)
            if isinstance(value, ast.AST):
                children.append(value)
            elif isinstance(value, list):
                children.extend(item for item in value if isinstance(item, ast.AST))
        return children

    def _first_tainted(self, nodes: Iterable[ast.AST]) -> tuple[str, int] | None:
        for node in nodes:
            origin = self._resolve(node)
            if origin is not None:
                return origin
        return None

    def _resolve_call(self, call: ast.Call) -> tuple[str, int] | None:
        # 净化器调用本身的结果不脏；别的实参该脏还是脏，由外层表达式自行解析。
        if self._sanitizer_hit(call):
            return None

        # 本文件顶层函数：一层直接调用，按实参绑定形参后在调用处分析函数体。
        # 函数体内再调本地函数属于递归/多层调用链，不在本期范围，直接按普通调用
        # 处理（实参脏则返回值脏），也避免自调用无限展开。
        if (
            self.locals is None
            and isinstance(call.func, ast.Name)
            and call.func.id in self.funcs
        ):
            return self._call_local_function(self.funcs[call.func.id], call)

        # 其它调用（内建、方法、未知函数）：实参脏则返回值脏。
        values = list(call.args) + [kw.value for kw in call.keywords]
        return self._first_tainted(values)

    def _call_local_function(
        self, func: ast.FunctionDef, call: ast.Call
    ) -> tuple[str, int] | None:
        bindings = self._bind_arguments(func, call)
        return self._analyze_function(func, bindings)

    def _bind_arguments(
        self, func: ast.FunctionDef, call: ast.Call
    ) -> dict[str, tuple[str, int]]:
        """把调用处实参的污点按形参名绑定。只传污点，不做求值。"""
        params = [arg.arg for arg in func.args.args]
        kwonly = [arg.arg for arg in func.args.kwonlyargs]
        bindings: dict[str, tuple[str, int]] = {}

        for index, param in enumerate(params):
            if index < len(call.args):
                origin = self._resolve(call.args[index])
                if origin is not None:
                    bindings[param] = origin

        for kw in call.keywords:
            if kw.arg is not None and (kw.arg in params or kw.arg in kwonly):
                origin = self._resolve(kw.value)
                if origin is not None:
                    bindings[kw.arg] = origin

        # *args / **kwargs：任一带进来的实参脏，接收它的形参就脏。
        star_origin = self._first_tainted(
            [arg for arg in call.args if isinstance(arg, ast.Starred)]
            + [kw.value for kw in call.keywords if kw.arg is None]
        )
        if star_origin is not None:
            if func.args.vararg is not None:
                bindings[func.args.vararg.arg] = star_origin
            if func.args.kwarg is not None:
                bindings[func.args.kwarg.arg] = star_origin
        return bindings

    def _analyze_function(
        self,
        func: ast.FunctionDef,
        bindings: dict[str, tuple[str, int]],
        seed_params: bool = False,
    ) -> tuple[str, int] | None:
        saved_locals = self.locals
        saved_self = self.self_table
        saved_function = self.current_function
        saved_args = self.current_args
        saved_seed = self.seed_self_params
        self.locals = dict(bindings)
        self.current_function = id(func)
        self.current_args = list(func.args.args) + list(func.args.kwonlyargs)

        is_method = bool(func.args.args) and func.args.args[0].arg == "self"
        if is_method:
            class_name = self.enclosing_class.get(id(func))
            self.self_table = self.class_attrs.setdefault(class_name or func.name, {})
        else:
            self.self_table = None

        # 没有调用点的实例方法（如 __init__）：形参本身不预设脏，等它被写进
        # self.* 时再按脏处理（见 visit_Assign / _record），让 __init__ 接到的
        # 脏数据能进类共享属性表，又不会让没存进 self 的形参凭空变脏。
        self.seed_self_params = seed_params and is_method

        self.return_taint.pop(id(func), None)
        for stmt in func.body:
            self.visit(stmt)

        returned = self.return_taint.pop(id(func), None)
        self.locals = saved_locals
        self.self_table = saved_self
        self.current_function = saved_function
        self.current_args = saved_args
        self.seed_self_params = saved_seed
        return returned

    def visit_Return(self, node: ast.Return) -> None:
        if node.value is not None and self.current_function is not None:
            func_id = self.current_function
            origin = self._resolve(node.value)
            if origin is not None and func_id not in self.return_taint:
                self.return_taint[func_id] = origin
        self.generic_visit(node)

    # ---- 赋值 -----------------------------------------------------

    def _record(self, target: ast.AST, origin: tuple[str, int]) -> None:
        if isinstance(target, ast.Name):
            if self.locals is not None:
                self.locals[target.id] = origin
            else:
                self.names[target.id] = origin
            return
        if isinstance(target, ast.Starred):
            self._record(target.value, origin)
            return
        if isinstance(target, (ast.Tuple, ast.List)):
            for elt in target.elts:
                self._record(elt, origin)
            return
        if isinstance(target, ast.Attribute):
            if self._is_self(target.value):
                assert self.self_table is not None
                self.self_table[target.attr] = origin
                return
            dotted = self._normalize(target)
            if dotted is not None:
                self.attrs[dotted] = origin
            return
        if isinstance(target, ast.Subscript):
            key = self._subscript_key(target)
            if key is not None:
                self.subscripts[key] = origin

    def _subscript_key(self, target: ast.Subscript) -> str | None:
        if isinstance(target.value, ast.Name):
            base = target.value.id
        else:
            dotted = self._normalize(target.value)
            if dotted is None:
                return None
            base = dotted
        slice_node = target.slice
        if isinstance(slice_node, ast.Constant):
            return f"{base}[{slice_node.value!r}]"
        return f"{base}[]"

    @staticmethod
    def _is_self(node: ast.AST) -> bool:
        return isinstance(node, ast.Name) and node.id == "self"

    def _self_param_origin(self, value: ast.AST) -> tuple[str, int] | None:
        """无调用点的实例方法里，``self.x = 形参`` 时把该形参视为脏。

        只给真正写进 ``self.*`` 的形参种污点，覆盖 ``__init__`` 接脏数据的场景，
        又不会让没存进 self 的形参凭空变脏。
        """
        if not self.seed_self_params or not isinstance(value, ast.Name):
            return None
        if self.locals is not None and value.id in self.locals:
            return None
        param_names = {
            arg.arg
            for arg in self.current_args
            if arg.arg != "self"
        }
        if value.id not in param_names:
            return None
        return _origin(f"param:{value.id}", value)

    def visit_Assign(self, node: ast.Assign) -> None:
        origin = self._resolve(node.value)
        if origin is None:
            origin = self._self_param_origin(node.value)
        if origin is not None:
            for target in node.targets:
                self._record(target, origin)
        self.generic_visit(node)

    def visit_AnnAssign(self, node: ast.AnnAssign) -> None:
        if node.value is not None:
            origin = self._resolve(node.value)
            if origin is None:
                origin = self._self_param_origin(node.value)
            if origin is not None:
                self._record(node.target, origin)
        self.generic_visit(node)

    def visit_AugAssign(self, node: ast.AugAssign) -> None:
        origin = self._resolve(node.value) or self._resolve(node.target)
        if origin is not None:
            self._record(node.target, origin)
        self.generic_visit(node)

    def visit_NamedExpr(self, node: ast.NamedExpr) -> None:
        origin = self._resolve(node.value)
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
        name = self._resolve_name(node.func)
        if name is not None:
            for spec in self.sinks:
                if _matches(name, [spec.name]) is None:
                    continue
                for arg in self._sink_args(node, spec.arg):
                    origin = self._resolve(arg, sink_arg=True)
                    if origin is None:
                        continue
                    self._report(node, spec.name, origin)
                    break
                break
        # 模块级的裸本地函数调用（返回值没人消费）也要分析那一层函数体；
        # 函数体内的本地调用属于多层调用链，不在本期范围。
        if (
            self.locals is None
            and isinstance(node.func, ast.Name)
            and node.func.id in self.funcs
        ):
            self._call_local_function(self.funcs[node.func.id], node)
        self.generic_visit(node)

    def _report(
        self, node: ast.Call, sink_name: str, origin: tuple[str, int]
    ) -> None:
        # 同一个危险调用节点只报一条；行号列号取危险调用本身。
        if id(node) in self.reported:
            return
        self.reported.add(id(node))
        self.findings.append(
            Finding(
                path=self.path,
                line=node.lineno,
                col=node.col_offset,
                rule=self.rule.id,
                sink=sink_name,
                trace=(origin[0], sink_name),
            )
        )

    # ---- 作用域 ---------------------------------------------------

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        # 模块级函数体只在被一层直接调用时分析（见 _call_local_function），
        # 函数内的赋值不外溢到模块级。
        if self.locals is not None:
            return

    visit_AsyncFunctionDef = visit_FunctionDef

    def visit_ClassDef(self, node: ast.ClassDef) -> None:
        # 类里非方法的语句按模块级处理；方法按源码顺序独立分析，共用 self.* 表。
        for item in node.body:
            if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                self._analyze_function(item, {}, seed_params=True)
            else:
                self.visit(item)


def analyze_source(source: str, path: str, rule: Rule) -> list[Finding]:
    """扫一段源码。语法错误直接抛 SyntaxError，由调用方决定怎么处理。"""
    tree = ast.parse(source, filename=path)
    walker = _Walker(path, rule, tree)
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
