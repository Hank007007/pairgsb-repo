"""credlint — a pre-commit credential scanner.

The package is split into small, single-purpose modules:

``walker``      enumerates candidate files
``rules``       loads the pattern table from ``rules/``
``entropy``     decides whether a value looks randomly generated
``decode``      peels one layer of base64 / URL-safe base64
``ignorefile``  allowlist matching
``scanner``     combines the pieces into findings
``report``      renders findings for humans and for CI
``redact``      masks secrets in text, URLs and log lines
"""

from .report import Finding, render
from .scanner import Scanner, scan_path, scan_text

__all__ = [
    "Finding",
    "Scanner",
    "render",
    "scan_path",
    "scan_text",
]

__version__ = "0.1.0"
