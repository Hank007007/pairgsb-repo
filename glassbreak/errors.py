"""Exception hierarchy used across glassbreak."""

from __future__ import annotations


class GlassbreakError(Exception):
    """Base class for every domain error raised by the service."""


class InvalidRequest(GlassbreakError):
    """The submitted request is malformed."""


class UnknownPrincipal(GlassbreakError):
    """The principal is not present in the group policy."""


class RequestNotFound(GlassbreakError):
    """No request with the supplied identifier exists."""


class RequestNotPending(GlassbreakError):
    """The request already moved out of the pending state."""


class RequestExpired(GlassbreakError):
    """The approval window for the request has closed."""


class VoteRejected(GlassbreakError):
    """A vote was refused and therefore not counted."""


class SelfApprovalNotAllowed(VoteRejected):
    """The subject of a request cannot approve it."""


class ApproverOutsideGroup(VoteRejected):
    """The voter is not a member of the subject's group."""


class NamedApproverCannotApprove(VoteRejected):
    """The voter is the approver named by the subject."""


class DuplicateVoteNotAllowed(VoteRejected):
    """The same approver cannot cast more than one vote for a request."""


class CredentialNotAvailable(GlassbreakError):
    """No credential can be issued for the request in its current state."""
