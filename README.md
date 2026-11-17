# runwright

`runwright` is a small, self-hosted CI executor. It reads `pipeline.yml` from a
repository, resolves the `${{ ... }}` expressions it contains against the
pipeline contexts, and runs each step in a child process.

```
bin/runwright.mjs      command line entry point
src/yaml.mjs           YAML reader used for pipeline files
src/config.mjs         pipeline shape validation
src/context.mjs        github/env/matrix/needs context construction
src/expr.mjs           expression evaluation and context path resolution
src/interpolate.mjs    substitution into scripts, env, with, if, name
src/secrets.mjs        secret storage and log masking
src/log.mjs            masked logging
src/runner.mjs         job/step scheduling and child process execution
docs/pipeline-spec.md  the behaviour the runner is required to have
examples/              a sample pipeline and a local action
test/                  node:test suites
```

## Usage

```sh
node bin/runwright.mjs --workspace path/to/repo
node bin/runwright.mjs --workspace path/to/repo --event event.json --event-name pull_request
```

Options:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | `pipeline.yml` | Pipeline file, relative to the workspace |
| `--workspace` | current directory | Repository root |
| `--event` | none | JSON file with the event payload exposed as `github.event` |
| `--matrix` | none | JSON file with matrix values exposed as `matrix` |
| `--github-repository` | from the event | `github.repository` |
| `--event-name` | from the event | `github.event_name` |
| `--timeout` | 600000 | Per-step timeout in milliseconds |
| `--quiet` | off | Suppress per-step progress lines |

Secrets are read from environment variables named `RUNWRIGHT_SECRET_<NAME>`.
They are available to pipelines as `secrets.<name>` in `env:` and `with:` only,
and are masked in everything the runner prints.

## Pipeline example

```yaml
name: ci

env:
  NODE_ENV: test

jobs:
  build:
    steps:
      - name: checkout
        run: |
          printf 'checkout %s\n' "$CHECKOUT_REF"

      - name: test
        if: ${{ github.event_name == 'pull_request' }}
        env:
          SUITE: unit
        run: |
          printf 'running %s tests\n' "$SUITE"

      - name: upload artifacts
        uses: local:scripts/upload.sh
        with:
          ARTIFACT_NAME: widget
```

Run the bundled sample with:

```sh
node bin/runwright.mjs --workspace examples --event event.json --event-name pull_request
```

`examples/event.json` is a realistic pull request payload. `--event` exposes it
as `github.event`, so expressions such as
`${{ github.event.pull_request.title }}` resolve against it.

## Tests

```sh
node --test
```

## Documentation

`docs/pipeline-spec.md` is the normative description of pipeline syntax,
contexts, interpolation, secret handling, environment composition, log masking
and execution results.
