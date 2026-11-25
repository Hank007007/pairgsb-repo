# Design

This is the umbrella document for snisieve. It states what the system as a
whole has to guarantee, how the stages compose, and how a reviewer can tell a
correct build from an incorrect one. It deliberately contains no code and no
implementation guidance; `docs/record-layer.md`, `docs/hello.md` and
`docs/policy.md` are the normative references for each stage.

## 1. What the proxy promises

The allow-list is a statement about *the name the upstream will use*. snisieve
can only honour that statement if the name it extracts is the same name the
upstream will act on. Everything below follows from that one requirement:

* The bytes the proxy inspects must be the bytes the upstream will parse.
  Framing that is interpreted differently by the two sides is a bypass.
* If the proxy cannot establish which name applies, it must not forward. An
  unclassified connection is a denied connection.
* Nothing client-controlled is trusted: not lengths, not counts, not case, not
  encodings, not the number of times a field appears.

## 2. Stages and their contracts

```
client bytes
   |  (1) read first flight, bounded
   v
TLS records
   |  (2) frame and validate, then reassemble
   v
one handshake message
   |  (3) parse ClientHello body
   v
extension block
   |  (4) locate and validate server_name
   v
server name (wire form)
   |  (5) normalize both sides, match labels
   v
allow / deny
```

Each arrow is a contract; a stage may reject but may never "repair" its input
into something the next stage would not otherwise see.

### 2.1 Read

At most 16 KiB of the first flight is buffered. Reading stops as soon as a
complete handshake message is available, and fails when the budget is exhausted
without one. The budget is a ceiling, not a target: no configuration or peer
behaviour may cause a larger read.

### 2.2 Frame and reassemble

The record layer validates every length against both the protocol bound
(2^14 per record) and the bytes actually buffered. Handshake payloads from
consecutive handshake records are concatenated and cut to the length the
handshake message declares. The result must not depend on where record
boundaries fall.

Exactly one handshake message may be present. A ClientHello followed by any
further handshake bytes — in the same record or a later one — is a denial, not
a reason to use the first message.

### 2.3 Parse

The ClientHello body is parsed according to its own declared lengths, within
the message boundary. `legacy_version` is validated against 0x0301–0x0303.
The extension block must consume the body exactly.

### 2.4 Extract

The server_name extension is validated structurally before its value is used:
list length against extension length, `name_type == 0`, host name printable
ASCII and not dot-terminated. At most one server_name extension may exist;
duplicates are an error rather than a choice.

The value handed to policy is the wire value the parser validated, with at most
one trailing dot removed and case left alone. Trimming, case folding and any
other normalization belong to the matcher, which applies them to both sides
symmetrically.

### 2.5 Match

Two rule forms: exact, and `*.` plus a name that matches exactly one additional
label. Comparison is case-insensitive on ASCII and ignores one trailing dot on
either side. No rule match means deny.

## 3. Invariants a build must satisfy

These are properties, stated so they can be checked directly against a build.

1. **Chop invariance.** The same ClientHello bytes, delivered as one record,
   two records or three, yield the same parsed message and the same decision.
2. **Length honesty.** A record whose declared payload exceeds 2^14, or exceeds
   the bytes available, is a parse failure. A handshake message shorter than
   its declared length is a failure. Extra bytes past the declared handshake
   length are not part of the message.
3. **Single hello.** Any handshake message after the ClientHello causes denial,
   regardless of how it was framed.
4. **Exact block.** Extension data that overruns the extension block, or
   trailing bytes inside the block, is a failure. So is a ClientHello body that
   overruns the handshake message.
5. **Version gate.** `legacy_version` outside 0x0301–0x0303 is a failure.
6. **SNI strictness.** `name_type != 0`, non-printable-ASCII host bytes, a
   list length inconsistent with the extension, or two server_name extensions
   are all failures. A single trailing dot is removed rather than rejected.
7. **Label-aware wildcard.** `*.example.com` matches `a.example.com` and not
   `example.com`, `a.b.example.com` or `evil-example.com`.
8. **Symmetric normalization.** `WWW.Example.COM.` and `www.example.com`
   compare equal, whether the dot and the case appear in the rule, the client
   name, or both. The hello layer removes one trailing dot on the way out;
   matching must not depend on that having happened.
9. **Bounded peek.** A first flight that does not contain a complete
   ClientHello within 16 KiB is denied; the proxy does not read further.
10. **Fail closed.** No failure path forwards. No log line contains the raw
    client-supplied name; a failed parse has no validated name to log.

## 4. Failure model

Stages return errors; the decision layer maps every error to deny. There is no
best-effort mode, no fallback allow, and no distinction between "malformed" and
"not allowed" as far as forwarding is concerned. Diagnostics identify the stage
and, when a validated name exists, the matched rule — not the unvalidated wire
name.

## 5. Out of scope

Decryption, HTTP, certificate validation, connection reuse, client
authentication, and any second ClientHello on the same connection. The proxy
classifies the first flight and then gets out of the way; it does not rewrite
or reorder bytes.
