"""Read an HTML fragment from stdin and print the sanitized result."""

import sys

from . import sanitize


def main():
    data = sys.stdin.buffer.read().decode("utf-8")
    sys.stdout.buffer.write(sanitize(data).encode("utf-8"))
    sys.stdout.buffer.write(b"\n")


if __name__ == "__main__":
    main()
