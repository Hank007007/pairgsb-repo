# egressvault — design notes

This document describes the behaviour egressvault is expected to exhibit. It is
written from the outside in: what a caller can observe, what the network team
can observe, and what an auditor can observe. `docs/egress-rules.md` is
normative; this document explains the shape of the system that satisfies it.

## 1. Position in the network

```
business service ──HTTP──▶ egressvault ──TLS──▶ upstream API
                              │
                              ├─ vault.json   (credential material, read-only)
                              └─ audit.jsonl  (one record per hop)
```

The business service is configured with `HTTP_PROXY`/`HTTPS_PROXY` pointing at
egressvault, so it sends ordinary absolute-form HTTP requests and never learns
that a credential exists. egressvault is the only process that holds secret
material, and it holds it only for the duration of a hop.

egressvault does **not** terminate TLS for the caller: the caller speaks plain
HTTP to the proxy, and the proxy opens its own TLS connection to the upstream.
This is what keeps credentials out of the business process entirely, and it is
also why upstream certificate validation (§5 of the rules) is the only thing
standing between the credential and an impersonated upstream.

## 2. Request lifecycle

A request passes through the following stages. Each stage has one owner module,
and a stage is expected to be able to say "no" without any later stage undoing
that decision.

1. **Admission** (`src/proxy.js`). Parse the absolute request target. Only
   `http` and `https` targets are accepted; anything else is refused before any
   routing, credential or network work happens. The caller's body is buffered
   up to a fixed limit.
2. **Hop construction** (`src/proxy.js`). Turn the target into a hop descriptor:
   normalised host, port, scheme, path, method, caller headers, body.
3. **Routing** (`src/routing.js`). Decide which rule, if any, governs the hop.
   This is a pure function of the hop host and the rule set.
4. **Credential selection and injection** (`src/vault.js`, `src/proxy.js`).
   Fetch the credential by key id, and write the authentication header plus the
   credential's extra headers *over* whatever the caller sent under those names.
5. **Transport** (`src/tls.js`, `src/proxy.js`). Choose direct vs. upstream-proxy
   routing, open the connection with certificate verification on, present the
   client certificate belonging to the selected credential when the credential
   declares one.
6. **Observation** (`src/audit.js`). Emit exactly one audit record for the hop.
7. **Redirect handling** (`src/redirect.js`). If the response is a followable
   3xx, construct the next hop and go back to stage 2. Otherwise relay.

## 3. The hop descriptor is the security boundary

Every hop is described by the same small record: host, port, scheme, path,
method, headers, body, and the credential chosen for that hop. Two properties
follow from this and should hold in every path through the code:

* **Credential identity is per hop.** It is recomputed for each hop and never
  carried forward. A redirect is not a "continuation of the same authorised
  request"; it is a new request to a possibly new principal.
* **Header provenance matters.** Headers in a hop come from exactly one of
  three sources — the caller, the credential for *this* hop, or the transport
  itself. When a hop is derived from a previous hop, the derivation must be
  explicit about which sources survive. In particular, a header that came from
  a credential is only valid for the host that credential was selected for.

## 4. Redirects

Redirects are the most common way a credential leaves the perimeter, so the
design treats them conservatively:

* A redirect target is a brand-new hop. It is routed from scratch, validated
  for TLS from scratch, and audited as its own record.
* The only thing a redirect may inherit is caller intent that is meaningful
  across hosts: the method and body, and even those only when the semantics of
  the status code say so.
* Whether the previous and next hop are "the same host" is decided on the
  normalised host plus port plus scheme — not on the string in `Location` and
  not on a prefix comparison.
* The chain is bounded. A caller sees a single final response; intermediate
  redirect responses and their `Set-Cookie` are not passed through.

## 5. Host matching

Host matching is exact-label matching. Operationally:

* The comparison key is the host with case folded, the trailing dot removed,
  and IPv6 brackets removed.
* A rule matches its own host and *single-label* children of it. It does not
  match strings that merely end with the rule host, strings that contain the
  rule host in the middle, or hosts the rule host is a suffix of in the
  character sense.
* The most specific matching rule wins, so an operator can carve out a
  subdomain with a different credential without ambiguity.
* A host with no matching rule is still proxied; it simply carries no
  credential. This is a deliberate choice: egressvault is a credential
  injector, not an allow-list firewall, and the allow-list lives in the
  network layer.

The same matching primitive is reused for `NO_PROXY`, because a network team
that writes `*.corp.internal` in one place and `corp.internal` in another
should not have to reason about two different notions of "matches".

## 6. TLS posture

* Upstream verification is unconditional. There is no configuration surface that
  turns it off, and no environment variable is consulted for it.
* The trust store is additive: the platform store plus an optional fleet CA
  file. Nothing removes trust anchors.
* The peer is checked against the host of the hop, and SNI carries that host.
* A client certificate is bound to the credential that selected it. A connection
  is never shared between two credentials or two hosts.
* Verification failures surface to the caller as a generic upstream error. The
  OpenSSL reason is a diagnostic for operators, and it never carries request or
  credential material because it is produced before any request bytes exist.

## 7. Audit

The audit log answers "which principal talked to which host, when, and how did
it go". It is deliberately poor at answering anything else. The record shape is
a fixed set of named fields; nothing is copied wholesale from a request object.

Redaction of the query string is a good example of the intended posture: the
path is useful for debugging, individual query values are not, and the cost of
being wrong is asymmetric. The same reasoning applies to headers — a header
*map* is exactly the thing that contains the injected credential, so the record
shape does not have a place to put one.

## 8. Failure modes

Errors reach the caller as a code, never as a message, stack or upstream text.
The distinction that matters operationally:

| caller sees | operator should look at |
|---|---|
| `EUPSTREAMTLS` | the upstream certificate chain and the trust store |
| `EUPSTREAM` | the upstream's availability, or an upstream proxy refusing |
| `ELOOP` | a redirecting upstream that never settles |
| `EBADREDIRECT` | an upstream emitting a `Location` that is not http(s) |
| `ETIMEOUT` | latency between egressvault and the upstream |

Every one of these also produces an audit record with a `null` status and the
error code, so a batch of failing requests is visible without enabling debug
logging.

## 9. Configuration

| variable | default | meaning |
|---|---|---|
| `EGRESS_VAULT_FILE` | — (required) | vault JSON |
| `EGRESS_AUDIT_FILE` | unset (no file) | audit JSONL destination |
| `EGRESS_MAX_REDIRECTS` | `5` | redirect chain bound |
| `EGRESS_CA_FILE` | unset | additional trust anchors |
| `EGRESS_BIND` | `127.0.0.1` | listen address |
| `EGRESS_PORT` | `0` | listen port |
| `EGRESS_TIMEOUT_MS` | `30000` | per-hop upstream timeout |
| `HTTP_PROXY`/`http_proxy`/`ALL_PROXY` | unset | upstream proxy for http hops |
| `HTTPS_PROXY`/`https_proxy`/`ALL_PROXY` | unset | upstream proxy for https hops |
| `NO_PROXY`/`no_proxy` | unset | hosts that bypass the upstream proxy |

## 10. Testing strategy

`node --test` spins up local `http` and `https` servers on loopback and points
the proxy at them; nothing reaches the public internet. The fixture
certificates under `test/fixtures/` are signed by a fixture CA that is also
under `test/fixtures/`, so the suite can exercise a verified TLS hop without
touching the machine's trust store. A fixture forwarding proxy stands in for a
corporate upstream proxy and records the absolute-form targets it is asked to
fetch.

Tests should be able to observe, from the upstream's side, exactly what the
proxy put on the wire: that is the only place where the answer to "was the
credential injected, and was it stripped" is unambiguous.
