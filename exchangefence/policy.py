"""Deployment policy shared by the service and token layers."""

TOKEN_EXCHANGE_GRANT = "urn:ietf:params:oauth:grant-type:token-exchange"

ISSUER = "https://auth.internal.exchangefence"

# Key used to sign the compact tokens this service issues.
TOKEN_SECRET = b"exchangefence-internal-signing-key"

# Upper bound on the lifetime of any token minted by this service.
MAX_TTL_SECONDS = 900

# Lifetime used when a caller does not ask for a specific one.
DEFAULT_TTL_SECONDS = 300

# Access tokens produced by this service.
ACCESS_TOKEN_TYPE = "urn:ietf:params:oauth:token-type:access_token"

# Token types accepted in subject_token / actor_token.
ALLOWED_TOKEN_TYPES = frozenset(
    {
        "urn:ietf:params:oauth:token-type:access_token",
        "urn:ietf:params:oauth:token-type:jwt",
    }
)

# The act chain may carry at most this many delegation levels.
MAX_ACT_DEPTH = 3
