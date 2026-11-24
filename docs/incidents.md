# Incidents

Two production incidents shaped the rules in the other documents. They are
recorded here so the reasons behind those rules are not lost.

## 2023-11-08 — a label ended a command

A user created a label containing a line break followed by `LOGOUT`. The
frontend sent the label as the value of a search key. The proxy wrote the value
into the command without treating it as an astring, so the line break ended the
command early and the text after it was executed as another command in the
same authenticated session.

Impact: the session was closed mid-request and the client received replies it
could not match to any outstanding command.

Rule derived: every user-controlled value is an astring. CR, LF and NUL cannot
be represented in a quoted string and must never be passed through; values that
need them use a literal, whose length is declared up front.

## 2024-02-19 — the same mailbox was cached twice

A client listed `INBOX`, then referred to `inbox` in a later request. The
folder cache keyed entries by the exact string it had received, so the two
spellings produced two entries and two upstream selections. Message counts and
flags were read from two different snapshots of the same mailbox.

Impact: unread counts flickered, and a mark-as-read could be applied to a
selection the user did not see.

Rule derived: mailbox identity is defined by the decoded name with `INBOX`
canonicalized, not by the raw bytes the client happened to send. Other names
stay case-sensitive.
