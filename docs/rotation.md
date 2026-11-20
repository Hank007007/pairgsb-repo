# KEK rotation

This document specifies what a key-encryption-key (KEK) rotation must
guarantee. It describes behaviour; it does not prescribe an implementation.

## 1. What rotation does

Records sealed under a KEK version keep working as long as that version stays
in the key ring within its grace period. Rotation moves records off an old KEK
version and onto the ring's current active version:

1. unwrap each record's DEK with the KEK it was wrapped under;
2. decrypt and re-encrypt the payload under the new version's header
   (the associated data changes because `key_version` is authenticated — see
   `docs/envelope-format.md` §3);
3. wrap the DEK with the new KEK;
4. replace the record's header, nonce, wrapped DEK, and ciphertext together.

The DEK itself may be reused across the rotation of one record; a rotation is
not a reason to re-key the data, only to re-wrap its key.

Records that already use the active version are skipped: their bytes are left
untouched, and they are reported as skipped rather than rotated.

## 2. Atomicity

Rotation is **all-or-nothing** with respect to the set of records passed in:

* If every record can be rotated, the call succeeds and every record ends up
  on the target version.
* If any record cannot be rotated — because its wrapped DEK does not
  authenticate, because it names a version missing from the ring, because its
  ciphertext fails authentication, or for any other reason — the call fails
  and **every record must be left exactly as it was before the call**. No
  record may be left partly rotated: a record must never end up with a header
  claiming the new version while its DEK or ciphertext still belongs to the
  old one.
* A failed rotation reports an error. It does **not** report success.

The reason atomicity matters is that rotation is normally followed by retiring
the old KEK version. If a rotation silently left records on the old version
and the operator then retires that version, those records become unreadable
once the grace period ends. A partial rotation that reports success is
therefore a data-loss event.

Callers that persist records to disk are responsible for persisting the
post-rotation state as a unit (for example by staging new files and renaming
them into place). The library-level contract is that the in-memory records
handed to the rotation are either all updated or all unchanged.

## 3. Counts

The result value reports what actually happened:

* `TargetVersion` is the version records were moved to.
* `Rotated` is the number of records whose contents were re-wrapped and
  committed by this call.
* `Skipped` is the number of records that already used `TargetVersion` and
  were left untouched.

On failure, the counts must not claim work that was not committed. A count
that includes records the call did not (or could not) rotate is wrong, because
operators use these numbers to decide whether it is safe to retire the old
version. In particular, a failed rotation must never report a non-zero
`Rotated` count for work that was rolled back.

## 4. Rotation and retirement

Retirement is a separate, explicit step: setting `retired_at` on the old
version. Recommended order:

1. add the new version to the ring and make it active;
2. rotate every record onto the new version;
3. once rotation has succeeded and been verified, set `retired_at` on the old
   version;
4. after the grace period has elapsed, remove the old version from the ring.

Retiring a version while records still reference it is allowed only after
rotation has completed successfully. A record whose version has expired cannot
be rotated afterwards, because its KEK is no longer available to unwrap the
DEK.
