# Changelog

All notable changes to this project are recorded here.  The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Policy layer for tenant supplied regular expressions: normalization and
  length accounting, the accepted syntax subset, complexity analysis, a
  bounded compilation cache, and a matching entry point with a per-request
  budget.
- `PatternCheck.check()` returning a `CheckResult` with a specific
  `ErrorCode` for each policy stage.
- `search_all()` returning matches grouped by field, together with the time
  the request took.
- Watchlist of pattern/flag pairs that overran the matching budget.
- Baseline tests for the normal path and `examples/run_checks.py`.

## [0.1.0] - 2026-03-01

### Added

- Initial project skeleton: package layout, documentation and the policy
  specification in `docs/pattern-policy.md`.
