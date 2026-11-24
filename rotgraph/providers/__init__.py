"""Vendor adapters.

Each adapter speaks one external service's "mint a key / stop a key" API and
exposes the same four operations, see ``docs/swap-window.md`` §2.
"""

from __future__ import annotations

from .base import Provider, ProviderError, build_providers  # noqa: F401

__all__ = ["Provider", "ProviderError", "build_providers"]
