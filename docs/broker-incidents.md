# Broker incident log

Collected reports from the platform team, tenant operators, and the on-call
rotation. Times are UTC. This file is a record of what people observed; it is
not a specification. Where it disagrees with `broker-design.md`, the design
document is authoritative.

---

## INC-2024-03-11 — "the beta operator could read our files"

Reported by: acme tenant operator.

Beta's job runner exports `FILESENTRY_UID=1001` in its unit file because a
third-party helper reads it. The platform starts a per-job broker instance from
that same unit, so the daemon inherits the variable. During a migration the
runner was pointed at its broker instance to move some files. It was able to
`read` and `list` under `/srv/tenants/acme/` even though the access file grants
beta uid 1002 only `/srv/tenants/beta/**`.

The helper's uid was 1002. The acme operator asked how the broker knew who was
calling; the answer we gave at the time was "the socket tells us". We could not
explain why the uid from the environment was reflected in the audit trail.

Follow-up: audit rows for the affected window carry uid 1001, and the same
result is reproduced by starting a broker instance with the variable set and
sending a request from an unrelated account. A broker started without the
variable refuses everything with `uid_unavailable`.

---

## INC-2024-04-02 — "acme-evil is not acme"

Reported by: security review.

A sandbox test created `/srv/tenants/acme-evil/x` owned by an unrelated local
account and ran a `list` request for that path as uid 1001. The broker answered
`ok=true` and returned the directory entries. The access file contains no rule
mentioning `acme-evil`.

The same review noted that a rule written as `/srv/tenants/*/reports` was used
to reach `/srv/tenants/acme/private/reports` — three segments deep — and that
`/srv/tenants/acme/../beta` was evaluated as if it still belonged to acme.

Reproduction on the review host: `/srv/tenants/acme/reports`,
`/srv/tenants/acme/private/reports`, and
`/srv/tenants/acme/private/deeper/reports` all select the
`/srv/tenants/*/reports` rule, and `/srv/tenants/acme-evil/x` selects the rule
for `/srv/tenants/acme/**`.

---

## INC-2024-04-19 — "the private directory is sometimes private"

Reported by: acme tenant operator.

`/srv/tenants/acme/private/` is supposed to be invisible to the tenant's own
automation. The access file grants acme uid 1001 `read` and `list` on
`/srv/tenants/acme/**` and a second rule denying everything on
`/srv/tenants/acme/private/**`.

Intermittent reads of `/srv/tenants/acme/private/keys.txt` succeeded. The
operator supplied a log from a cron job that probes the path once a minute:
over one night, 61 of 940 probes returned `ok=true` with the file contents.
Restarting the daemon changed the failure rate but did not eliminate it. The
operator's phrase: "it is private on Tuesdays".

---

## INC-2024-05-06 — "the link that should not have worked"

Reported by: acme tenant operator.

`/srv/tenants/acme/link` is a symlink used by a legacy build script; after the
build host was rebuilt it pointed at `/etc/shadow` instead of the intended
`/srv/tenants/acme/archive/shadow`. A `read` request as uid 1001 returned
`ok=true`. The operator pasted the first line of the response into the ticket
before realising what it was; the ticket has been redacted.

The broker's own logs show the request as authorised, with no indication that
anything unusual had happened. The operator expected a refusal that named the
reason. "Denied" would have been acceptable; silence was not.

---

## INC-2024-05-21 — "ownership moved off the tenant"

Reported by: on-call.

A maintenance script running as uid 1001 called `chown` on
`/srv/packages/agent-v3.tar` to make the file writable by the tenant's build
user. The broker carried out the change. The file had been owned by root and
is consumed by the host provisioning pipeline; provisioning failed the next
morning for every host built in the previous twelve hours.

The script was only supposed to touch `/srv/tenants/acme/`. There was no rule
for `/srv/packages/` for uid 1001 in the access file, and the request still
succeeded.

Follow-up: the on-call engineer asked whether `chown` was "handled the same way
as `read`". It was explained to them that it is not, and that they should not
assume the same checks apply.

---

## INC-2024-06-03 — "empty pattern behaviour"

Reported by: security review.

Two lower-priority observations from the same review as INC-2024-04-02,
recorded here so they are not lost:

* A request path with a trailing slash, or with `//` inside it, was treated as
  a different path from its normalised form in some parts of the daemon and the
  same path in others.
* A pattern ending in a literal directory name, such as
  `/srv/tenants/acme/reports`, was reported by one tenant as not covering
  `/srv/tenants/acme/reports/q1.csv`, while the same tenant reported that it
  did cover it on another host. The two hosts run the same access file.

Neither was reproducible on demand.
