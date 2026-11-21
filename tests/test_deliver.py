"""Happy-path tests for the delivery entry point."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

import pytest

from outpostcall import deliver
from outpostcall.model import Event, Integration
from outpostcall.sign import DIGEST_HEADER, SIGNATURE_HEADER, TIMESTAMP_HEADER

FIXTURES = Path(__file__).parent / "fixtures"


def load_event() -> Event:
    data = json.loads((FIXTURES / "event.json").read_text(encoding="utf-8"))
    return Event(fields=data["fields"], body=data["body"].encode("utf-8"))


def make_integration(port: int) -> Integration:
    return Integration(
        base_url=f"http://127.0.0.1:{port}",
        path_template="/v1/orders/{order_id}/events",
        secret=b"test-secret",
        domains=("127.0.0.1",),
        allow_insecure=True,
    )


def test_deliver_posts_to_rendered_template(recorder):
    rec, port = recorder
    integration = make_integration(port)
    event = Event(fields={"order_id": "A1001"}, body=b'{"ok":true}')

    result = deliver(integration, event)

    assert result.status_code == 200
    assert result.ok
    assert len(rec.requests) == 1
    sent = rec.requests[0]
    assert sent.method == "POST"
    assert sent.target == "/v1/orders/A1001/events"
    assert sent.body == b'{"ok":true}'


def test_deliver_emits_signature_headers(recorder):
    rec, port = recorder
    integration = make_integration(port)
    event = Event(fields={"order_id": "A1001"}, body=b"payload")

    deliver(integration, event)

    sent = rec.requests[0]
    assert sent.header(TIMESTAMP_HEADER) is not None
    assert sent.header(SIGNATURE_HEADER).startswith("v1=")
    expected = "sha-256=" + hashlib.sha256(b"payload").hexdigest()
    assert sent.header(DIGEST_HEADER) == expected


def test_deliver_forwards_static_headers(recorder):
    rec, port = recorder
    integration = Integration(
        base_url=f"http://127.0.0.1:{port}",
        path_template="/hooks/{kind}",
        secret=b"test-secret",
        headers={"X-Tenant-ID": "t-42"},
        domains=("127.0.0.1",),
        allow_insecure=True,
    )

    deliver(integration, Event(fields={"kind": "created"}))

    sent = rec.requests[0]
    assert sent.target == "/hooks/created"
    assert sent.header("X-Tenant-ID") == "t-42"


def test_deliver_reports_error_status(recorder):
    rec, port = recorder
    rec.reply(status=204, body=b"")
    integration = make_integration(port)

    result = deliver(integration, Event(fields={"order_id": "B7"}))

    assert result.status_code == 204
    assert result.ok


def test_delivery_with_root_base(recorder):
    rec, port = recorder
    integration = Integration(
        base_url=f"http://127.0.0.1:{port}/",
        path_template="/c/{name}",
        secret=b"test-secret",
        domains=("127.0.0.1",),
        allow_insecure=True,
    )

    deliver(integration, Event(fields={"name": "alpha"}))

    assert rec.requests[0].path == "/c/alpha"


def test_delivery_with_fixture_event(recorder):
    rec, port = recorder
    integration = make_integration(port)
    event = load_event()

    deliver(integration, event)

    sent = rec.requests[0]
    assert sent.target == "/v1/orders/A1001/events"
    assert sent.body == event.body