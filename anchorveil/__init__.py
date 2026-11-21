"""anchorveil — a YAML-subset loader for internal configuration manifests."""

from .errors import (
    AliasError,
    AliasLimitError,
    DuplicateKeyError,
    ManifestError,
    MergeError,
    ParseError,
    TagError,
    UnknownTagError,
)
from .loader import load_file, load_text
from .tags import TagRegistry, default_registry

__all__ = [
    "AliasError",
    "AliasLimitError",
    "DuplicateKeyError",
    "ManifestError",
    "MergeError",
    "ParseError",
    "TagError",
    "TagRegistry",
    "UnknownTagError",
    "default_registry",
    "load_file",
    "load_text",
]
