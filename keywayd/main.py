"""keywayd — process entry point for keywaytransit.

Runs a tiny JSON-over-HTTP service backed by a key directory.
"""

from __future__ import annotations

import argparse
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from keyway.audit import AuditLog, NullAudit
from keyway.http.routes import Request, Router


def build_router(args) -> Router:
    from keyway.keystore import KeyStore

    store = KeyStore(args.keys_dir)
    audit = AuditLog(args.audit_log) if args.audit_log else NullAudit()
    return Router(store, audit)


class Handler(BaseHTTPRequestHandler):
    router: Router = None  # set on the server instance

    def do_POST(self):  # noqa: N802 - stdlib naming
        length = int(self.headers.get("content-length", "0") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        try:
            body = json.loads(raw.decode("utf-8") or "{}")
        except (UnicodeDecodeError, json.JSONDecodeError):
            self._reply(400, {"error": "invalid_request"})
            return

        request = Request(
            method="POST",
            path=self.path.split("?")[0],
            headers={k.lower(): v for k, v in self.headers.items()},
            body=body,
        )
        response = self.router.dispatch(request)
        self._reply(response.status, response.body)

    def _reply(self, status: int, body: dict):
        payload = json.dumps(body, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *args):  # silence default access logging
        return None


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="keywayd")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8080)
    parser.add_argument("--keys-dir", default="keys")
    parser.add_argument("--audit-log", default=None)
    args = parser.parse_args(argv)

    router = build_router(args)
    Handler.router = router
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"keywayd listening on {args.host}:{args.port}", file=sys.stderr)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
