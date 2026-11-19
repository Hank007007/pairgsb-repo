"""A small Redis-shaped fake used to exercise store pluggability.

It mimics only the handful of commands the adapter needs.  Values are kept as
Python objects; a production adapter would serialize them.
"""

from __future__ import annotations

from dataclasses import replace

from sessforge.store import RememberToken, SessionRecord, SessionStore


class FakeRedis:
    """Minimal stand-in for a Redis client."""

    def __init__(self) -> None:
        self._data: dict[str, dict[str, object]] = {}

    def hset(self, key: str, field: str, value: object) -> None:
        self._data.setdefault(key, {})[field] = value

    def hget(self, key: str, field: str) -> object | None:
        return self._data.get(key, {}).get(field)

    def hdel(self, key: str, field: str) -> None:
        bucket = self._data.get(key)
        if bucket is not None:
            bucket.pop(field, None)
            if not bucket:
                self._data.pop(key, None)

    def hgetall(self, key: str) -> dict[str, object]:
        return dict(self._data.get(key, {}))

    def keys(self, pattern: str) -> list[str]:
        if pattern.endswith("*"):
            prefix = pattern[:-1]
            return [key for key in self._data if key.startswith(prefix)]
        return [key for key in self._data if key == pattern]


class RedisSessionStore(SessionStore):
    """A :class:`SessionStore` backed by :class:`FakeRedis`."""

    def __init__(self, client: FakeRedis) -> None:
        self._client = client

    def create_session(self, record: SessionRecord) -> None:
        key = f"sess:{record.sid}"
        if self._client.hget(key, "record") is not None:
            raise ValueError(f"session already exists: {record.sid}")
        self._client.hset(key, "record", record)

    def get_session(self, sid: str) -> SessionRecord | None:
        value = self._client.hget(f"sess:{sid}", "record")
        return value if isinstance(value, SessionRecord) else None

    def save_session(self, record: SessionRecord) -> None:
        self._client.hset(f"sess:{record.sid}", "record", record)

    def delete_session(self, sid: str) -> None:
        self._client.hdel(f"sess:{sid}", "record")

    def list_user_sessions(self, user_id: str) -> list[SessionRecord]:
        out: list[SessionRecord] = []
        for key in self._client.keys("sess:*"):
            value = self._client.hget(key, "record")
            if isinstance(value, SessionRecord) and value.user_id == user_id:
                out.append(value)
        out.sort(key=lambda item: (item.created_at, item.sid))
        return out

    def touch_session(self, sid: str, last_seen: float) -> None:
        record = self.get_session(sid)
        if record is not None:
            self._client.hset(f"sess:{sid}", "record", replace(record, last_seen=last_seen))

    def get_epoch(self, user_id: str) -> int:
        value = self._client.hget(f"epoch:{user_id}", "value")
        return int(value) if isinstance(value, int) else 0

    def set_epoch(self, user_id: str, epoch: int) -> None:
        self._client.hset(f"epoch:{user_id}", "value", int(epoch))

    def put_remember_token(self, token_hash: str, record: RememberToken) -> None:
        self._client.hset(f"rt:{token_hash}", "record", record)
        if record.family_id:
            self._client.hset(f"rt-family:{record.family_id}", token_hash, 1)

    def get_remember_token(self, token_hash: str) -> RememberToken | None:
        value = self._client.hget(f"rt:{token_hash}", "record")
        return value if isinstance(value, RememberToken) else None

    def delete_remember_token(self, token_hash: str) -> None:
        self._client.hdel(f"rt:{token_hash}", "record")

    def list_remember_tokens(self, user_id: str) -> list[RememberToken]:
        out: list[RememberToken] = []
        for key in self._client.keys("rt:*"):
            value = self._client.hget(key, "record")
            if isinstance(value, RememberToken) and value.user_id == user_id:
                out.append(value)
        return out

    def pop_remember_token(self, token_hash: str) -> RememberToken | None:
        key = f"rt:{token_hash}"
        value = self._client.hget(key, "record")
        if not isinstance(value, RememberToken):
            return None
        self._client.hdel(key, "record")
        self._client.hdel(f"rt-family:{value.family_id}", token_hash)
        return value

    def mark_remember_token_redeemed(self, token_hash: str, family_id: str) -> None:
        self._client.hset("rt:redeemed", token_hash, family_id)

    def get_redeemed_remember_family(self, token_hash: str) -> str | None:
        value = self._client.hget("rt:redeemed", token_hash)
        return value if isinstance(value, str) else None

    def invalidate_remember_family(self, family_id: str) -> None:
        members = self._client.hgetall(f"rt-family:{family_id}")
        for token_hash in list(members):
            self._client.hdel(f"rt:{token_hash}", "record")
            self._client.hdel(f"rt-family:{family_id}", token_hash)
