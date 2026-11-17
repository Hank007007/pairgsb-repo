# sudopolicy

`sudopolicy` is the authorization library used by the bastion host executor.
Given a policy table and a description of a command the executor is about to
run, it decides whether the command is allowed, which identity it runs as,
whether a password is needed, and which environment variables may be kept.

The library is pure Go and depends only on the standard library.

## Layout

```
sudopolicy.go              public API: Load, Parse, Evaluate and the model types
internal/policy            policy model: rules, requests, results
internal/parse             configuration file parser
internal/match             user, host, executable and argument predicates
internal/eval              policy evaluation
cmd/sudopolicy             command line front end
policy/sudopolicy.conf     example policy table
docs/policy-grammar.md     configuration syntax and semantics (normative)
docs/design.md             design and reasoning
```

## Usage

```go
table, err := sudopolicy.Load("policy/sudopolicy.conf")
if err != nil {
    return err
}

result, err := sudopolicy.Evaluate(table, sudopolicy.Request{
    User:    "bob",
    Groups:  []string{"ops"},
    Host:    "web1",
    Runas:   "root",
    Argv:    []string{"/bin/ls", "-la"},
    KeepEnv: []string{"LANG"},
})
```

A denied request is not an error: `Evaluate` returns a result whose `Allowed`
field is false and whose `Reason` explains why. Errors are reserved for
malformed policies and malformed requests.

## Command line

```
go run ./cmd/sudopolicy \
    -policy policy/sudopolicy.conf \
    -user ops -groups ops -host web1 -runas root \
    -- /bin/ls -la
```

The command prints a single line such as `allow runas=root line=6` and exits
non-zero when the command is not allowed.

## Tests

```
go test ./...
```
