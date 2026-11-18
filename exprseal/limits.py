"""Hard limits that apply to every expression handled by the engine."""

from __future__ import annotations

MAX_EXPRESSION_BYTES = 512
MAX_TOKENS = 2048
MAX_AST_DEPTH = 32
MAX_EVAL_STEPS = 10_000
