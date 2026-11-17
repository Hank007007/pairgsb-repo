"""Exception types raised by the template engine."""

from __future__ import annotations


class TemplateError(Exception):
    """Base class for every error raised while rendering a template."""


class TemplateSyntaxError(TemplateError):
    """The template source cannot be parsed."""


class TemplateNotFoundError(TemplateError):
    """A referenced template could not be located or read."""


class TemplateSecurityError(TemplateError):
    """The template attempted an access that the sandbox forbids."""


class TemplateLimitError(TemplateError):
    """Rendering exceeded one of the configured resource limits."""
