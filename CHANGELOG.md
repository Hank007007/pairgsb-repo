# Changelog

All notable changes to this project are recorded here.  The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Complexity analysis now implements the policy rules: nested repetition
  depth, overlapping alternation branches inside repeats and the 2^16
  repeat-count product.  All eight catastrophic shapes in the policy
  appendix (R1–R8) are rejected with `REPETITION_RISK`.
- The 512 character ceiling is measured after normalization, so patterns
  that grow during NFKC normalization or escape decoding are rejected with
  `PATTERN_TOO_LONG`.
- The matching budget is enforced per `search_all` request across all
  fields, and the engine is interrupted with a hard timeout instead of
  being measured after the match finishes.
- The compilation cache key now includes the effective flags, so the same
  pattern searched with different flags no longer shares a compiled entry.

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
