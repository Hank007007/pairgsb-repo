"""Token grammar, parsing, and construction.

A token is ``tg_<id8>_<secret>``.  ``id8`` is the public prefix; ``secret`` is
key material and is never persisted in clear text.
"""

from __future__ import annotations

import re
import secrets
from dataclasses import dataclass

PREFIX = "tg"
ID_LEN = 8
SECRET_LEN = 64
TOKEN_LEN = len(PREFIX) + 1 + ID_LEN + 1 + SECRET_LEN

_TOKEN_RE = re.compile(
    r"\A"
    + re.escape(PREFIX)
    + r"_([0-9a-f]{"
    + str(ID_LEN)
    + r"})_([0-9a-f]{"
    + str(SECRET_LEN)
    + r"})\Z"
)


@dataclass(frozen=True)
class ParsedToken:
    """The two variable parts of a token, after a successful parse."""

    token_id: str
    secret: str

    @property
    def prefix(self) -> str:
        return self.token_id


def new_token_id() -> str:
    """Return a fresh public identifier."""
    return secrets.token_hex(ID_LEN // 2)


def new_secret() -> str:
    """Return fresh secret material."""
    return secrets.token_hex(SECRET_LEN // 2)


def new_family_id() -> str:
    """Return a fresh family identifier."""
    return secrets.token_hex(16)


def format_token(token_id: str, secret: str) -> str:
    """Join the public identifier and the secret into a wire token."""
    return f"{PREFIX}_{token_id}_{secret}"


def token_id_of(raw: object) -> str:
    """Return the public identifier carried by a token.

    A bare ``id8`` is accepted so callers may revoke by prefix.  A malformed
    value yields an empty string.
    """
    if isinstance(raw, str) and re.fullmatch(r"[0-9a-f]{" + str(ID_LEN) + r"}", raw):
        return raw
    parsed = parse_token(raw)
    return parsed.token_id if parsed is not None else ""


def parse_token(raw: object) -> ParsedToken | None:
    """Parse a wire token.

    Returns ``None`` when the value does not match the grammar in
    ``docs/token-format.md``.
    """
    if not isinstance(raw, str):
        return None
    match = _TOKEN_RE.match(raw)
    if match is None:
        return None
    return ParsedToken(token_id=match.group(1), secret=match.group(2))
