"""Data types shared by the delivery pipeline."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Mapping, Sequence

from .policy import EgressPolicy


@dataclass(frozen=True)
class Integration:
    """A tenant-configured webhook integration."""

    base_url: str
    path_template: str
    secret: bytes
    headers: Mapping[str, str] = field(default_factory=dict)
    dynamic_headers: Mapping[str, str] = field(default_factory=dict)
    domains: Sequence[str] = field(default_factory=tuple)
    allow_insecure: bool = False
    sign_headers: Sequence[str] = field(default_factory=tuple)

    @property
    def policy(self) -> EgressPolicy:
        return EgressPolicy(domains=tuple(self.domains), allow_insecure=self.allow_insecure)


@dataclass(frozen=True)
class Event:
    """An event to deliver. ``fields`` feed the path template and dynamic headers."""

    fields: Mapping[str, Any] = field(default_factory=dict)
    body: bytes = b""

    @classmethod
    def from_parts(cls, fields: Mapping[str, Any] | None = None, body: bytes = b"") -> "Event":
        return cls(fields=dict(fields or {}), body=body)


@dataclass
class DeliveryResult:
    """The outcome of a delivery attempt."""

    status_code: int
    headers: Mapping[str, str]
    body: bytes
    url: str
    history: list[str] = field(default_factory=list)

    @property
    def ok(self) -> bool:
        return 200 <= self.status_code < 300