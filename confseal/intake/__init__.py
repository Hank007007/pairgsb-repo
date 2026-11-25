"""Reading a submitted policy text into a node tree."""

from .anchors import MAX_ALIAS_DEPTH, MAX_NODES, Budget, expand
from .loader import load_document

__all__ = [
    "MAX_ALIAS_DEPTH",
    "MAX_NODES",
    "Budget",
    "expand",
    "load_document",
]
