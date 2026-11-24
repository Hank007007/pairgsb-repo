# Size limits

These limits are enforced per message, on the wire representation.

## 1 Message size

A single message may be at most 8192 bytes. The limit applies to the UTF-8
encoding of the message as received: a CJK character takes three bytes, so a
body of 4000 Chinese characters exceeds the limit even though it is only 4000
characters long. A message that exceeds the limit is rejected as a whole; its
fields are not partially forwarded.

## 2 Structured data elements

One message may contain at most 32 SD-ELEMENTs. A 33rd element makes the
message invalid, irrespective of its content.

## 3 Parameters per element

One SD-ELEMENT may contain at most 64 PARAM-NAME/PARAM-VALUE pairs. A 65th
pair makes the message invalid.

The limits in this document are checked after structural parsing, so a message
that is both malformed and oversized is reported with the structural reason.
