# Peer verification

This document is the specification for the client and server TLS
configurations produced by `tlspool`. It describes observable handshake
behaviour, not an implementation.

## §1 Client configuration

§1.1 `BuildClientTLS` produces a client-side `tls.Config` that verifies the
peer certificate chain against the configured `RootCAs`.

§1.2 Verification is on by default. A caller that supplies only `RootCAs` gets
full verification: chain building to a trusted authority, validity-period
checks, and hostname verification against `ServerName` (or the dial address
when `ServerName` is empty).

§1.3 Verification is relaxed only when the caller explicitly opts in with
`AllowSelfSigned`. In that mode peers that do not chain to a known authority
are accepted, which is intended for local development.

§1.4 A peer presenting a self-signed certificate, or a certificate from an
authority absent from `RootCAs`, must make the handshake fail in the default
mode.

§1.5 Legacy fields that are no longer part of the policy must not weaken
verification. In particular, an unset optional field must never be interpreted
as "skip verification".

§1.6 `MinVersion` is TLS 1.2 or later.

## §2 Server configuration

§2.1 `BuildServerTLS` produces a server-side `tls.Config` that requires and
verifies client certificates against `ClientCAs` (`tls.RequireAndVerifyClientCert`).

§2.2 When `PeerIdentities` is non-empty, the peer leaf must be valid for at
least one of the listed DNS names. Its subject alternative names are the
source of truth.

§2.3 When `PeerIdentities` is empty, any client that chains to `ClientCAs` is
accepted; no name check is applied.

§2.4 A client certificate issued by an authority outside `ClientCAs`, or a
client that presents no certificate, makes the handshake fail.

## §3 Shared pool

§3.1 Client and server configurations built from the same pool agree on which
authorities are trusted. A certificate accepted by one side is accepted by the
other when the name requirements of §1.2 and §2.2 are met.

§3.2 Verification results must not depend on process-wide state; every call to
`BuildClientTLS` or `BuildServerTLS` is self-contained.
