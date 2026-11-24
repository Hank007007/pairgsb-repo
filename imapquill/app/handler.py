"""Request dispatch: frontend request -> command frames -> result code."""

from __future__ import annotations

import logging

from ..cmd.build import build_fetch, build_search, build_store

log = logging.getLogger("imapquill.handler")


class DispatchResult(dict):
    """Result returned to the frontend.

    ``code`` describes the outcome of the request.
    """


def dispatch(request: dict, connection) -> DispatchResult:
    """Translate one frontend request, send it, and report the outcome."""
    operation = request.get("op")
    try:
        if operation == "search":
            frames = build_search(
                request.get("folder", "INBOX"),
                request.get("criteria", {}),
                tag=request.get("tag", "A1"),
            )
        elif operation == "fetch":
            frames = build_fetch(
                request.get("folder", "INBOX"),
                request.get("uids", []),
                section=request.get("section", ""),
                tag=request.get("tag", "A2"),
            )
        elif operation == "store":
            frames = build_store(
                request.get("folder", "INBOX"),
                request.get("uids", []),
                request.get("flags", []),
                tag=request.get("tag", "A3"),
            )
        else:
            raise ValueError("unknown operation: %r" % (operation,))
    except Exception as exc:  # noqa: BLE001
        log.warning("request failed: %s", exc)
        return DispatchResult(code="bad_request", error=str(exc))

    replies = []
    for command in frames:
        reply = connection.send(command)
        replies.append(reply)
        status = reply.get("status", "").upper()
        if status in ("NO", "BAD"):
            log.warning("request failed: %s", reply.get("text", ""))
            return DispatchResult(
                code="bad_request",
                error=reply.get("text", ""),
                replies=replies,
                commands=frames,
            )
    return DispatchResult(code="ok", replies=replies, commands=frames)
