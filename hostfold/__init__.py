"""hostfold: authority normalization for an outbound forward proxy.

The proxy hands every upstream URL (or bare authority) to
:func:`hostfold.canonicalize` first.  The returned triple is the only thing
policy code is allowed to look at.
"""

from .errors import CanonicalizationError, PolicyConfigError
from .parse import Canonical, canonicalize, split_authority
from .policy import Policy, Rule, is_allowed, load_policy

__all__ = [
    "Canonical",
    "CanonicalizationError",
    "Policy",
    "PolicyConfigError",
    "Rule",
    "canonicalize",
    "is_allowed",
    "load_policy",
    "split_authority",
]

__version__ = "0.1.0"
