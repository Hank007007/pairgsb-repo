#!/usr/bin/env node
/**
 * HTTP entry point.
 *
 * The demo principal reader trusts a `x-tenant` header (or the `TUPLERESOLVE_TENANT`
 * environment variable as a fallback). Replace `authenticate` with the real
 * principal resolution when embedding the server.
 */

import { createServer } from '../src/http/server.js';
import { createRuntime } from '../src/runtime.js';
import { createService } from '../src/service.js';

const runtime = createRuntime();

function authenticate(request) {
  const header = request.headers['x-tenant'];
  const tenant = (typeof header === 'string' && header) || process.env.TUPLERESOLVE_TENANT;
  if (!tenant) return null;
  return { tenant };
}

const server = createServer({
  serviceFor: (principal) => createService(runtime, principal.tenant),
  authenticate,
});

const port = Number(process.env.PORT ?? 8080);
server.listen(port, () => {
  process.stdout.write(`tupleresolve listening on ${port}\n`);
});
