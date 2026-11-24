# Operations log

## 2026-01-14 — forged alert through a user-supplied field

An application wrote a user-controlled display name into its log line without
sanitising it. The name contained a newline followed by text shaped like a
complete syslog message from a firewall. The downstream collector received two
lines where syslogfold had accepted one message: the original record plus a
second, entirely fabricated alert attributed to the firewall. The collector
has no way to tell the two apart because both looked like well-formed CEF.

Follow-up: any value that leaves syslogfold must not be able to start a new
record, a new extension pair, or a new line, regardless of what the sending
application put into it.

## 2026-02-03 — two devices merged into one tenant channel

Two devices at different sites used the same short hostname, `acme-edge`. They
sent from different network ranges. Both were routed to the same tenant
channel and their events were de-duplicated against each other, so a portion
of one site's audit trail disappeared from the channel and the remainder was
attributed to the wrong site. On-call could not reconstruct which device had
produced which record.

Follow-up: the tenant is determined by both the sender's network and the
hostname, and de-duplication must not collapse events from different
severities or processes. Attribution must not depend on a substring of a
hostname.
