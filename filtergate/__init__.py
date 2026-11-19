"""filtergate — multi-tenant filter DSL gateway for log search."""

from __future__ import annotations

from .adapters import MongoAdapter
from .errors import ErrorCode
from .gate import CompileResult, FilterGate

__all__ = ["CompileResult", "ErrorCode", "FilterGate", "MongoAdapter"]
__version__ = "0.1.0"
