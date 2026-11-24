"""Replay captured messages through syslogfold.

Usage::

    python examples/replay.py [fixtures/messages]

For every ``*.log`` file the first line is treated as one wire message.  The
sender address is taken from ``SOURCES`` below.  Each message is parsed, its
fields are printed, then the CEF line produced by the pipeline is printed.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="backslashreplace")
except (AttributeError, ValueError):
    pass

from syslogfold.pipeline import ingest  # noqa: E402

FIXTURE_DIR = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "fixtures",
    "messages",
)

SOURCES = {
    "acme-login.log": "10.20.3.9",
    "edge-sshd.log": "192.168.4.17",
    "lab-search.log": "100.64.0.5",
    "acme-request.log": "10.99.1.1",
    "acme-unicode.log": "10.99.1.2",
}

FIELDS = (
    "facility",
    "severity",
    "version",
    "timestamp",
    "hostname",
    "app_name",
    "procid",
    "msgid",
    "structured_data",
    "msg",
)


def load_messages(directory):
    names = sorted(n for n in os.listdir(directory) if n.endswith(".log"))
    for name in names:
        path = os.path.join(directory, name)
        with open(path, "r", encoding="utf-8") as handle:
            line = handle.readline().rstrip("\r\n")
        yield name, SOURCES.get(name, "127.0.0.1"), line


def main(argv):
    directory = argv[1] if len(argv) > 1 else FIXTURE_DIR
    for name, source, line in load_messages(directory):
        print(f"=== {name}  source={source}")
        result = ingest(line, source=source)
        if "message" in result:
            message = result["message"]
            for field in FIELDS:
                print(f"  {field}: {message.get(field)!r}")
        if "error" in result:
            print(f"  error: {result['error']}")
        print(f"  tenant: {result.get('tenant')}")
        print(f"  cef: {result.get('cef')}")
        print()


if __name__ == "__main__":
    main(sys.argv)
