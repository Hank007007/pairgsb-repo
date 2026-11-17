"""cfgfold — the control plane's tenant configuration loader."""

from __future__ import annotations

from .defaults import DEFAULTS
from .errors import ConfigError
from .loader import (
    MAX_DEPTH,
    MAX_DOCUMENT_BYTES,
    MAX_NODES,
    load_document,
)
from .merge import deep_merge
from .tenant import load_tenant_config

__all__ = [
    "ConfigError",
    "DEFAULTS",
    "MAX_DEPTH",
    "MAX_DOCUMENT_BYTES",
    "MAX_NODES",
    "deep_merge",
    "load_document",
    "load_tenant_config",
]
