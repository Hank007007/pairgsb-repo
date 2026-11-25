"""HTTP-facing entry point for policy uploads.

``handle_upload`` is the function the upload route calls once the request body
has been read.  It returns the JSON body the route will send back, together
with the status the route should use.
"""

import yaml

from .intake.loader import load_document
from .policy.compile import compile_policy

OK = 200
BAD_REQUEST = 400


def handle_upload(body):
    """Process one uploaded policy document.

    ``body`` is the raw text of the request body.  The return value is a dict
    with the keys ``ok``, ``status``, ``policy`` and ``error``; ``policy`` is
    ``None`` when ``ok`` is false and ``error`` is ``None`` otherwise.
    """
    if not isinstance(body, str):
        raise TypeError("upload body must be text")
    try:
        document = load_document(body)
        policy = compile_policy(document)
    except yaml.YAMLError as exc:
        return _failure(exc)
    return {"ok": True, "status": OK, "policy": policy.to_dict(), "error": None}


def _failure(exc):
    mark = getattr(exc, "problem_mark", None)
    line = mark.line + 1 if mark is not None else None
    return {
        "ok": False,
        "status": BAD_REQUEST,
        "policy": None,
        "error": {
            "code": "invalid_yaml",
            "line": line,
            "message": str(exc),
        },
    }
