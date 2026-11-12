"""给示例接收端发一条带签名的请求。

    python examples/sender.py --secret-file .secret --event-id evt_1
    python examples/sender.py --body-file payload.bin      # 二进制正文
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from hooksign import Secret  # noqa: E402
from hooksign.signing import sign_header  # noqa: E402


def main() -> int:
    parser = argparse.ArgumentParser(description="hooksign 示例发送端")
    parser.add_argument("--secret-file", default=".secret")
    parser.add_argument("--url", default="http://127.0.0.1:8080/webhooks/orders")
    parser.add_argument("--event-id", default="evt_0001")
    parser.add_argument("--body-file", default=None)
    parser.add_argument("--repeat", type=int, default=1)
    args = parser.parse_args()

    secret_text = Path(args.secret_file).read_text(encoding="utf-8").strip()
    secret = Secret(id="primary", key=secret_text.encode())
    path = urllib.parse.urlsplit(args.url).path

    if args.body_file:
        body = Path(args.body_file).read_bytes()
    else:
        body = json.dumps({"id": args.event_id, "type": "order.paid"}).encode("utf-8")

    headers = sign_header(secret, method="POST", path=path, body=body)
    headers["Content-Type"] = "application/json"

    for attempt in range(1, args.repeat + 1):
        request = urllib.request.Request(
            args.url, data=body, headers=headers, method="POST"
        )
        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                print(f"[{attempt}] HTTP {response.status} {response.read().decode()}")
        except urllib.error.HTTPError as exc:
            print(f"[{attempt}] HTTP {exc.code} {exc.read().decode()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
