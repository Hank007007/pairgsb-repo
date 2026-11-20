"""Shared fixtures for the scopevault test suite."""

from __future__ import annotations

import threading
from typing import Dict, List

import pytest

from scopevault.client import TokenClient


class FakeTransport:
    """Records every token endpoint call and answers with a canned payload."""

    def __init__(self, expire_in=3600, token_type="Bearer", scope=None):
        self.calls: List[Dict[str, str]] = []
        self._lock = threading.Lock()
        self._expires_in = expire_in
        self._token_type = token_type
        self._scope = scope
        self._counter = 0

    def post(self, url: str, form) -> dict:
        with self._lock:
            self.calls.append(dict(form, url=url))
            self._counter += 1
            index = self._counter
        payload = {
            "access_token": f"tok-{index}",
            "token_type": self._token_type,
            "expires_in": self._expires_in,
        }
        if self._scope is not None:
            payload["scope"] = self._scope
        return payload


@pytest.fixture
def transport() -> FakeTransport:
    return FakeTransport()


@pytest.fixture
def client(transport: FakeTransport) -> TokenClient:
    return TokenClient(transport=transport)
