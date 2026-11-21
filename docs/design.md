# findingsledger — design notes

This is the design note that accompanies the normative specifications in
`docs/`. It explains what the system is for and what correct behaviour looks
like, without prescribing an implementation.

The individual guarantees are specified in:

| Topic | Document |
|---|---|
| record format, hash chain, checkpoints, verification | `docs/ledger-format.md` |
| finding identity | `docs/fingerprint.md` |
| states and transitions | `docs/lifecycle.md` |
| waiver approval, expiry, signing | `docs/waivers.md` |
| diff output, exit code, determinism | `docs/diff.md` |

## 1. Problem

Every repository runs a scanner on every build. The scanner is stateless: it
tells you what is wrong right now, with the file and line where it saw it. That
is enough to fail a build, but not enough to run a security programme, because
the interesting questions are historical:

- is this the same problem we saw last month, or a new one?
- has somebody already decided to accept it?
- who accepted it, why, and until when?
- did the fix we made actually stick?

The ledger answers those questions by turning a stream of scans into a history.
It is deliberately narrow: it does not scan anything, and it does not decide
what a security issue is. It records what the scanner saw, recognises identity
across scans, and tracks decisions.

## 2. Three responsibilities

### 2.1 Identity (fingerprints)

The ledger has to decide, given a finding from this scan and a history of
previous findings, whether they are the same thing. A line number is a poor
identity: it is where the code currently sits, not what the code is. An
identity built on line numbers turns every edit above a finding into a resolve
plus a new finding, which makes the history useless and makes reopens
impossible.

A fingerprint is therefore built from the file (normalized so that equivalent
paths coincide), the rule, and a coarse snippet of the matched code. The
resulting identity survives moving the finding, reformatting around it, and
seeing the same file through a different spelling of its path; it changes when
the flagged code itself changes. See `fingerprint.md` for the exact rules.

### 2.2 State (lifecycle)

Each fingerprint has a state. The states are few and the transitions are
deliberately restrictive, because the value of the ledger is that a state means
something: `fixed` means the scanner stopped reporting it, `waived` means a
human decision is in force, `reopened` means a resolved problem came back.
A transition that is not drawn in `lifecycle.md` is a programming or process
error, and the ledger should refuse it rather than invent a meaning for it.

Two details matter more than they look:

- a waiver's expiry is part of the state. A `waived` finding whose waiver has
  expired is `open` again for every purpose the ledger serves, without any
  record needing to change;
- sequence numbers identify positions in the history. They must be strictly
  increasing under any concurrency, because "the record at seq N" is the
  address used by verification and by incident response.

### 2.3 Decisions (waivers)

Suppression is the part of a scanning programme that most easily goes wrong:
something is silenced, and six months later nobody knows who silenced it or
whether the reason still holds. The ledger makes suppression an object: a
waiver with a reason, a submitter, an approver and an expiry, signed so that it
cannot be edited after the fact.

The rules that give that object its meaning are the two-person rule (the person
who wants the silence cannot also approve it), a bounded lifetime (nothing is
silenced forever by accident), and integrity (changing any field invalidates
the signature, so an operator who wants a longer waiver has to write a new one,
with a new approval). See `waivers.md`.

## 3. Integrity model

The ledger file is the record of what happened. Its integrity rests on a hash
chain plus signed checkpoints:

- each record hashes its own contents and names its predecessor's hash, so any
  edit to a record invalidates it and the record after it;
- periodically a checkpoint signs the hash at a sequence number, so the
  position of the chain is anchored outside the file itself.

Verification recomputes all of this from the file contents. It has to be able to
answer, precisely, "the history is broken at seq N", and to distinguish a
tampered record from a tampered checkpoint. Verifying only part of the chain,
or reading a checkpoint without checking its signature, would produce a green
verification that proves nothing. See `ledger-format.md` §6 for the required
checks.

## 4. The CI gate

`scanctl diff` is the only part of the system a build pipeline normally sees.
Three properties make it usable as a gate:

- it is keyed on identity, so a rebase or a formatter run does not produce
  spurious failures;
- it only counts genuinely new high-risk findings, so fixing an old finding or
  carrying an accepted one does not block a build;
- it is deterministic, so its JSON output can be committed as a snapshot and
  compared on every run.

A consequence of the last point, and of the "append-only" premise in general, is
that a baseline finding which disappears must be recorded, not merely omitted:
the ledger is the memory, and a memory that silently forgets is worse than no
memory at all. See `diff.md`.

## 5. Operating model

- `LEDGER_KEY` is provisioned to writers and to the verification job, and to
  nobody else. It is never committed.
- The ledger and its checkpoint sidecar are stored together and backed up
  together; restoring one without the other is a broken restore.
- `scanctl verify` runs in CI on every change to the ledger, not only after an
  incident.

## 6. Scope

Out of scope: the scanner itself, severity policy, ticket-system integration,
and any UI. The ledger is a file format, a small library, and a command; that
is the whole surface it intends to own.
