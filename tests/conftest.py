"""Shared fixtures: a local recording HTTP server."""

from __future__ import annotations

import threading
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Callable

import pytest


@dataclass
class RecordedRequest:
    method: str
    target: str
    headers: dict[str, str]
    body: bytes

    @property
    def path(self) -> str:
        return self.target.split("?", 1)[0]

    @property
    def query(self) -> str:
        parts = self.target.split("?", 1)
        return parts[1] if len(parts) == 2 else ""

    def header(self, name: str) -> str | None:
        return self.headers.get(name.lower())


@dataclass
class Recorder:
    requests: list[RecordedRequest] = field(default_factory=list)
    responder: Callable[[RecordedRequest], tuple[int, dict[str, str], bytes]] | None = None

    def reply(self, status: int = 200, headers: dict[str, str] | None = None, body: bytes = b"ok") -> None:
        self.responder = lambda _req: (status, headers or {}, body)


class _Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _handle(self) -> None:  # noqa: N802 - stdlib naming
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length) if length else b""
        recorded = RecordedRequest(
            method=self.command,
            target=self.path,
            headers={name.lower(): value for name, value in self.headers.items()},
            body=body,
        )
        self.server.recorder.requests.append(recorded)
        responder = self.server.recorder.responder
        status, headers, payload = responder(recorded) if responder else (200, {}, b"ok")
        self.send_response(status)
        for name, value in headers.items():
            self.send_header(name, value)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if payload:
            self.wfile.write(payload)

    do_GET = _handle
    do_POST = _handle
    do_PUT = _handle

    def log_message(self, *_args) -> None:  # silence test output
        return


@pytest.fixture
def recorder():
    rec = Recorder()
    server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    server.recorder = rec
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield rec, server.server_address[1]
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)