"""A scripted upstream connection used by the example replay and by tests.

It records every command it is asked to write and answers with canned reply
lines.  It is not a real IMAP client.
"""

from __future__ import annotations


class ScriptedConnection:
    """Answer commands from a list of ``"OK ..."`` / ``"NO ..."`` lines."""

    def __init__(self, scripted: list[str] | None = None) -> None:
        self._scripted = list(scripted or [])
        self.written: list[str] = []

    def send(self, command: str) -> dict:
        self.written.append(command)
        if self._scripted:
            line = self._scripted.pop(0)
        else:
            line = "OK completed"
        tag = command.split(" ", 1)[0] if command else ""
        status, _, text = line.partition(" ")
        return {"tag": tag, "status": status, "text": text}
