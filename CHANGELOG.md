# Changelog

本文件记录 pyscan 的用户可见变更，格式参考 Keep a Changelog。

## Unreleased

### 修复

- 别名还原：按本文件自己的 `import` / `from ... import` 建立别名表，
  `import os as o`、`from os import system`、`import subprocess as sp`
  等写法都能还原成点分全名再匹配；去掉后缀匹配，`request.args` 不再命中
  `flask.request.args`，多段全名的根名字必须来自 import（单段内建名除外）。
- 赋值传播：`Attribute`（`self.cmd = q`、`cfg.cmd = q`）、`Subscript`
  （`opts["cmd"] = q`）、`Tuple`/`List` 逐元素解包和链式赋值都会传播污点。
- 一层函数：同文件内一层直接调用会把实参污点绑到形参，并追踪 `return`
  带出的返回值污点；递归、多层调用链、方解析按语义不做，递归不会再卡死。
- 组合表达式：`BinOp`、`JoinedStr`、`IfExp`、`Subscript`、`Dict`、
  集合字面量等子表达式脏则整体脏；净化器之外的 `Call` 实参脏则返回值脏。
- 净化器：`SanitizerSpec.name` 按归一化全名做组件边界精确匹配，不再按
  名字尾巴匹配；净化器调用结果不脏，未被净化的兄弟实参照常报。
- 报告位置：`Finding.line` / `Finding.col` 改报危险调用本身的行列，
  不再报污染源那一行；`trace` 保留来源与危险点两段。
- 作用域：进函数清空局部变量表且赋值不外溢；同一个类的实例方法共用一张
  `self.*` 属性表，`__init__` 写进 `self.cmd` 的污点在其它方法里接着脏。

### 测试

- 新增 `tests/test_taint_engine.py`，覆盖 `docs/feedback-notes.md`
  的每条反馈与 `docs/rule-semantics.md` §3.3 列出的全部组合表达式；
  基线 `tests/test_baseline.py` 保持不变。
