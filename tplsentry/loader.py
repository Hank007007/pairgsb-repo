"""Template lookup and caching."""

from __future__ import annotations

import os
import re

from .errors import TemplateNotFoundError, TemplateSecurityError
from .parser import Template

_DRIVE_RE = re.compile(r"^[A-Za-z]:")


class TemplateLoader:
    """Loads templates from a directory on disk."""

    def __init__(self, template_dir: str, cache: bool = True) -> None:
        self.template_dir = os.path.realpath(template_dir)
        self.cache = cache
        self._cache: dict[str, Template] = {}

    def resolve_template(self, name: str) -> str:
        """Return the filesystem path for the template called ``name``."""

        if not isinstance(name, str) or not name.strip():
            raise TemplateSecurityError("template name must be a non-empty relative path")
        if (
            os.path.isabs(name)
            or name.startswith(("/", "\\"))
            or _DRIVE_RE.match(name)
        ):
            raise TemplateSecurityError(
                f"absolute template path is not allowed: {name!r}"
            )
        candidate = os.path.realpath(os.path.join(self.template_dir, name))
        root = self.template_dir
        if candidate != root and not candidate.startswith(root + os.sep):
            raise TemplateSecurityError(
                f"template name escapes the template root: {name!r}"
            )
        return candidate

    def read_source(self, name: str) -> str:
        path = self.resolve_template(name)
        try:
            with open(path, "r", encoding="utf-8") as handle:
                return handle.read()
        except OSError as exc:
            raise TemplateNotFoundError(f"cannot read template {name!r}: {exc}") from exc

    def get_template(self, name: str) -> Template:
        if self.cache and name in self._cache:
            return self._cache[name]
        template = Template(self.read_source(name), name)
        template.loader = self
        if self.cache:
            self._cache[name] = template
        return template

    def clear(self) -> None:
        self._cache.clear()
