"""Error type shared by the intake and policy layers."""

__all__ = ["PolicyError"]


class PolicyError(Exception):
    """A rejection that can be reported to the caller as a code plus a location.

    ``code`` is the stable identifier returned to clients.  ``line`` is the
    1-based line in the submitted document, or ``None`` when the problem is
    not tied to a single line.
    """

    def __init__(self, code, message, line=None):
        super().__init__(message)
        self.code = code
        self.message = message
        self.line = line

    def to_dict(self):
        return {"code": self.code, "line": self.line, "message": self.message}

    def __str__(self):
        if self.line is None:
            return "%s: %s" % (self.code, self.message)
        return "%s (line %d): %s" % (self.code, self.line, self.message)
