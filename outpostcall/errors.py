"""Exception hierarchy for outbound delivery."""


class OutpostError(Exception):
    """Base class for every error raised by this package."""


class ConfigError(OutpostError):
    """An integration or policy is not usable as configured."""


class URLBuildError(OutpostError):
    """A base URL or path template could not be rendered safely."""


class QueryError(OutpostError):
    """A query component could not be assembled."""


class HeaderError(OutpostError):
    """A request header set is not transmittable."""


class SignatureError(OutpostError):
    """A request cannot be signed or its covered data changed after signing."""


class EgressError(OutpostError):
    """A destination is not permitted by the egress policy."""


class RedirectError(EgressError):
    """A redirect could not be followed."""


class TransportError(OutpostError):
    """The request could not be completed by the transport."""


class RetryLimitExceeded(OutpostError):
    """The retry budget for a delivery was exhausted."""