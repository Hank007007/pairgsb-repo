# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `internal/runner`: `Build` renders a step declaration into a ready-to-run
  `*exec.Cmd` and reports the audit copies of argv and env.
- `internal/plugin`: dependency-free loader for the `plugin.yaml` subset
  described in `docs/design.md`.
- `cmd/echoargs`: helper that prints the argv, cwd and selected env entries it
  received; used by the demo and for manual verification.
- `cmd/demo`: builds a plan from a manifest, optionally runs it, and can build
  the same plan twice to confirm the result is reproducible.
- `examples/plugins/demo`: sample manifest covering a regular step and a
  legacy step.

## [0.1.0] - 2025-10-20

### Added

- First published cut of the execution-plan layer: plugin manifest loading,
  `Build`, and the `cmd/demo` entry point.
