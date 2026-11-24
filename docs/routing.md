# Routing, de-duplication and forwarding

## 1 Sources and routes

A route is a tuple of network, hostname label and tenant:

```
network          hostname label      tenant
10.0.0.0/8       acme                acme
10.20.0.0/16     acme                acme-finance
100.0.0.0/8      acme                lab
192.168.0.0/16   edge                edge-ops
```

A route matches a message when both conditions hold:

1. The sender address is contained in the route's network. Containment is
   decided on the numeric address, so `10.0.0.0/8` contains `10.99.1.1` and
   does not contain `100.1.1.1`.
2. The hostname label equals the first DNS label of HOSTNAME (the text before
   the first `.`, compared exactly and case-insensitively). A hostname with no
   dot uses its whole value as the label. The label comparison is an equality:
   `acme-evil` has the label `acme-evil` and therefore does not match `acme`.

## 2 Tenant selection

The tenant of a message is the tenant of the matching route with the longest
network prefix. Declaration order is irrelevant: a more specific route beats a
less specific one. If several routes have the same prefix length, the first
declared among them wins.

If no route matches, the message goes to the tenant `unassigned`.

HOSTNAME is validated before it is used for routing. A malformed hostname is
not a routing key and the message cannot be assigned by hostname; it goes to
`unassigned`.

## 3 De-duplication

The de-duplication key identifies one event, not one timestamp. Two messages
that differ in severity (the low three bits of PRI) or in PROCID are different
events even when TIMESTAMP, HOSTNAME and MSGID are identical. The key is
therefore built from at least:

- TIMESTAMP
- HOSTNAME
- MSGID
- severity
- PROCID

Facility may be added when it is needed to separate sources, but severity and
PROCID are mandatory. Two messages with the same key are duplicates; two
messages that differ in any key component are both kept, even within the same
tenant and the same second.

## 4 CEF forwarding

Each accepted message is forwarded as one CEF line:

```
CEF:0|syslogfold|syslogfold|1.0|<msgid>|<name>|<severity>|<extension>
```

The header fields `<msgid>` and `<name>` escape `\` as `\\` and `|` as `\|`.
The extension is a space-separated list of `key=value` pairs. In every
extension value:

- `\` is escaped as `\\`
- `=` is escaped as `\=`
- newline (LF) is escaped as `\n`
- carriage return (CR) is escaped as `\r`

A downstream collector that splits the extension on unescaped spaces and then
on the first unescaped `=` must recover exactly the same keys, values and
order that were emitted. A value may not be able to introduce a new key/value
pair or a new line.

Messages that fail validation are not CEF records. They are reported to the
caller as errors together with the reason; the original bytes are never
wrapped into an accepted CEF record.
