"""Error types shared by the cfgfold pipeline."""

from __future__ import annotations

from typing import Optional


class ConfigError(ValueError):
    """A tenant configuration document was rejected.

    ``code`` is the stable, machine-readable reason described in
    ``docs/config-rules.md``.  ``str(exc)`` always has the form
    ``cfgfold: <code>: <detail>``.
    """

    def __init__(
        self,
        code: str,
        message: str,
        *,
        line: Optional[int] = None,
        column: Optional[int] = None,
    ) -> None:
        self.code = code
        self.message = message
        self.line = line
        self.column = column

        detail = message
        if line is not None and column is not None:
            detail = f"{message} (line {line}, column {column})"
        elif line is not None:
            detail = f"{message} (line {line})"
        super().__init__(f"cfgfold: {code}: {detail}")

