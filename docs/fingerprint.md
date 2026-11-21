# Finding fingerprints

This document is normative. It defines how a finding's identity is computed.
The fingerprint is what lets the ledger recognise the same problem across scans.

## §1. Inputs

A fingerprint is derived from exactly three things:

1. the normalized file path,
2. the rule identifier,
3. a normalized snippet of the code at the finding's location.

The line number is **not** an input. Neither is the severity, the message, the
scan timestamp, nor anything else about the run.

## §2. Path normalization

Before hashing, the path is normalized:

1. backslashes become forward slashes;
2. redundant `.` segments and repeated separators are removed, and `..` is
   resolved where it can be;
3. the result is compared without regard to ASCII case.

Points 1 and 2 mean `./a.go`, `a.go` and `src/../a.go` all denote the same file.
Point 3 means `A.go` and `a.go` also denote the same file, which is what
case-insensitive filesystems (macOS, Windows) already assume. All forms of the
same file must produce the same fingerprint, and a finding reported once per
form must be counted once.

## §3. Snippet normalization

The snippet is the source text around the match that the scanner supplies. It is
normalized before hashing:

1. leading and trailing whitespace is trimmed;
2. runs of inner whitespace collapse to a single space;
3. the result is compared without regard to ASCII case.

The purpose of the snippet is to distinguish two findings of the same rule in
the same file. It is deliberately coarse: formatting changes, re-commenting, and
re-indentation must not change it. A snippet must therefore be stable across
edits that do not change the code being flagged.

When a scanner supplies no snippet, the snippet component is the empty string.

## §4. The algorithm

```
key = normalizePath(file) + "\x1f" + ruleId + "\x1f" + normalizeSnippet(snippet)
fingerprint = lowercase hex SHA-256 of key
```

The unit separator (`\x1f`) is used instead of a printable character so that a
path or rule id containing `|` or `:` cannot be confused with a field boundary.

The fingerprint is a 64-character lowercase hex string. `findingId` is its first
12 characters.

## §5. Stability requirement

Moving a finding up or down in a file, adding or removing unrelated lines
elsewhere in that file, or reformatting the match must leave the fingerprint
unchanged as long as the normalized snippet is unchanged.

Consequently:

- a finding that shifts by a few lines is still the same finding: no new record,
  no state reset;
- a finding whose state is `fixed` and that reappears is recorded as `reopened`,
  not as a fresh `open`;
- two paths that denote the same file collapse into a single finding.

A change to the flagged code itself may legitimately produce a different
fingerprint, because the problem is no longer the same problem.
