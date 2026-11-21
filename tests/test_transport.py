"""Transport tests against a local recording server."""

from __future__ import annotations

import pytest

from outpostcall import transport
from outpostcall.errors import TransportError
from outpostcall.policy import EgressPolicy

ALLOW_LOCAL = EgressPolicy(domains=("127.0.0.1",), allow_insecure=True)


def test_send_returns_response(recorder):
    rec, port = recorder
    result = transport.send("POST", f"http://127.0.0.1:{port}/x", {}, b"hi", ALLOW_LOCAL)
    assert result.status_code == 200
    assert result.body == b"ok"
    assert rec.requests[0].target == "/x"


def test_send_follows_redirect(recorder):
    rec, port = recorder
    target = f"http://127.0.0.1:{port}/final"
    rec.responder = lambda req: (
        (302, {"Location": target}, b"")
        if req.path == "/start"
        else (200, {}, b"done")
    )

    result = transport.send("GET", f"http://127.0.0.1:{port}/start", {}, b"", ALLOW_LOCAL)

    assert result.status_code == 200
    assert [r.path for r in rec.requests] == ["/start", "/final"]


def test_send_follows_same_host_chain(recorder):
    rec, port = recorder

    def responder(req):
        if req.path == "/a":
            return 302, {"Location": "/b"}, b""
        if req.path == "/b":
            return 302, {"Location": "/c"}, b""
        return 200, {}, b"end"

    rec.responder = responder
    result = transport.send("GET", f"http://127.0.0.1:{port}/a", {}, b"", ALLOW_LOCAL)
    assert [r.path for r in rec.requests] == ["/a", "/b", "/c"]
    assert result.status_code == 200


def test_send_does_not_follow_304(recorder):
    rec, port = recorder
    rec.reply(status=304)
    result = transport.send("GET", f"http://127.0.0.1:{port}/x", {}, b"", ALLOW_LOCAL)
    assert result.status_code == 304
    assert len(rec.requests) == 1


def test_parse_retry_after_seconds():
    assert transport.parse_retry_after("3") == 3.0
    assert transport.parse_retry_after(None) is None