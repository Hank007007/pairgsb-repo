"""rotseal: rotate, seal and verify local NDJSON application logs."""

__version__ = "0.1.0"

from .chain import Report, verify_chain
from .keys import add_key, active_key, load_keyring
from .retention import prune
from .rotate import rotate
from .sealing import seal, seal_segment

__all__ = [
    "Report",
    "add_key",
    "active_key",
    "load_keyring",
    "prune",
    "rotate",
    "seal",
    "seal_segment",
    "verify_chain",
    "__version__",
]
