# tupleresolve

tupleresolve is the relation-based authorization (ReBAC) evaluation library
shared by the internal code host and the document system. It evaluates
Zanzibar-style namespace configurations against tuples of the form
`object#relation@subject` and answers three questions:

* `check(object, relation, subject)` — is this subject a member?
* `expand(object, relation)` — who are the members?
* `write` / `delete` — mutate tuples, returning a zookie.

Reads can carry an `at_least` zookie to require a snapshot no older than a
given mutation, and every request names a tenant whose tuple space and
namespace registry are private.

## Running

Node.js 20 or newer. There are no runtime dependencies.

```sh
node --test          # the test suite
node bin/serve.js    # the HTTP service on $PORT (default 8080)
```

## Layout

```
docs/            specification (model, evaluation, consistency, tenancy) and design notes
src/model/       namespace registry and compilation
src/graph/       check and expand evaluators
src/store/       tuple storage and zookie / snapshot bookkeeping
src/cache/       decision cache
src/http/        HTTP handlers and server
test/            node:test suite and namespace fixtures
bin/             service entry point
```

## API

```js
import { createRuntime, createService } from './src/index.js';

const runtime = createRuntime();
const service = createService(runtime, 'acme');

service.registerNamespace({ name: 'doc', version: 1, relations: { reader: { this: {} } } });
const { zookie } = service.write('doc:1#reader@user:alice');

service.check('doc:1', 'reader', 'user:alice');                  // true
service.check('doc:1', 'reader', 'user:alice', { at_least: zookie });
service.expand('doc:1', 'reader');                                // { subjects: ['user:alice'], ... }
```

Over HTTP the same operations are exposed by `bin/serve.js` as
`POST /check`, `POST /expand`, `POST /tuples`, `POST /tuples/delete`,
`POST /namespaces` and `GET /namespaces`. Requests are authenticated by a
principal that carries a tenant; the service never trusts a body-supplied
tenant that disagrees with it.

## Semantics

The authoritative description of rewrite semantics, evaluation, consistency
and tenancy is in `docs/`:

* [`docs/model.md`](docs/model.md) — namespaces, rewrite operators, tuples,
  zookies, the registry.
* [`docs/evaluation.md`](docs/evaluation.md) — `check`, `expand`, recursion
  depth, cycle detection, contextual tuples.
* [`docs/consistency.md`](docs/consistency.md) — zookie advancement,
  `at_least` reads, decision caching.
* [`docs/tenancy.md`](docs/tenancy.md) — per-tenant tuple spaces, namespace
  ownership, cross-tenant visibility.
* [`docs/design.md`](docs/design.md) — how the implementation is layered and
  which layer owns which decision.
