# Policy

This document defines how the proxy turns a server name into an allow or deny
decision, what the rule file looks like, and how failures are handled. It
describes behaviour, not code.

## 1. Scope

The proxy is an egress gate: a connection is forwarded to its upstream only
when the name the client presented is explicitly allowed. There is no implicit
allow anywhere in this document.

## 2. Rule file

### 2.1 Format

The rule file is plain text, one rule per line. Blank lines are ignored. A line
whose first non-space character is `#` is a comment and is ignored. Everything
else is a rule, with surrounding whitespace removed.

### 2.2 Kinds of rule

Only two rule forms exist:

* **Exact**: a bare name. It matches a client name that is the same name.
* **Wildcard**: the two characters `*.` followed by a bare name. It matches a
  client name that has exactly one additional label in front of that bare name.

Anything else is not a rule: a `*` that is not the leading `*.`, a bare `*`, a
name containing `*` anywhere else, or an empty name. A rule file containing a
malformed rule is rejected at load time rather than being partially loaded.

The wildcard is deliberately narrow. `*.example.com` matches `a.example.com`
and nothing else in that subtree. It does not match `example.com` itself (that
requires no extra label), and it does not match `a.b.example.com` (that is two
extra labels). It also cannot match a name that merely *ends* with the rule's
suffix without a label boundary: `evil-example.com` is not in `example.com`.

## 3. Matching

### 3.1 Evaluation

The client name is compared against every loaded rule. A name is allowed when at
least one rule matches. The order of rules in the file does not affect the
result; there are no negative rules and no first-match semantics at this layer.

### 3.2 Label boundaries

Matching is on labels, not on byte suffixes. The boundary between the wildcard
label and the rule's suffix is a dot. The boundary at the start of an exact rule
is the start of the name. Substring, prefix, and suffix comparison that ignore
where labels start and end do not implement this document.

### 3.3 Default deny

A name that matches no rule is denied. So is a name for which the policy has no
rules at all. There is no fall-through to the upstream and no "log and allow"
path.

### 3.4 Empty and invalid names

An empty name never matches. A name that failed validation in
`docs/hello.md` §2.4 never reaches matching; if one somehow does, it is denied.

### 3.5 Name normalization

Comparison is case-insensitive. Before comparison both the rule name and the
client name are normalized: ASCII letters are folded to lower case, and a single
trailing dot is removed from either side if present. A trailing dot is a legal
fully-qualified form, so `WWW.Example.COM.` and `www.example.com` are the same
name. Normalization is symmetric: a rule written with a trailing dot and a
client name that arrived with one must compare equal to the same names without
dots, and so must every mixture of the two.

No other normalization happens. No IDNA/punycode conversion, no percent
decoding, no whitespace stripping beyond the line trimming of §2.1, no label
reordering.

## 4. Failures

### 4.1 Peek budget

The proxy may buffer at most 16 KiB of the client's first flight while deciding.
If a complete ClientHello has not arrived within that budget, the connection is
denied. The limit is a hard ceiling: the proxy must not keep reading in the hope
that a larger buffer will eventually contain the message. A record header that
declares a payload larger than the remaining budget cannot be satisfied under
this limit and is a failure for the same reason.

### 4.2 Any parse failure denies

If any stage fails — the record layer, the hello parser, the SNI extraction, or
policy loading — the connection is denied. There is no path where a failure to
classify a connection results in it being forwarded. "Could not determine the
name" and "the name is not allowed" have the same outcome.

### 4.3 Logging

Diagnostics may record that a connection was denied and why, at a coarse
granularity: the stage that failed and the rule or reason. They must not record
the client-supplied server name verbatim. The name is unvalidated,
attacker-chosen data; writing it to logs spreads it beyond the one connection
that sent it and can be used to forge log entries. If the name must appear at
all, only a validated form may, and failures by definition have no validated
form.
