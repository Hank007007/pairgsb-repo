"""pyscan —— 一个只跑 AST 的 Python 污点分析器。

对外只暴露三个东西：`Rule` 规则对象、`analyze_source` 与 `analyze_paths`。
"""

from .model import Finding, Rule, SanitizerSpec, SinkSpec
from .engine import analyze_source, analyze_paths
from .rules import DEFAULT_RULE

__all__ = [
    "Finding",
    "Rule",
    "SanitizerSpec",
    "SinkSpec",
    "DEFAULT_RULE",
    "analyze_source",
    "analyze_paths",
]
__version__ = "0.6.2"
