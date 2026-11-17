"""Header value parsing for multipart parts."""

from __future__ import annotations

from collections.abc import Mapping


def parse_header(value: str) -> tuple[str, dict[str, str]]:
    """Split a header value into its main value and its parameters.

    Parameter names are compared case-insensitively.  The main value is
    returned without surrounding whitespace.
    """

    pieces = value.split(";")
    main = pieces[0].strip()
    params: dict[str, str] = {}

    for piece in pieces[1:]:
        if "=" not in piece:
            continue
        name, _, raw_value = piece.partition("=")
        name = name.strip().lower()
        if not name:
            continue
        parameter = raw_value.strip()
        if (
            len(parameter) >= 2
            and parameter[0] == '"'
            and parameter[-1] == '"'
        ):
            parameter = parameter[1:-1]
        params[name] = parameter

    return main, params


def resolve_filename(params: Mapping[str, str]) -> str | None:
    """Return the filename carried by a Content-Disposition parameter table."""

    return params.get("filename")
