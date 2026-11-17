# sudopolicy design

`sudopolicy` is the authorization library used by the bastion host executor.
The executor hands it a description of a command it is about to run and asks
whether the command may proceed, under which identity, whether a password is
needed, and which environment variables may be kept. `sudopolicy` answers from
a static policy file and never executes anything itself.

## Contract

The evaluator is a pure function of its inputs: the policy table and the
request. It does not read the process environment, the operating system's user
database, or the filesystem's notion of the current user. Group membership and
the requested identity arrive with the request, so the same inputs always give
the same result, which keeps decisions reproducible and testable.

A request carries:

* the operator's user name and group memberships,
* the target host name,
* the identity the operator asked to run as, if any,
* the argument vector the executor intends to execute, with the executable
  path in argv[0],
* the environment variables the operator wants to preserve,
* whether a password has already been supplied.

A result carries the decision, the identity to run as, the password
requirement, the variables that survive, the line of the deciding rule, and a
human-readable reason when the command is denied.

## Components

* **Policy model** — the in-memory form of the configuration: an ordered list
  of entries, each with its user, host and target-user lists, tags, executable
  path and argument patterns, plus a request and result type.
* **Parser** — turns configuration text into the ordered policy model,
  validating the grammar and reporting the line number of a malformed rule.
  Comments and blank lines do not become entries, but every other line keeps
  its position so that rule order and reported line numbers stay meaningful.
* **Matchers** — independent predicates for user, host, executable and
  argument matching. Each predicate answers one question about one aspect of
  the request; they do not know about rule order, tags or outcomes.
* **Evaluator** — combines the matchers, applies rule precedence and tags, and
  produces the result.

The public package re-exports the model types and offers `Load`, `Parse` and
`Evaluate` so the executor depends on one import path. `cmd/sudopolicy` is a
small command-line front end that makes the library usable by hand and in
tests of the surrounding system.

## Semantics

The normative behaviour is defined by
[policy-grammar.md](policy-grammar.md); the points below summarise the design
decisions behind it.

### Precedence

Rules are a sequence, not a set. The last matching rule decides the outcome.
This lets an operator state a broad rule and then carve out exceptions, or
state a general exception and then re-allow a narrower case, reading the file
from top to bottom. Because position is what gives a rule its power, the same
rule moved up or down the file can change the decisions of the table.

### Negation

A leading `!` marks a rule as a denial. It is a normal rule for matching
purposes and a denial for outcome purposes. Because precedence is
last-match-wins, a negation overrides the rules above it and is itself
overridden by matching rules below it. Negations are scoped: they deny only
the user/host/identity/command/argument combinations they spell out, never a
whole command or a whole user.

### Command identity

Authorization is about the file that will be executed, so the executable named
by a rule and the executable named by argv[0] are the same command only when
both paths lead to the same file. A rule for `/usr/bin/systemctl` does not
authorize some other `systemctl` that an attacker placed elsewhere.

### Arguments

The absence of argument patterns is meaningful: it authorises the bare
command only, with no arguments. Wildcards are deliberately weak by default —
`*` stops at whitespace — so that a single pattern cannot swallow a shell
metacharacter sequence, a second command, or a pipe that the executor would
later interpret. `**` is available when a rule really does intend to accept an
arbitrary argument tail.

### Tags

Tags qualify the rule they are attached to and nothing else. The deciding rule
is the only source of the password requirement and of the environment
permission. `SETENV` is an explicit grant: wanting to preserve variables
against a rule that does not grant it is a denial, not a silent strip, because
silently dropping `LD_PRELOAD` or a similar variable changes the meaning of
the command the operator asked for.

### Errors

Configuration errors are reported when the policy is loaded, with a line
number, and prevent a table from being produced. An evaluation without a table
or without a command is an error. A well-formed request that is not authorised
is not an error; it is a denied result carrying a reason, so the executor can
log it and, for a password requirement, prompt and retry.

## Testing

The library is tested at two levels: the matchers are exercised directly as
predicates, and the evaluator is exercised end to end with a table and a
request. The tests use only the standard library. Policy text is supplied in
memory, and executable paths come from temporary directories, so the tests do
not depend on the host's accounts, installed programs, or environment.
