# RFC 5424 profile

syslogfold accepts one message per datagram / line. The wire form is:

```
<PRI>VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP STRUCTURED-DATA [SP MSG]
```

The fields are separated by exactly one SP (0x20). No other whitespace is a
separator. A field may be the NILVALUE `-`, which means "the sender did not
supply this value"; NILVALUE is not a literal string and must not be forwarded
as one.

## 2 Header

### 2.1 PRI

`PRI` is `<`, then 1 to 3 decimal digits, then `>`.

- The numeric value is in the range 0..191 inclusive.
- Leading zeros are forbidden, with the single exception of the value `0`
  itself, which is written `<0>`.
- A message without a PRI, or with a PRI that is empty, non-decimal, negative,
  too long, or out of range, is not a syslog message and must be rejected. It
  must not be assigned a default facility or severity.

### 2.2 VERSION

`VERSION` is exactly the single character `1`. Any other value, including a
missing value, is rejected.

### 2.3 TIMESTAMP

`TIMESTAMP` is an RFC 3339 `date-time`:

- `full-date` `T` `full-time`, with `T` uppercase.
- The time carries an explicit offset: either `Z` or `+HH:MM` / `-HH:MM`.
- Fractional seconds are allowed and must be preserved; the profile does not
  round or drop them.
- The date and time must exist on the calendar. Values such as
  `2026-02-30T10:00:00Z` or `2026-13-01T00:00:00Z` are rejected even though
  they have the right shape.
- The instant may not be more than 300 seconds ahead of the receiving host's
  clock. A larger skew is treated as invalid.
- NILVALUE (`-`) is accepted for a sender that has no clock, and is represented
  as an absent timestamp downstream.

### 2.4 HOSTNAME, APP-NAME, PROCID, MSGID

Each of these fields is either NILVALUE, or a non-empty string of 1 to 255
printable US-ASCII characters (`%d33`..`%d126`). SP is not a printable ASCII
character in this sense and therefore may not appear inside a field; a sender
that needs to separate words must encode them. Each field is parsed on its own;
a value that contains `-` is an ordinary value and is not NILVALUE.

## 3 STRUCTURED-DATA

STRUCTURED-DATA is either NILVALUE or one or more SD-ELEMENTs written back to
back with no separator:

```
[SD-ID *(SP PARAM-NAME "=" %d34 PARAM-VALUE %d34)]
```

- SD-ID is 1 to 32 printable US-ASCII characters excluding SP, `=`, `]` and
  `"`.
- PARAM-NAME uses the same character set as SD-ID.
- PARAM-VALUE is a UTF-8 string in which `"`, `\` and `]` are escaped by a
  preceding backslash. The escapes must be emitted by producers and undone by
  this layer; an unescaped `"` always ends the value and an unescaped `]`
  always ends the element.
- The closing `]` of an element is the first `]` that is not part of an escape
  sequence and is not inside a quoted PARAM-VALUE.
- SD-ID values are unique within one message. Two elements with the same SD-ID
  make the message invalid; neither element may silently replace the other.

The end of STRUCTURED-DATA is the end of the last element. If a MSG follows, it
is separated from the structured data by exactly one SP. Everything after that
SP is MSG, including any `[` or `]` characters it contains.

## 4 MSG

MSG is optional and may be empty. It is UTF-8 and may begin with a UTF-8 BOM
(`EF BB BF`). The BOM is an encoding marker, not content: after decoding it is
not part of MSG and is not forwarded.

The size limit for one message is measured on the encoded byte length of the
whole wire message, not on the number of decoded characters. MSG, header fields
and structured data all count toward it.
