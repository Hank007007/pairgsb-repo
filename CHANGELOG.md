# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `docs/merge-rules.md` is now the normative description of layer order, key
  admissibility, environment coercion, and JSON Patch behaviour.
- `docs/design.md` explains the module split and the properties the design is
  meant to preserve.
- `examples/proto-check.js` loads a fixture with hostile keys and reports
  whether the global object stayed clean.

### Changed

- `applyPatch()` reports every operation outcome through its `errors` array;
  the existing `{ document, errors }` shape is unchanged.

## [0.3.0] - 2025-09-02

### Added

- Layered loading: defaults, `base.json`, environment overlays, environment
  variables, and `--set` entries.
- JSON Patch entry point with `add`, `remove`, `replace`, `move`, `copy`, and
  `test`.
- Environment variable nesting through the `__` separator.
- Error records with `code`, `path`, and `message` for every surface.

### Fixed

- Layer files are parsed independently so a malformed overlay no longer
  prevents later overlays from applying.
- `load()` clones its defaults instead of aliasing them, so mutating the
  returned document no longer reaches the caller's object.

## [0.2.0] - 2025-06-18

### Added

- `--set path=value` support in `load()`.
- Dotted path helper shared by the command line and the environment overlay.

### Fixed

- Overlay arrays replace the value found in earlier layers rather than
  concatenating onto it.

## [0.1.0] - 2025-04-07

### Added

- Initial `load({ defaults, layers, env, prefix, argv })` surface.
- Recursive merge of plain objects across layers.
