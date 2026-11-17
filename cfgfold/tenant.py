"""Entry point used by the control plane to render a tenant's configuration."""

from __future__ import annotations

from .defaults import DEFAULTS
from .loader import load_document
from .merge import deep_merge


def load_tenant_config(text: str) -> dict:
    """Return the effective configuration for one tenant upload.

    ``text`` is the document exactly as the tenant uploaded it: YAML by
    default, or JSON when the document opens with ``{``.  The result is the
    platform defaults with the tenant overlay applied.
    """
    overlay = load_document(text)
    return deep_merge(DEFAULTS, overlay)
