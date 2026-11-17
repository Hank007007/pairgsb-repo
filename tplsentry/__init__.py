"""tplsentry — a sandboxed notification template engine."""

from __future__ import annotations

from .errors import (
    TemplateError,
    TemplateLimitError,
    TemplateNotFoundError,
    TemplateSecurityError,
    TemplateSyntaxError,
)
from .escape import Markup, escape
from .limits import DEFAULT_LIMITS, Limits
from .loader import TemplateLoader
from .parser import Template, TemplateParser
from .render import Renderer, render
from .sandbox import SafeContext

__all__ = [
    "DEFAULT_LIMITS",
    "Limits",
    "Markup",
    "Renderer",
    "SafeContext",
    "Template",
    "TemplateError",
    "TemplateLimitError",
    "TemplateLoader",
    "TemplateNotFoundError",
    "TemplateParser",
    "TemplateSecurityError",
    "TemplateSyntaxError",
    "escape",
    "render",
]

__version__ = "0.1.0"
