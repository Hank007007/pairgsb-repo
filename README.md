# cspfence

cspfence compiles a tenant's CSP policy into the response headers the CDN edge
writes. It is the layer that decides whether a tenant policy may relax
protection, and that keeps a tenant-supplied string from breaking out of the
response header.

## Pipeline

```
tenant text / JSON
        │
        ▼
   tokenize()            src/policy/tokenize.mjs
        │
        ▼
   parse_policy()        src/policy/parse.mjs
        │                validate_source_expression() lives here
        ▼
 merge_with_baseline()   src/policy/merge.mjs
        │                baseline: src/platform/profile.mjs
        ▼
   emit_headers()        src/http/emit.mjs
        │                Content-Security-Policy
        │                Reporting-Endpoints
        ▼
 resolve_endpoint()      src/report/endpoint.mjs
                         registry: src/platform/registry.mjs
```

### Layer responsibilities

| layer | takes | produces | does not |
|---|---|---|---|
| tokenize | policy text | flat tokens, each carrying whether a directive ends after it | validate anything |
| parse_policy | text or JSON | `{ directive: [source, ...] }` | merge with the baseline |
| merge_with_baseline | parsed policy | merged policy | serialize |
| emit_headers | parsed policy | response headers | re-derive the policy |
| resolve_endpoint | tenant URL | URL safe to put in a header | write headers |

## Documents

* `docs/design.md` — what each layer is responsible for and how the stages feed
  each other.
* `docs/policy-grammar.md` — tokens, directives, source expressions, errors.
* `docs/merge.md` — how the platform baseline and a tenant policy combine.
* `docs/serialize.md` — directive order, header size and character limits.
* `docs/reports.md` — which reporting endpoints are acceptable.

These documents are the contract for this layer: they define tokens,
directives, merging, serialization and reporting.

## Running

```
node bin/csp.js inline-two.policy
node bin/csp.js tab-separated.policy --baseline
node --test
```

`bin/csp.js` reads `fixtures/policies/<name>`, prints the parsed policy, the
policy merged with the baseline, and the headers that would be written.

## Layout

```
bin/csp.js                   command line entry point
src/policy/syntax.mjs        shared keywords, directive order, size limit
src/policy/error.mjs         PolicyError
src/policy/tokenize.mjs      text → tokens
src/policy/parse.mjs         tokens/JSON → directive table, source validation
src/policy/merge.mjs         tenant policy + baseline
src/http/emit.mjs            policy → headers
src/report/endpoint.mjs      reporting endpoint resolution
src/platform/profile.mjs     baseline policy, default collector, locked directives
src/platform/registry.mjs    tenant → registered domains
fixtures/policies/           sample tenant policies
test/                        node:test suites
```

The implementation uses only Node built-in modules.
