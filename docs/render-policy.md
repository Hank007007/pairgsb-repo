# Render policy

`termveil` sits between the log search service and the terminal. Matched log
lines are joined into a `text/plain` response before the color blocks are
returned. Every byte of that output reaches a terminal, and log content is
untrusted: applications write user input into their logs verbatim. The policy
below is the contract the render filter must uphold.

## 1. Strategies

A policy name selects exactly one of three strategies. Any other name is
undefined and must produce an error. An undefined name must never be mapped
onto a defined strategy.

### 1.1 `plain`

The response is textual only. Every control sequence is rewritten as visible
text so the reader can see what was in the log instead of executing it.

- An escape sequence is rendered as a literal `\x1b` followed by the bytes that
  follow the introducer, for example the three bytes `ESC [ 2 J` become the
  seven characters `\x1b[2J`, and `ESC [ 3 1 m` becomes `\x1b[31m`.
- OSC and DCS sequences are shown the same way: introducer, payload, and
  terminator as visible text. `ESC ] 0 ; t i t l e BEL` becomes
  `\x1b]0;title\x07`; a DCS terminated by `ESC \` ends in `\x1b\`.
- No ESC byte (0x1B) and no C1 byte may remain anywhere in the output.

### 1.2 `color`

Only a whitelist of SGR sequences survives; everything else is escaped as in
`plain`.

The whitelist is the set of SGR parameter lists that carry color, weight, or a
full reset:

| Parameters | Effect |
|---|---|
| `0` | reset |
| `1` | bold |
| `22` | normal weight |
| `30`–`37`, `39` | foreground color / default foreground |
| `40`–`47`, `49` | background color / default background |
| `90`–`97`, `100`–`107` | bright foreground / background |

A sequence may carry several of these joined by `;`, for example `1;31`. Every
parameter in the list must be white-listed; a single non-whitelisted parameter
rejects the whole sequence. In particular `8` (conceal) is not permitted, so
`ESC[8m` and `ESC[0;8m` are escaped rather than passed through. Any final byte
other than `m`, and any intermediate byte (0x20–0x2F), rejects the sequence.

A C1 CSI introducer (U+009B) is escaped like any other C1 control and is never
interpreted as an SGR introducer.

### 1.3 `width`

The input is filtered exactly as under `color`: white-listed SGR sequences are
copied through, every other control sequence is rewritten as visible text, and
control characters are escaped. The resulting text is then truncated to the
configured column limit.

- Truncation is by display width, not by rune count or byte count.
- A kept SGR sequence occupies zero columns and is always copied whole; a cut
  never lands inside a sequence.
- A full-width East Asian character (`East_Asian_Width` value `W` or `F`)
  occupies two columns; a half-width or ambiguous-width one occupies one.
- A zero-width joiner sequence (for example a family emoji or a
  skin-tone-modifier sequence) is one grapheme cluster and is never split. A
  ZWJ emoji cluster counts as two columns regardless of how many code points it
  contains.
- Combining marks attach to the preceding base character and add no width.
  Variation selectors and skin-tone modifiers add no width of their own; they
  belong to their base cluster.
- Truncation never leaves a partial UTF-8 sequence, a partial escape sequence,
  or an isolated ESC byte at the cut point. If the next cluster does not fit in
  the remaining columns, it is dropped whole.

The documented examples:

| Limit | Input | Output | Reason |
|---|---|---|---|
| 6 | `AB漢字CD` | `AB漢字` | `A`+`B` = 2 columns, `漢`+`字` = 4 more; `C` would start column 7 |
| 5 | `AB漢字CD` | `AB漢` | `字` needs two columns, only one remains, so it is dropped whole |
| 3 | `👨‍👩‍👧‍👦ab` | `👨‍👩‍👧‍👦a` | the family cluster takes two columns; `b` would start column 4 |

## 2. Character classes that never pass through

The following classes are escaped in `plain` and `width`, and rejected by the
whitelist in `color`:

- **C0 controls** (U+0000–U+001F) and DEL (U+007F).
- **C1 controls** (U+0080–U+009F). U+009B is the eight-bit CSI introducer; it is
  escaped like every other C1 control and is never interpreted as text or as an
  SGR introducer.
- **Bidirectional format characters** (U+202A–U+202E and U+2066–U+2069). These
  can reorder displayed text and must be escaped.
- **Zero-width and invisible format characters** (U+200B, U+200E–U+200F,
  U+2060–U+2064, U+FEFF).

U+200C (zero-width non-joiner) and U+200D (zero-width joiner) are not in that
class: they take part in emoji and script clusters and are handled by the
width logic instead.

## 3. What "escaped" means

A byte that belongs to a control sequence is shown as literal text: the ESC byte
becomes `\x1b`, and any other non-printing byte in the sequence (BEL in an OSC,
the `ESC \` that closes a DCS) is spelled with the same backslash form used
below. Printable bytes are emitted unchanged.

A standalone control character that is not part of a sequence (C0, C1, bidi,
zero-width) is shown as a backslash escape: `\xNN` for code points below
U+0080, `\uNNNN` otherwise, with lower-case hex digits.

## 4. Guarantees

1. `PolicyFor` returns an error for every name not defined above.
2. Under `plain`, the output contains no ESC byte and no C1 byte.
3. Under `color`, the only ESC bytes in the output introduce white-listed SGR
   sequences.
4. Under `width`, the only ESC bytes in the output introduce white-listed SGR
   sequences, and truncation never splits a rune, a cluster, or a sequence.

## 5. Sample logs

`testdata/` holds one sample line per hostile construct. `cmd/preview` renders
each of them under all three policies; with its default column limit of 6 it
also exercises the truncation examples above. Correct output for the
escape-related samples:

| Sample | `plain` |
|---|---|
| `erase.log` (`ESC[2J`) | `clear\x1b[2Jscreen` |
| `osc-title.log` | `title\x1b]0;evil\x07window` |
| `hyperlink.log` | `link\x1b]8;;https://evil.example\x07click here\x1b]8;;\x07` |
| `dcs.log` | `data\x1bP1;2;3qpayload\x1b\end` |
| `c1-csi.log` (U+009B) | `\u009b31mred\u009b0m text` |
| `bidi.log` | `invoice\u202efdp\u202c.txt` |

Under `color`, `ESC[31m` and `ESC[0m` survive while the conceal sequences in
`conceal.log` (`ESC[8m` and `ESC[0;8m`) become `\x1b[8m` and `\x1b[0;8m`.
Under `width`, `wide.log` yields `AB漢字` at limit 6 and `AB漢` at limit 5,
and `emoji.log` yields `👨‍👩‍👧‍👦a` at limit 3.
