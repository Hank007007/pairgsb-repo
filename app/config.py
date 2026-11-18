"""Runtime configuration for the token service."""

from __future__ import annotations

import os
from dataclasses import dataclass

DEFAULT_SECRET = "grantshift-local-development-secret"
DEFAULT_ISSUER = "https://grantshift.internal"
DEFAULT_DB_PATH = "grantshift.db"

ACCESS_TOKEN_TTL = 900
REFRESH_TOKEN_TTL = 86_400
AUTHORIZATION_CODE_TTL = 300


@dataclass(frozen=True)
class Settings:
    """Everything the service needs to run.

    Values come from the environment when the app is created, so a deployment
    can point the service at a different database or secret without editing
    code.
    """

    secret: str = DEFAULT_SECRET
    issuer: str = DEFAULT_ISSUER
    db_path: str = DEFAULT_DB_PATH
    access_token_ttl: int = ACCESS_TOKEN_TTL
    refresh_token_ttl: int = REFRESH_TOKEN_TTL
    code_ttl: int = AUTHORIZATION_CODE_TTL

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            secret=os.environ.get("GRANTSHIFT_SECRET", DEFAULT_SECRET),
            issuer=os.environ.get("GRANTSHIFT_ISSUER", DEFAULT_ISSUER),
            db_path=os.environ.get("GRANTSHIFT_DB", DEFAULT_DB_PATH),
            access_token_ttl=int(
                os.environ.get("GRANTSHIFT_ACCESS_TTL", ACCESS_TOKEN_TTL)
            ),
            refresh_token_ttl=int(
                os.environ.get("GRANTSHIFT_REFRESH_TTL", REFRESH_TOKEN_TTL)
            ),
            code_ttl=int(os.environ.get("GRANTSHIFT_CODE_TTL", AUTHORIZATION_CODE_TTL)),
        )
