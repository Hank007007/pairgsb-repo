# sudopolicy policy grammar

This document is the reference for the configuration file consumed by
`sudopolicy`. The evaluator's behaviour in terms of these rules is described in
[design.md](design.md).

## §1 File structure

A policy file is a sequence of rules, one per line.

* Blank lines are ignored.
* Lines whose first non-blank character is `#` are comments and are ignored.
* A rule is a single physical line. There is no line continuation.
* Rules are kept in the order in which they appear in the file. That order is
  significant; see §3.

## §2 Rule syntax

```
users hosts = (runas) [TAG:]... command [args...]
```

* `users` and `hosts` are comma-separated lists with no spaces inside them.
  * A user element is a literal user name, `%` followed by a group name, or
    `*`. `%name` applies to any operator who is a member of the group.
  * A host element is a literal host name or `*`.
* `runas` is a comma-separated list of target user names. The first element is
  the default identity used when the caller does not request one.
* `TAG:` is a tag name followed by a colon. Zero or more tags may appear. Tag
  names are case-insensitive; `NOPASSWD` and `SETENV` are defined in §7 and §8.
* `command` is the absolute path of an executable.
* `args` are zero or more argument patterns; their meaning is defined in §5.
* The `=` must be present and the target user list must be enclosed in
  parentheses. Malformed lines are configuration errors.

## §3 Evaluation order and negation

Every rule is considered in file order. A rule *matches* a request when all of
the following hold:

1. one of its user elements matches the operator (§4),
2. one of its host elements matches the target host,
3. its target user list permits the requested identity (§6),
4. its command identifies the requested executable (§5), and
5. its argument patterns accept the requested arguments (§5).

The outcome is decided by **the last matching rule in the file**, not the
first. Later rules override earlier ones.

A rule may be written with a leading `!` before the user list:

```
!bob web1 = (root) /usr/bin/systemctl restart
```

A negated rule matches exactly like a non-negated one, but when it is the last
matching rule the request is **denied**. A negated rule therefore only overrides
matches that occur before it; a later matching rule overrides the negation. All
other fields of a negated rule — users, hosts, target user, command and
arguments — participate in matching as usual, so a negation is scoped to the
combinations it names.

If no rule matches, the request is denied.

## §4 User and host matching

* A literal user name matches only the same name.
* `%group` matches when the request lists that group in its group membership.
* `*` matches any user.
* A literal host name matches only the same name; `*` matches any host.
* Names are compared case-sensitively.

## §5 Command and argument matching

The command is compared by **file identity**, not by file name: the path in the
rule and argv[0] of the request must denote the same executable. Paths are
compared after normalisation, and two paths that reach the same file through
symbolic links are the same command. Two different files that happen to share a
base name are different commands, so a rule for `/usr/bin/systemctl` does not
authorise `/tmp/evil/systemctl`.

The argument section of a rule forms a pattern for the arguments following
argv[0].

* If the rule lists no arguments, the command must be run with **no**
  arguments.
* Otherwise the argument patterns, joined with single spaces, are matched
  against the request's arguments, also joined with single spaces. The pattern
  must account for the whole argument string; there is no implicit trailing
  wildcard.
* `*` matches a sequence of characters that contains no whitespace.
* `**` matches a sequence of characters, whitespace included.
* Matching is case-sensitive and literal for all other characters.

So `ls *` accepts `ls -la` but not `ls -la /etc && sh`, while `ls **` accepts
both.

## §6 Target user

The requested identity must be one of the target users listed by the rule. When
the request does not ask for an identity, the first listed target user is used.
The chosen identity is reported in the evaluation result.

## §7 Passwords

Unless a rule carries `NOPASSWD:`, running the command requires a password. If
the request does not already carry a password, the command is not allowed and
the result tells the executor that a password is needed. `PASSWD:` is the
explicit form of the default. Tags of the last matching rule determine the
password requirement; tags of earlier rules do not.

## §8 Environment

Environment variables requested by the caller survive into the command only
when the last matching rule carries `SETENV:`. A request that asks to preserve
variables and is decided by a rule without `SETENV:` is denied; the policy does
not silently drop the variables. When the request asks to preserve nothing, the
result preserves nothing.

## §9 Result

An evaluation reports:

* whether the command is allowed,
* whether a password is needed,
* the identity to run as (empty when denied),
* the variables that survive into the command,
* the line number of the rule that decided the outcome, or zero when no rule
  matched,
* a short reason when the command is denied.
