"""hooksign —— webhook 请求签名校验。"""

from .headers import HEADER_NAME, ParsedSignature, parse_signature_header
from .secrets import Secret, SecretRing
from .verify import SignedRequest, VerificationResult, WebhookVerifier

__all__ = [
    "HEADER_NAME",
    "ParsedSignature",
    "Secret",
    "SecretRing",
    "SignedRequest",
    "VerificationResult",
    "WebhookVerifier",
    "parse_signature_header",
]

__version__ = "0.4.0"
