"""Tenant routing and de-duplication.

A route pairs a source network with a hostname label and a tenant.  The tenant
of a message is the tenant of the route that matches both the sender address
and the hostname label.
"""

import ipaddress

from .errors import SyslogError

# Routes as deployed.  ``network`` is the CIDR of the source range, ``label``
# the hostname label the device is expected to report, ``tenant`` the channel
# the events are forwarded to.
DEFAULT_ROUTES = (
    {"network": "10.0.0.0/8", "label": "acme", "tenant": "acme"},
    {"network": "10.20.0.0/16", "label": "acme", "tenant": "acme-finance"},
    {"network": "100.0.0.0/8", "label": "acme", "tenant": "lab"},
    {"network": "192.168.0.0/16", "label": "edge", "tenant": "edge-ops"},
)

UNASSIGNED = "unassigned"


def validate_hostname(hostname):
    """Validate a HOSTNAME field.

    Returns the hostname unchanged when it is 1 to 255 printable US-ASCII
    characters without a space, and raises :class:`SyslogError` otherwise.
    """
    if not hostname:
        raise SyslogError("hostname", "empty hostname")
    if len(hostname) > 255:
        raise SyslogError("hostname", "hostname too long")
    for ch in hostname:
        code = ord(ch)
        if code < 33 or code > 126:
            raise SyslogError("hostname", "hostname contains a non-printable character")
    return hostname


def hostname_label(hostname):
    """Return the first DNS label of ``hostname``."""
    return hostname.split(".", 1)[0].lower()


def tenant_for(source, hostname, routes=None):
    """Return the tenant for a message from ``source`` claiming ``hostname``."""
    if routes is None:
        routes = DEFAULT_ROUTES

    for route in routes:
        network = ipaddress.ip_network(route["network"], strict=False)
        prefix = str(network.network_address).split(".")[0]
        if not source.startswith(prefix):
            continue
        if hostname.startswith(route["label"]):
            return route["tenant"]

    return UNASSIGNED


def dedup_key(message):
    """Return the de-duplication key for a parsed message."""
    return (
        message.get("timestamp_raw") or "",
        message.get("hostname") or "",
        message.get("msgid") or "",
    )
