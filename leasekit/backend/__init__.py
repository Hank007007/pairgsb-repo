"""存储后端。"""

from .base import Backend
from .redis import RedisBackend

__all__ = ["Backend", "RedisBackend"]
