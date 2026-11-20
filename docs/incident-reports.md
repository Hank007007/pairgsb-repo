# Incident Report — INC-2025-0417

**Title:** Downstream export job obtained write scope outside the user's authority
**Severity:** High
**Status:** Closed
**Owner:** Data Platform Security

## Summary

A batch export service (`svc-export`) exchanged end-user access tokens for
downstream tokens and received `files:write` even though the affected users
held only read access. The downstream token was accepted by the warehouse
gateway for eleven hours before the job was stopped.

## Timeline (UTC)

| Time | Event |
|---|---|
| 2025-04-17 08:12 | A scheduled export begins. `svc-export` calls the exchange endpoint for each user in a reporting group. |
| 2025-04-17 08:14 | Warehouse gateway logs show `files:write` operations attributed to users whose directory groups grant read-only access. |
| 2025-04-17 09:40 | A data steward notices unexpected modification timestamps on two shared datasets. |
| 2025-04-17 10:05 | Security on-call begins tracing the writing identity through gateway logs. |
| 2025-04-17 11:22 | Exchange traffic for `svc-export` is disabled. |
| 2025-04-17 19:15 | Affected tokens expire; gateway rejects the writes. |
| 2025-04-18 | Post-incident review; remediation work opened. |

## What happened

The export service is registered for `datasets:read`, `files:read`, and
`files:write`, because a separate administrative workflow legitimately needs
write access. Its registration therefore lists the write scope.

For each user the service requested a scope set that included `files:write`.
The affected users' access tokens carried only `datasets:read` and
`files:read`. The exchange service issued tokens containing the union of the
requested and registered scopes. Because the user's own token was not
consulted when the effective scope was computed, the issued token carried the
write scope that the user never held.

The gateway saw a correctly signed token with `files:write` and a plausible
`client_id`, so it accepted the writes. No individual request looked unusual.

## Contributing factors

1. **The subject token's authority was not part of the narrowing decision.**
   The issued scope was derived from the request and the caller's
   registration, so the end user's actual grants never constrained it.
2. **Registration scope was treated as a grant.** A client registration
   records what a service may ever hold, not what a particular end user may
   delegate at a particular moment.
3. **Delegation history was not durable.** Investigators could not reconstruct
   which user token each downstream token descended from, because the
   delegation chain on issued tokens held only the most recent hop. Tying a
   downstream token back to its originating token required correlating gateway
   logs by timestamp.
4. **Revocation did not propagate.** When the incident response team revoked
   the first exchange's subject token, tokens already derived from it remained
   valid until expiry.

## Impact

- 38 users' read-only authority was exceeded for the duration of the batch.
- Two shared datasets received unintended writes. Both were restored from
  snapshots; no data left the platform.
- No credential or signing key was exposed.

## Remediation

- Scope narrowing now considers every source of authority named in the
  exchange specification, and the issued scope cannot exceed any of them.
- Delegation chains preserve every hop so that a downstream token can be
  traced to its originating token without log correlation.
- Revocation of a token invalidates tokens derived from it.
- Client registrations are reviewed so that no service is registered for
  scopes it does not need.

## Open follow-ups

- Gateway audit logging should record the full delegation chain for write
  operations.
- Add an alert when an issued token carries a scope absent from the subject
  token.
