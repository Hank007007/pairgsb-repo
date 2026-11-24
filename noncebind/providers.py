"""Provider registry for the noncebind OIDC login component."""

from __future__ import annotations

from dataclasses import dataclass


class UnknownProviderError(KeyError):
    """Raised when a provider name is not present in the registry."""


@dataclass(frozen=True)
class ProviderConfig:
    """Static configuration for one identity provider."""

    name: str
    issuer: str
    client_id: str
    client_secret: str
    audience: str
    algorithms: tuple[str, ...]
    authorization_endpoint: str
    token_endpoint: str
    jwks_uri: str
    redirect_uri: str
    scope: str = "openid profile email"
    clock_skew_seconds: int = 120


_PROVIDERS: dict[str, ProviderConfig] = {
    "okta-dev": ProviderConfig(
        name="okta-dev",
        issuer="https://idp.example.com/oauth2/default",
        client_id="noncebind-console",
        client_secret="okta-dev-client-secret",
        audience="noncebind-console",
        algorithms=("RS256",),
        authorization_endpoint="https://idp.example.com/oauth2/default/v1/authorize",
        token_endpoint="https://idp.example.com/oauth2/default/v1/token",
        jwks_uri="https://idp.example.com/oauth2/default/v1/keys",
        redirect_uri="https://console.example.com/oidc/callback",
    ),
    "azure-ad": ProviderConfig(
        name="azure-ad",
        issuer="https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47/v2.0",
        client_id="noncebind-portal",
        client_secret="azure-ad-client-secret",
        audience="noncebind-portal",
        algorithms=("RS256",),
        authorization_endpoint=(
            "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47"
            "/oauth2/v2.0/authorize"
        ),
        token_endpoint=(
            "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47"
            "/oauth2/v2.0/token"
        ),
        jwks_uri=(
            "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47"
            "/discovery/v2.0/keys"
        ),
        redirect_uri="https://portal.example.com/auth/callback",
    ),
    "keycloak-eu": ProviderConfig(
        name="keycloak-eu",
        issuer="https://sso.eu.example.com/realms/internal",
        client_id="noncebind-ops",
        client_secret="keycloak-eu-client-secret",
        audience="noncebind-ops",
        algorithms=("RS256", "ES256"),
        authorization_endpoint=(
            "https://sso.eu.example.com/realms/internal/protocol/openid-connect/auth"
        ),
        token_endpoint=(
            "https://sso.eu.example.com/realms/internal/protocol/openid-connect/token"
        ),
        jwks_uri=(
            "https://sso.eu.example.com/realms/internal/protocol/openid-connect/certs"
        ),
        redirect_uri="https://ops.example.com/session/callback",
    ),
    "auth0-partner": ProviderConfig(
        name="auth0-partner",
        issuer="https://partner.eu.auth0.com/",
        client_id="noncebind-partner",
        client_secret="auth0-partner-client-secret",
        audience="noncebind-partner",
        algorithms=("ES256",),
        authorization_endpoint="https://partner.eu.auth0.com/authorize",
        token_endpoint="https://partner.eu.auth0.com/oauth/token",
        jwks_uri="https://partner.eu.auth0.com/.well-known/jwks.json",
        redirect_uri="https://partners.example.com/login/return",
    ),
}


def get_provider(name: str) -> ProviderConfig:
    """Return the configuration for ``name``.

    Raises:
        UnknownProviderError: if no provider is registered under that name.
    """

    try:
        return _PROVIDERS[name]
    except KeyError as exc:
        raise UnknownProviderError(name) from exc


def provider_names() -> tuple[str, ...]:
    """Return the registered provider names in a stable order."""

    return tuple(_PROVIDERS)
