"""Value objects shared by the cache and the token client."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Tuple


@dataclass(frozen=True)
class TokenEntry:
    """A cached access token together with what it is good for.

    ``scopes`` is the canonical set of scopes the issuer actually granted for
    this token.  ``expires_at`` is the instant at which the token stops being
    usable, expressed as a UTC wall-clock timestamp.
    """

    access_token: str
    token_type: str
    scopes: Tuple[str, ...]
    expires_at: datetime
