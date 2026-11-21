"""Delivery entry point: assemble a request from an integration and send it."""

from __future__ import annotations

from typing import Any, Mapping
from urllib.parse import urlsplit

from . import headers as header_mod
from . import queryset, sign, transport, urlbuild
from .model import DeliveryResult, Event, Integration


def deliver(
    integration: Integration,
    event: Event,
    params: Mapping[str, Any] | None = None,
    *,
    method: str = "POST",
    timestamp: int | None = None,
    max_redirects: int = transport.MAX_REDIRECTS,
) -> DeliveryResult:
    """Deliver ``event`` through ``integration`` and return the response."""
    policy = integration.policy

    url = urlbuild.join(integration.base_url, integration.path_template, event.fields)
    policy.check_url(url)

    request_headers = header_mod.assemble(
        integration.headers, integration.dynamic_headers, event.fields
    )

    stamp = timestamp if timestamp is not None else sign.now()
    target = urlsplit(url).path
    message = sign.canonical_string(method, target, request_headers, stamp, event.body)
    request_headers.update(
        sign.signature_headers(message, integration.secret, stamp, event.body)
    )

    url = queryset.build(url, params)

    return transport.send(
        method,
        url,
        request_headers,
        event.body,
        policy,
        max_redirects=max_redirects,
    )