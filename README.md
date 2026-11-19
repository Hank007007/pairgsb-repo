# argvsieve-exec-args

Execution-plan layer for an internal CI runner. It turns a plugin's step
declaration plus the values a pipeline author typed into the UI into an
`*exec.Cmd` that the runner can start.

- The signed-off security contract lives in [`docs/argv-policy.md`](docs/argv-policy.md).
- The intended behaviour, data flow and public contract are described in
  [`docs/design.md`](docs/design.md).

## Layout

```
cmd/demo            build (and optionally run) a plan from a manifest
cmd/echoargs        helper that prints the argv / cwd / env it received
internal/plugin     dependency-free plugin.yaml loader
internal/runner     rendering, env merge, argv assembly (Build)
docs/               policy and design notes
examples/plugins/   sample manifests
```

## Build and test

```sh
go build ./...
go test ./...
```

## Try it

The demo builds `./bin/echoargs` from `cmd/echoargs` on first use.

```sh
# Build the plan and show the resulting argv/env, twice, to compare them.
go run ./cmd/demo -set GREETING="hello world" -set TAG=ci -set BRANCH=main -twice

# Also start the process.
go run ./cmd/demo -set GREETING="hello world" -set TAG=ci -set BRANCH=main -run

# The sample manifest also ships a step whose declaration predates the
# discrete-argv format.
go run ./cmd/demo -step legacy-report -run
```

The helper prints `argv[i]=...` lines, the working directory and a few
environment entries, so the built plan can be checked without extra tooling.

## Acceptance checklist

A correct implementation satisfies every item below; each one maps back to a
clause in `docs/argv-policy.md`.

1. Values containing `;rm -rf /`, `$(id)`, backticks or `&&` still produce an
   argv whose `argv[0]` is the declared executable, and no `sh`/`bash` shows up
   as `argv[0]` in the process tree (§1).
2. A single `${INPUT_X}` expands exactly one level; if its value contains
   `${INPUT_Y}` that text survives verbatim (§2.1).
3. A value containing spaces remains one argv element (§2.2).
4. `LD_PRELOAD`, `PATH`, `GIT_SSH_COMMAND`, `NODE_OPTIONS` and friends are
   dropped from a step's env; duplicate keys collapse to one entry that matches
   the allowlist in appendix A (§3).
5. Every positional argument sits after a single `--`, and a positional that
   starts with `-` is not treated as an option (§4).
6. An argument containing NUL (or a raw newline) makes `Build` fail with the
   documented error code instead of reaching `exec` (§5, §7).
7. The legacy sample step still runs, with the behaviour described by the
   policy (§1.4).
8. Two builds of the same declaration produce byte-identical argv and env
   (§6); `cmd/demo -twice` reports `reproducible: yes`.
