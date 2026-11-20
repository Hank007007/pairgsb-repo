'use strict';

const http = require('node:http');

const { createGrantService } = require('./index');
const { PolicyError, createPolicyRegistry } = require('./policy/tenant');
const { PathError } = require('./pathnorm/norm');
const { createRevocationStore, defaultRevocationPath } = require('./store/revoke');

const DEMO_POLICIES = [
  {
    tenantId: 'tenant-a',
    root: '/tenant-a',
    adminActions: ['read', 'write'],
    defaultTtlMs: 60_000,
    maxTtlMs: 3_600_000,
  },
  {
    tenantId: 'tenant-b',
    root: '/tenant-b',
    adminActions: ['read'],
    defaultTtlMs: 60_000,
    maxTtlMs: 900_000,
  },
];

const MAX_BODY_BYTES = 64 * 1024;

function readJsonBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(new Error('request body too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      if (chunks.length === 0) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')));
      } catch {
        reject(new Error('request body is not valid JSON'));
      }
    });
    req.on('error', reject);
  });
}

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(body),
  });
  res.end(body);
}

function statusForError(error) {
  if (error instanceof PathError || error instanceof PolicyError) {
    return 400;
  }
  if (error instanceof TypeError) {
    return 400;
  }
  return 500;
}

/**
 * Build the HTTP surface of the service.
 *
 * @param {{
 *   policies: { get(tenantId: string): object | undefined },
 *   revocationFile?: string,
 *   now?: () => number,
 * }} options
 * @returns {import('node:http').Server}
 */
function createServer(options) {
  const store = createRevocationStore({ filePath: options.revocationFile });
  const service = createGrantService({
    policies: options.policies,
    store,
    now: options.now,
  });

  return http.createServer(async (req, res) => {
    try {
      const url = new URL(req.url, 'http://localhost');

      if (req.method === 'POST' && url.pathname === '/grants') {
        const body = await readJsonBody(req);
        const result = service.issue({
          tenantId: body.tenantId,
          path: body.path,
          actions: body.actions,
          ttlMs: body.ttlMs,
        });
        sendJson(res, 201, { token: result.token, grant: result.grant });
        return;
      }

      if (req.method === 'POST' && url.pathname === '/verify') {
        const body = await readJsonBody(req);
        const result = service.verify(body.token, {
          action: body.action,
          requestPath: body.path,
          tenantId: body.tenantId,
        });
        sendJson(res, result.ok ? 200 : 403, result);
        return;
      }

      if (req.method === 'POST' && url.pathname === '/revocations') {
        const body = await readJsonBody(req);
        service.revoke(body.tenantId, body.id);
        res.writeHead(204);
        res.end();
        return;
      }

      sendJson(res, 404, { error: 'not found' });
    } catch (error) {
      sendJson(res, statusForError(error), { error: error.message });
    }
  });
}

function main() {
  const policies = createPolicyRegistry(DEMO_POLICIES);
  const server = createServer({
    policies,
    revocationFile: defaultRevocationPath(),
  });
  const port = Number(process.env.PORT ?? 8080);
  server.listen(port, () => {
    process.stdout.write(`tenantmint listening on http://127.0.0.1:${port}\n`);
  });
}

if (require.main === module) {
  main();
}

module.exports = { DEMO_POLICIES, createServer };
