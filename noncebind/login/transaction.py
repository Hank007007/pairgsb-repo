"""One-time login transaction state."""

from __future__ import annotations

import secrets
import time
from dataclasses import dataclass, field
from typing import Callable

from ..providers import ProviderConfig


class TransactionError(ValueError):
    """Base class for login transaction errors."""


class UnknownTransactionError(TransactionError):
    """Raised when a state value does not identify a live transaction."""


@dataclass(frozen=True)
class LoginTransaction:
    """State created when a login is started and consumed on callback."""

    state: str
    nonce: str
    provider: str
    redirect_uri: str
    max_age: int | None
    created_at: float


def _new_token() -> str:
    return secrets.token_urlsafe(32)


def validate_redirect_uri(transaction: LoginTransaction, redirect_uri: str | None) -> bool:
    """Return whether ``redirect_uri`` matches the one bound to ``transaction``."""

    if not isinstance(redirect_uri, str):
        return False
    return redirect_uri == transaction.redirect_uri


@dataclass
class TransactionStore:
    """In-memory store of pending login transactions."""

    clock: Callable[[], float] = time.time
    _transactions: dict[str, LoginTransaction] = field(default_factory=dict)

    def issue(
        self,
        provider: ProviderConfig,
        redirect_uri: str,
        max_age: int | None = None,
        *,
        nonce: str | None = None,
        state: str | None = None,
        now: float | None = None,
    ) -> LoginTransaction:
        """Create a login transaction for one authorization request."""

        created_at = self.clock() if now is None else float(now)
        transaction = LoginTransaction(
            state=state or _new_token(),
            nonce=nonce or _new_token(),
            provider=provider.name,
            redirect_uri=redirect_uri,
            max_age=max_age,
            created_at=created_at,
        )
        self._transactions[transaction.state] = transaction
        return transaction

    def consume(self, state: str, *, now: float | None = None) -> LoginTransaction:
        """Consume and return the transaction identified by ``state``.

        A transaction is single-use: it is removed from the store on the first
        successful lookup so that no later callback can present it again.

        :Raises UnknownTransactionError: if no live transaction exists for
            ``state``, including a transaction that was already consumed.
        """

        transaction = self._transactions.pop(state, None)
        if transaction is None:
            raise UnknownTransactionError(state)
        return transaction

    def get(self, state: str) -> LoginTransaction | None:
        """Look up a pending transaction without consuming it."""

        return self._transactions.get(state)

    def __len__(self) -> int:
        return len(self._transactions)
