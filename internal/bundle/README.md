# Trust bundle layout

The trust bundle is a directory of epochs. Each epoch holds the anchors and
intermediate CAs that were valid while that epoch was active.

```
internal/bundle/
  epochs.json
  e0001/
    anchors/*.pem
    intermediates/*.pem
  e0002/
    anchors/*.pem
    intermediates/*.pem
```

`epochs.json` names the epoch currently in use, the epochs that may still be
used for verification, and the epochs that have been retired:

```json
{
  "current": "e0001",
  "active": ["e0001"],
  "retired": []
}
```

Rules:

- Every epoch named in `current` or `active` must have a directory.
- A certificate is verified only against the anchors of the epoch that issued
  it. Certificates issued under a retired epoch must not be accepted.
- During rotation an epoch may appear in both `active` and `retired`; the
  retired entry wins once the epoch has been drained.
- PEM files may contain more than one certificate. Anything that does not
  parse as a certificate is an error, not a skipped file.
- New epochs are added by writing a new directory and updating
  `epochs.json`; existing epoch directories are never rewritten in place.
