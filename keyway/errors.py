"""Public error types used across the service.

Every error exposes a stable machine readable ``code``.  The HTTP layer is
expected to surface only the code (plus a key name when the request named
one) to callers.
"""


class KeywayError(Exception):
    """Base class for all errors the service reports deliberately."""

    code = "internal_error"
    status = 500

    def __init__(self, message: str = "", *, key_name: str | None = None):
        super().__init__(message)
        self.key_name = key_name


class InvalidRequest(KeywayError):
    code = "invalid_request"
    status = 400


class InvalidBlob(KeywayError):
    code = "invalid_blob"
    status = 400


class KeyNotFound(KeywayError):
    code = "key_not_found"
    status = 404


class KeyVersionGone(KeywayError):
    code = "key_version_gone"
    status = 410


class ContextMismatch(KeywayError):
    code = "context_mismatch"
    status = 400


class DecryptFailed(KeywayError):
    code = "decrypt_failed"
    status = 400


class BatchTooLarge(KeywayError):
    code = "batch_too_large"
    status = 413
