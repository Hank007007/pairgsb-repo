"""Package metadata for outpostcall-egress."""

from .api import deliver
from .errors import (
    ConfigError,
    EgressError,
    HeaderError,
    OutpostError,
    QueryError,
    RedirectError,
    RetryLimitExceeded,
    SignatureError,
    TransportError,
    URLBuildError,
)
from .model import DeliveryResult, Event, Integration

__all__ = [
    "deliver",
    "DeliveryResult",
    "Event",
    "Integration",
    "OutpostError",
    "ConfigError",
    "URLBuildError",
    "QueryError",
    "HeaderError",
    "SignatureError",
    "EgressError",
    "RedirectError",
    "TransportError",
    "RetryLimitExceeded",
]

__version__ = "0.1.0"