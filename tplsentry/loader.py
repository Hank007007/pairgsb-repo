"""Template lookup and caching."""

from __future__ import annotations

import os

from .errors import TemplateNotFoundError
from .parser import Template


class TemplateLoader:
    """Loads templates from a directory on disk."""

    def __init__(self, template_dir: str, cache: bool = True) -> None:
        self.template_dir = os.path.abspath(template_dir)
        self.cache = cache
        self._cache: dict[str, Template] = {}

    def resolve_template(self, name: str) -> str:
        """Return the filesystem path for the template called ``name``."""

        path = os.path.join(self.template_dir, name)
        return path

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
