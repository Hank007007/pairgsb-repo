# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `kv` and `json` encoders for audit events, with a shared field ordering.
- Field collection layer with value normalization and documented error
  values.
- `cmd/auditdemo` for printing a representative event, and `cmd/siemcheck`
  for counting events as the receiver would.
- `docs/log-encoding.md` (wire contract) and `docs/design.md` (package
  design).
- Golden baseline for the ordinary-event output of both formats.
