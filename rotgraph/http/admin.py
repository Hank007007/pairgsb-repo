"""Admin HTTP surface used by ``rotgraphd``.

Two endpoints::

    POST /rotate      body: {"plan": "<path>"}  -> run a batch, return its result
    GET  /healthz                              -> {"ok": true, "version": ...}
"""

from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import urlparse

from .. import __version__
from ..plan import PlanError, load_plan
from ..runner import execute


class AdminState:
    """Process-wide settings the handler needs."""

    def __init__(self, *, journal_dir: str | None = None):
        self.journal_dir = journal_dir
        self.lock = threading.Lock()


def _journal_for(state: AdminState, batch_or_path: str, batch: str | None = None) -> str | None:
    if not state.journal_dir:
        return None
    import os

    name = batch or batch_or_path
    return os.path.join(state.journal_dir, f"{name}.journal")


def make_handler(state: AdminState):
    class AdminHandler(BaseHTTPRequestHandler):
        server_version = f"rotgraph/{__version__}"

        def log_message(self, fmt, *args):  # keep the daemon quiet by default
            return

        def _send(self, status: int, payload: Any) -> None:
            encoded = json.dumps(payload, sort_keys=True).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def do_GET(self):  # noqa: N802 - stdlib naming
            if urlparse(self.path).path == "/healthz":
                self._send(200, {"ok": True, "version": __version__})
                return
            self._send(404, {"ok": False, "error": "not found"})

        def do_POST(self):  # noqa: N802 - stdlib naming
            if urlparse(self.path).path != "/rotate":
                self._send(404, {"ok": False, "error": "not found"})
                return
            length = int(self.headers.get("Content-Length", "0") or 0)
            try:
                request = json.loads(self.rfile.read(length) or b"{}")
            except ValueError:
                self._send(400, {"ok": False, "error": "invalid JSON body"})
                return
            plan_path = request.get("plan")
            if not isinstance(plan_path, str) or not plan_path:
                self._send(400, {"ok": False, "error": "plan is required"})
                return
            with state.lock:
                try:
                    plan = load_plan(plan_path)
                    journal_path = _journal_for(state, plan_path, plan.batch)
                    result = execute(plan, journal_path=journal_path)
                except PlanError as exc:
                    code = getattr(exc, "kind", "invalid_plan")
                    self._send(
                        400,
                        {
                            "ok": False,
                            "error": str(exc),
                            "error_code": code,
                            "error_kind": code,
                        },
                    )
                    return
            if result.ok:
                status = 200
            elif result.error_code in {
                "invalid_plan",
                "duplicate_key",
                "cycle",
                "unknown_dependency",
            }:
                status = 400
            else:
                status = 500
            self._send(status, result.to_dict())

    return AdminHandler


def serve(host: str = "127.0.0.1", port: int = 8080, *, journal_dir: str | None = None) -> None:
    """Run the admin server until interrupted."""
    state = AdminState(journal_dir=journal_dir)
    server = ThreadingHTTPServer((host, port), make_handler(state))
    try:
        server.serve_forever()
    finally:
        server.server_close()
