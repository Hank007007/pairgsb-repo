# Incident reports

Post-incident notes from teams running earlier builds of sessforge.  They are
kept so that the reasoning behind `docs/session-policy.md` is not lost.  Each
entry records what was observed and what the policy now requires.

## 2021-03 — "the cookie that followed the user home"

A support engineer shared a screenshot from a troubleshooting session.  The URL in the
screenshot still carried the session identifier of an anonymous visitor.  The
visitor signed in from that browser minutes later and the engineer, who had the
screenshot, was able to drive the authenticated account with the same value.
The pre-login and post-login requests were served by the same record.

Policy response: §3.2–§3.3.  Login issues a fresh identifier and retires the
anonymous one.

## 2021-08 — "always-on dashboards"

Several operations dashboards polled once a minute, so their sessions were
never idle for more than a minute.  Some of those sessions had been open for
eleven days.  When the monitoring vendor was compromised, the stolen browser
profiles still had working cookies because the dashboards kept them warm.

Policy response: §4.2–§4.4.  The absolute deadline does not move.

## 2022-02 — "the sixth tab"

A customer opened a sixth tab and was told that they had too many sessions.
Their first five tabs included two that had been abandoned for a week.  The
customer could not find a way to log in without closing all other tabs, and
the help desk had no way to clear the stale ones.

Policy response: §5.2–§5.4.  The least recently used session is evicted to make
room, and the new login is admitted.

## 2022-09 — "the password reset that did not reset"

After a phishing report, the security team reset a user's password and
reviewed the account's active sessions.  The sessions created before the reset
continued to serve requests for the next several hours; nothing in the
validation path distinguished them from sessions created afterwards.

Policy response: §6.3–§6.6.  Credential changes advance the user's epoch and
sessions from older epochs fail on their next validation.

## 2023-04 — "both tabs logged in at once"

A long-lived remember-me token was replayed by two tabs racing to refresh at
the same moment.  One tab received a new token and continued; the other was
told the token did not exist.  Nobody looked further because "not found"
sounded like an ordinary expired cookie.  A later audit of the access logs
showed the original token value had been used from two different addresses the
previous night.

Policy response: §7.3–§7.8.  Tokens rotate, belong to a family, and a replayed
member invalidates the family and is reported as such.

## 2023-11 — "logout only cleared the browser"

A shared-kiosk deployment logged users out by expiring the cookie in the
response.  A copied cookie, captured before logout, continued to work after the
kiosk was returned.  The server-side row was still present and still valid.

Policy response: §9.1–§9.2.  Logout removes the record; the cookie expiry is
only the client-side half.
