# rulemill

Tenant-authored alert rules are single expressions such as

```
severity == "high" and source.ip in cidr("10.0.0.0/8")
```

Every incoming event is evaluated against each rule for the tenant that owns
it; a truthy verdict marks the event as a hit. Rules are untrusted input, so
the evaluator treats the expression language as a sandbox: only declared
fields can be read, only listed pure functions can be called, and both parsing
and evaluation are bounded.

`docs/rule-language.md` is the specification. It is the contract the evaluator
must satisfy — read it before changing anything under `src/`.

## Layout

| Path | Role |
|---|---|
| `src/lexer.js` | turns rule text into tokens |
| `src/parser.js` | turns tokens into an AST |
| `src/compile.js` | `compileRule(source, { tenantId })`, schema binding, rule cache |
| `src/fields.js` | per-tenant field schemas and `readField` |
| `src/eval.js` | `evaluate(compiled, event)` and the AST interpreter |
| `src/fns.js` | whitelisted pure functions |
| `src/regex.js` | pattern support for `matches` |
| `src/errors.js` | `RuleError` with stable codes |
| `bin/run.js` | evaluates `fixtures/rules/` against `fixtures/events/` |
| `test/` | `node:test` suite |

## Usage

```js
const { compileRule } = require('./src/compile');
const { evaluate } = require('./src/eval');

const rule = compileRule('severity == "high" and source.ip in cidr("10.0.0.0/8")', {
  tenantId: 'acme',
});

evaluate(rule, {
  severity: 'high',
  source: { ip: '10.1.2.3' },
}); // true
```

A compilation failure throws a `RuleError` with a `code` field; a run-time
failure while evaluating an event yields a non-matching verdict.

## Commands

```
node --test        # test suite
node bin/run.js    # fixture walk-through
```

Both use only the Node.js standard library. Node 18 or newer.
