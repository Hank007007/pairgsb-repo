"""Request dispatch: frontend request -> command frames -> result code."""

from __future__ import annotations

import logging

from ..cmd.build import build_fetch, build_search, build_store
from ..errors import BadRequest, CommandTooLong, ImapQuillError, ProtocolError

log = logging.getLogger("imapquill.handler")


class DispatchResult(dict):
    """Result returned to the frontend.

    ``code`` describes the outcome of the request.
    """


def dispatch(request: dict, connection) -> DispatchResult:
    """Translate one frontend request, send it, and report the outcome."""
    operation = request.get("op") if isinstance(request, dict) else None
    folder = request.get("folder") if isinstance(request, dict) else None
    context = "op=%r folder=%r" % (operation, folder)
    try:
        if not isinstance(request, dict):
            raise BadRequest("request must be an object")
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
            raise BadRequest("unknown operation: %r" % (operation,))
    except BadRequest as exc:
        log.warning("local failure [bad_request]: %s: %s", context, exc)
        return DispatchResult(code="bad_request", error=str(exc))
    except (ProtocolError, CommandTooLong) as exc:
        log.warning("local failure [protocol_error]: %s: %s", context, exc)
        return DispatchResult(code="protocol_error", error=str(exc))
    except ImapQuillError as exc:
        log.warning("local failure [protocol_error]: %s: %s", context, exc)
        return DispatchResult(code="protocol_error", error=str(exc))

    replies = []
    for command in frames:
        reply = connection.send(command)
        replies.append(reply)
        status = reply.get("status", "").upper()
        if status in ("NO", "BAD"):
            code = "upstream_no" if status == "NO" else "upstream_bad"
            log.warning(
                "upstream rejection [%s]: %s tag=%r: %s",
                code,
                context,
                reply.get("tag", ""),
                reply.get("text", ""),
            )
            return DispatchResult(
                code=code,
                error=reply.get("text", ""),
                replies=replies,
                commands=frames,
            )
    return DispatchResult(code="ok", replies=replies, commands=frames)
