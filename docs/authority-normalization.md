# Authority normalization

`hostfold` turns whatever the proxy receives from upstream — an absolute URL or
a bare authority — into a single canonical triple:

```
(scheme, host_ascii, port)
```

Policy is allowed to look at that triple and nothing else. The whole point of
the canonical form is that **two spellings that reach the same origin must
produce the same triple**, and two spellings that reach different origins must
not. If the canonical host is not the host the proxy will actually dial, every
policy rule above it is decoration.

This document is the normative specification. It describes required behaviour,
not the way the code is arranged.

---

## 1. Input forms

Accepted inputs are:

* an absolute URL: `scheme://authority[path][?query][#fragment]`;
* a bare authority: `host[:port]`, with the IPv6 form `[addr][:port]`.

A bare authority is interpreted as `http`.

Recognised schemes are `http` (default port 80) and `https` (default port 443).
Anything else is refused with `unsupported_scheme`. Scheme matching is
case-insensitive; the canonical scheme is lowercase.

The path, query and fragment are irrelevant to normalization and are ignored.

---

## 2. Authority segmentation

The authority component is `[userinfo@]host[:port]`, where `host` is either a
registered name or an IP literal in square brackets.

### 2.1 Userinfo

Any occurrence of `@` in the authority is a refusal (`userinfo`). Credentials
in the authority are one of the oldest ways to make a URL *look* like it points
at one host while the connection goes to another; the proxy does not accept
them at all. There is no notion of "the real host is the part after `@`" here —
the whole authority is rejected, including a bare trailing `@`.

### 2.2 Host

For a non-bracketed host:

* an IPv6 literal must be written in brackets. An unbracketed address such as
  `2001:db8::1` is not a host; in an authority it is indistinguishable from a
  malformed `host:port` pair and is refused (`bad_ipv6`).
* the host component must be non-empty (`empty_host`).
* space, control characters (U+0000–U+001F, U+007F), and the characters
  `"<>\^`{|}` are never valid (`bad_host`). Percent signs are not decoded; a
  host that contains `%` is refused (`bad_host`) rather than decoded, because
  the decoded value may not be the value the origin sees.
* a single trailing dot is **not** part of the name: `metadata.internal.` and
  `metadata.internal` denote the same origin and must produce the same
  canonical host. Strip trailing dots before any further processing.
* a dot at the beginning, or two consecutive dots, produces an empty label and
  is refused (`empty_label`). This applies to the host as written, before and
  after trailing-dot handling.

Brackets belong to the IPv6 literal. In the canonical triple an IPv6 host is
rendered with its brackets, in lowercase, RFC 5952 compressed form, for
example `[2001:db8::1]`. A zone identifier (`%…`) inside the brackets is
refused (`bad_ipv6`).

### 2.3 Port

A port, when present, is a sequence of ASCII decimal digits only. Then:

* an absent port, or a present but empty port component (trailing `:`),
  selects the scheme default;
* a non-decimal port (letters, signs, whitespace) is refused (`bad_port`);
* the numeric value must be in `1..65535`; `0` and values above 65535 are
  refused (`bad_port`).

The canonical port is always the integer actually used for the connection.

---

## 3. Host mapping

After segmentation, a registered name is mapped to its ASCII form. The mapping
is the UTS #46 processing with `transitional = false` (IDNA2008), applied
label by label, with these consequences:

1. **Dot equivalents.** U+3002 IDEOGRAPHIC FULL STOP, U+FF0E FULLWIDTH FULL
   STOP and U+FF61 HALFWIDTH IDEOGRAPHIC FULL STOP separate labels exactly
   like U+002E. `evil。example` is the two-label name `evil.example`.
2. **Compatibility mapping.** Fullwidth and other compatibility variants of
   ASCII letters and digits map to ASCII. `ｅｖｉｌ.example` is
   `evil.example`.
3. **Case folding.** The result is lowercase. `Example.COM` and `example.com`
   are the same host.
4. **Non-ASCII labels.** A label that survives mapping with non-ASCII
   characters is encoded to an A-label (`xn--…`). For example `bücher.example`
   becomes `xn--bcher-kva.example`; with `transitional = false`, `ß` is kept
   as `ß` rather than folded to `ss`.
5. **Failures are errors, never pass-through.** If a label cannot be mapped to
   a valid A-label — invalid code points, a label longer than 63 octets, an
   A-label that does not round-trip, or an empty label after mapping — the
   whole authority is refused (`bad_hostname`). Returning the input string
   unchanged is not an acceptable outcome: an unmappable name is precisely the
   case where the proxy cannot know what it will dial.

ASCII input receives the same treatment: it is mapped (case folding, dot
equivalents) and validated. "Already ASCII" is not a reason to skip the
processing.

A canonical registered name is therefore lowercase ASCII, contains no empty
label, and has no trailing dot.

---

## 4. IP literals

### 4.1 IPv4

An IPv4 literal is recognised **before** the name path, because `2130706433`
and `127.0.0.1` are the same origin and a policy that lists one of them must
catch the other.

The host is an IPv4 literal when it consists of one to four dot-separated
parts, every part being a number written as:

* decimal (`127`, `2130706433`), optionally with leading zeros;
* hexadecimal with a `0x`/`0X` prefix (`0x7f`);
* octal with a leading `0` (`0177`).

The parts are read left to right into the 32-bit address; each part must fit
in the bytes that remain, and the final part fills them all. Rules of thumb:

* one part means the whole 32-bit value (`2130706433` → `127.0.0.1`);
* two parts mean the first part is one octet and the second fills the last
  three (`127.1` → `127.0.0.1`);
* three parts mean the first two are octets and the third fills the last two;
* four parts are four octets.

A part that is too large for the bytes it must fill, or five or more parts, or
a part that is not a valid number in one of the three forms, means the host is
**not** an IPv4 literal — and, because a dotted numeric name that is *almost*
an address is a classic confusion, it is refused (`bad_ipv4`) rather than
treated as a registered name. Octet values above 255 in four-part form are
refused.

The canonical IPv4 host is dotted decimal with no leading zeros:
`127.0.0.1`, `192.0.2.10`.

### 4.2 IPv6

See §2.2. IPv6 literals must be bracketed; the canonical form keeps the
brackets, is lowercase and compressed.

---

## 5. Policy matching

Policy files are YAML documents:

```yaml
version: 1
default: allow | deny
rules:
  - name: <unique, non-empty string>
    action: allow | deny
    hosts:    [<canonical host>, ...]      # optional
    suffixes: [<suffix>, ...]              # optional
    ports:    [<1..65535>, ...]            # optional
```

Unknown keys, a `version` other than `1`, a missing or invalid `default`, a
rule with an invalid `action`, duplicate rule names, or an out-of-range port
make the document invalid and the loader must refuse it rather than fall back
to permissive defaults.

Matching:

* rules are evaluated in file order; the **first** matching rule decides. If
  no rule matches, `default` applies;
* a rule matches a target only if its canonical host/port pair matches every
  constraint the rule states (`hosts`, `suffixes`, `ports` are ANDed; each
  list is ORed inside itself);
* `hosts` compares for equality against the canonical host;
* `suffixes` compares on **label boundaries**: a suffix `trusted.example`
  matches `trusted.example` and `api.trusted.example`, and must not match
  `nottrusted.example`. Equivalently, the host either equals the suffix or
  ends with `.` + suffix;
* matching is only ever done against canonical strings. Raw input must not be
  consulted anywhere in this layer.

---

## 6. Error taxonomy

Every refusal raises `hostfold.CanonicalizationError`, whose `reason`
attribute is one of:

| reason | meaning |
|---|---|
| `empty_target` | nothing to canonicalize |
| `empty_authority` | no authority component |
| `empty_host` | authority with an empty host |
| `userinfo` | authority contains `@` |
| `empty_label` | leading/adjoining dot in the host |
| `bad_host` | disallowed character in the host |
| `bad_hostname` | host cannot be mapped to a valid A-label |
| `bad_ipv4` | dotted-numeric host that is not a valid IPv4 literal |
| `bad_ipv6` | malformed or unbracketed IPv6 literal |
| `bad_port` | non-decimal or out-of-range port |
| `unsupported_scheme` | scheme other than http/https |
| `bad_url` | input that cannot be split as a URL at all |

A refusal is final: the proxy must not fall back to the raw input, and policy
must not be consulted.

---

## 7. Worked examples

| input | canonical triple |
|---|---|
| `https://api.trusted.example/v1/items` | `("https", "api.trusted.example", 443)` |
| `HTTP://example.com:8080/` | `("http", "example.com", 8080)` |
| `example.com` | `("http", "example.com", 80)` |
| `http://127.0.0.1/` | `("http", "127.0.0.1", 80)` |
| `http://2130706433/` | `("http", "127.0.0.1", 80)` |
| `http://0x7f.0.0.1/` | `("http", "127.0.0.1", 80)` |
| `http://127.1/` | `("http", "127.0.0.1", 80)` |
| `http://0177.0.0.1/` | `("http", "127.0.0.1", 80)` |
| `http://metadata.internal./` | `("http", "metadata.internal", 80)` |
| `http://evil。example/` | `("http", "evil.example", 80)` |
| `http://ｅｖｉｌ.example/` | `("http", "evil.example", 80)` |
| `http://bücher.example/` | `("http", "xn--bcher-kva.example", 80)` |
| `http://[2001:0DB8:0:0:0:0:0:1]:8443/` | `("http", "[2001:db8::1]", 8443)` |
| `http://api.internal@evil.example/` | refusal, `userinfo` |
| `http://[::1]:0/` | refusal, `bad_port` |
| `http://` + a 60-character non-ASCII label | refusal, `bad_hostname` |

With `policies/egress.yaml`, `http://nottrusted.example/` is denied by the
`example` suffix rule; `trusted.example`'s suffix rules do not reach it.
