# Property parameters

This document defines how parameters attached to iCalendar properties are
rendered. It covers quoting, escaping inside quoted values, multi-valued
parameters, and the two parameters with special handling (`CN`, `ALTREP`).

## 1. Parameter syntax

A property with parameters is written as

```
NAME;PARAM1=value1;PARAM2=value2:VALUE
```

The property name, every parameter, and the value are separated by `;`; the
property value is separated from the parameter list by a single `:`.

## 2. Parameter values

### 2.1 When a value must be quoted

A parameter value is written **without** double quotes only when it consists
exclusively of so-called *token* characters:

```
ALPHA / DIGIT / "-" / "." / "+" / "_"
```

and is non-empty. Every other value — in particular any value containing a
colon `:`, a semicolon `;`, a comma `,`, a double quote `"`, a backslash `\`,
or whitespace — must be enclosed in double quotes.

An empty parameter value is written as `""`.

### 2.2 Escaping inside quoted values

Inside a quoted parameter value:

- `"` is written as `\"`
- `\` is written as `\\`
- CRLF, LF and CR are written as the two-character sequence `\n` (as for TEXT
  values in `ical.md` §3.1); a raw line break may never appear in a parameter.
- `;`, `,` and `:` need no escaping inside quotes.

The escape is applied before the surrounding quotes are added. As with TEXT
values the backslash must be escaped first, and the escapes introduced for the
other characters must not be escaped again.

### 2.3 Parameters that take multiple values

Some parameters are defined as multi-valued. In this service that includes
`ROLE`, `DELEGATED-TO`, `DELEGATED-FROM` and `MEMBER`.

A multi-valued parameter with N values is written as **N separate
occurrences** of the same parameter name, in the order the values were
supplied:

```
ROLE=CHAIR;ROLE=REQ-PARTICIPANT
```

Each occurrence is quoted independently by §2.1. A comma is **never** used to
pack several values into one parameter occurrence: a comma inside a value is
ordinary data, is escaped as TEXT where the value is a TEXT value, and is
quoted when the value is quoted.

Concretely: an attendee whose single role text is `CHAIR,REQ-PARTICIPANT`
yields exactly one parameter, `ROLE="CHAIR,REQ-PARTICIPANT"`. An attendee with
the two roles `CHAIR` and `REQ-PARTICIPANT` yields two parameters,
`ROLE=CHAIR;ROLE=REQ-PARTICIPANT`. These are different and must not collapse
into one another.

For this service an attendee has exactly **one** role, so the `ROLE`
parameter on an `ATTENDEE` line has exactly one occurrence, and a comma
coming from user input is data, never a separator.

### 2.4 `CN` (common name)

- `CN` carries a display name and is always a single value.
- The value is escaped as TEXT (`ical.md` §3.1): backslash, semicolon, comma
  and line breaks are escaped, so a display name can never terminate the
  property or introduce a parameter.
- After escaping, the quoting rules of §2.1 apply: a display name that still
  contains a `"` or `:` or whitespace is quoted, and the quotes required by
  §2.2 are applied.
- `CN` is emitted on `ORGANIZER` and on every `ATTENDEE`.

### 2.5 `ALTREP` (alternate representation)

- `ALTREP` carries a URI and is a single value.
- A URI that is a valid token is written bare; any URI containing `:`, `/`,
  `?`, `&`, `=` or similar is quoted per §2.1.
- The URI is not TEXT-escaped; inside the quotes only `"` and `\` are escaped
  per §2.2. A line break in an `ALTREP` value is not permitted and is
  rejected rather than escaped.
- `ALTREP` is emitted only when an attachment description with an alternate
  location is supplied.

### 2.6 Parameter ordering

The parameter order on a property follows the order in which the parameters
are supplied. `CN` precedes `ROLE`. A single-valued parameter appears at most
once on a property; a multi-valued parameter appears once per value, with its
occurrences adjacent to each other.

## 3. Validation

A parameter value that is not a string, or that contains a raw CR or LF, is a
request error. The generated content line itself must never contain a raw CR
or LF inside a parameter; this follows from §2.2 and is what keeps user text
from creating new properties.