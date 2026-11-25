"""confseal - intake for outbound-filter policy documents.

The package is split in two layers:

``confseal.intake``
    Turns the text submitted by an operator into a plain-Python tree:
    one document, no aliases left, string keys.

``confseal.policy``
    Turns that tree into the policy object handed to the filter engine.

``confseal.api``
    Glues both layers together behind the JSON response used by the
    upload endpoint.
"""

from .api import handle_upload
from .intake import Budget, expand, load_document
from .policy import Policy, PolicyError, Rule, compile_policy

__all__ = [
    "Budget",
    "Policy",
    "PolicyError",
    "Rule",
    "compile_policy",
    "expand",
    "handle_upload",
    "load_document",
]

__version__ = "0.1.0"
