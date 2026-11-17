"""Built-in platform defaults for tenant configuration."""

from __future__ import annotations

from typing import Any, Dict

# The process-wide default configuration.  Every tenant load starts from this
# document before the tenant overlay is applied.
DEFAULTS: Dict[str, Any] = {
    "quota": {
        "requests_per_minute": 600,
        "burst": 1200,
        "storage_mb": 1024,
        "max_upload_mb": 32,
    },
    "callbacks": {
        "primary": None,
        "fallback": None,
        "timeout_ms": 5000,
        "retries": 2,
    },
    "features": {
        "exports": True,
        "webhooks": False,
        "audit_log": False,
    },
    "internal": {
        "routing": {
            "region": "us-east-1",
            "shard": 0,
        },
        "limits": {
            "hard_ceiling_rpm": 100000,
        },
        "revision": 1,
    },
}
