"""最小可用的 Webhook 接收端（标准库 http.server）。

跑起来：

    python examples/receiver.py --secret-file .secret --port 8080

然后用 ``examples/sender.py`` 发一条带签名的请求过去。
"""

from __future__ import annotations

import argparse
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from hooksign import SecretRing, Secret  # noqa: E402
from hooksign.store import DeliveryStore  # noqa: E402
from hooksign.verify import SignedRequest, WebhookVerifier  # noqa: E402
from hooksign.cache import ReplayCache  # noqa: E402


def build_verifier(secret_text: str, db_path: str) -> WebhookVerifier:
    ring = SecretRing([Secret(id="primary", key=secret_text.encode())])
    return WebhookVerifier(
        ring, tolerance_s=300, replay_cache=ReplayCache(ttl_s=300)
    )


# 规范第 7 节的原因码 -> HTTP 状态码映射。
REASON_HTTP = {
    "missing_header": 401,
    "malformed_header": 400,
    "timestamp_skew": 401,
    "bad_signature": 401,
}


class Handler(BaseHTTPRequestHandler):
    verifier: WebhookVerifier
    store: DeliveryStore

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length)
        request = SignedRequest(
            method="POST",
            path=self.path,
            headers={k: v for k, v in self.headers.items()},
            body=body,
        )
        result = self.verifier.verify(request)
        if not result.ok:
            if result.reason == "replay":
                # 窗口内重复投递：幂等命中，按成功处理但不再执行业务。
                self._reply(200, {"ok": True, "reason": "replay", "duplicate": True})
            else:
                code = REASON_HTTP.get(result.reason, 401)
                self._reply(code, {"ok": False, "reason": result.reason})
            return

        try:
            payload = json.loads(body.decode("utf-8"))
            event_id = str(payload["id"])
        except (UnicodeDecodeError, ValueError, KeyError):
            self._reply(400, {"ok": False, "reason": "bad_payload"})
            return

        fresh = self.store.record(event_id)
        self._reply(202 if fresh else 200, {"ok": True, "duplicate": not fresh})

    def _reply(self, code: int, payload: dict) -> None:
        data = json.dumps(payload).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("[receiver] " + fmt % args + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="hooksign 示例接收端")
    parser.add_argument("--secret-file", default=".secret")
    parser.add_argument("--db", default=":memory:")
    parser.add_argument("--port", type=int, default=8080)
    args = parser.parse_args()

    secret_text = Path(args.secret_file).read_text(encoding="utf-8").strip()
    Handler.verifier = build_verifier(secret_text, args.db)
    Handler.store = DeliveryStore(args.db)

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"listening on http://127.0.0.1:{args.port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
