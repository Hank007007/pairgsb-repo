"""ASGI entry point for deployments: ``uvicorn app.asgi:app``."""

from __future__ import annotations

from .config import Settings
from .main import create_app

app = create_app(Settings.from_env())
