"""Exception hierarchy for the manifest loader."""


class ManifestError(Exception):
    """Base class for every error raised while loading a manifest."""


class ParseError(ManifestError):
    """The manifest text does not conform to the supported syntax."""

    def __init__(self, message: str, line: int | None = None) -> None:
        self.line = line
        if line is not None:
            message = f"line {line}: {message}"
        super().__init__(message)


class DuplicateKeyError(ParseError):
    """The same mapping declares the same key more than once."""

    def __init__(self, key: str, line: int | None = None) -> None:
        self.key = key
        super().__init__(f"duplicate key {key!r}", line)


class AliasError(ManifestError):
    """An alias cannot be resolved."""

    def __init__(self, message: str, line: int | None = None) -> None:
        self.line = line
        if line is not None:
            message = f"line {line}: {message}"
        super().__init__(message)


class AliasLimitError(AliasError):
    """Expanding aliases exceeded the configured node budget."""


class MergeError(ManifestError):
    """A merge key has an operand that cannot be merged."""

    def __init__(self, message: str, line: int | None = None) -> None:
        self.line = line
        if line is not None:
            message = f"line {line}: {message}"
        super().__init__(message)


class UnknownTagError(ManifestError):
    """A tag was used that is not present in the tag registry."""

    def __init__(self, tag: str, line: int | None = None) -> None:
        self.tag = tag
        self.line = line
        if line is not None:
            message = f"line {line}: unknown tag {tag!r}"
        else:
            message = f"unknown tag {tag!r}"
        super().__init__(message)


class TagError(ManifestError):
    """A registered tag handler failed or was used incorrectly."""
