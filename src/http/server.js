'use strict';

const http = require('node:http');
const { validateRequest } = require('./gateway');

const DEFAULT_PORT = 8787;

/**
 * Build the HTTP server. Routes:
 *
 *   POST /validate   body: { schema, instance, tenantId? }
 *   GET  /healthz
 *
 * @param {object} [options]
 * @returns {http.Server}
 */
function createServer(options = {}) {
  const tenantSchemas = options.tenantSchemas || new Map();

  return http.createServer(async (req, res) => {
    if (req.method === 'GET' && req.url === '/healthz') {
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
      return;
    }

    if (req.method === 'POST' && req.url === '/validate') {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);

      let payload;
      try {
        payload = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      } catch {
        res.writeHead(400, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ ok: false, errors: [{ path: '', keyword: 'parse', message: 'invalid JSON' }] }));
        return;
      }

      const tenantId = payload.tenantId || req.headers['x-tenant-id'] || null;
      const schema = payload.schema || tenantSchemas.get(tenantId);

      if (!schema) {
        res.writeHead(404, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ ok: false, errors: [{ path: '', keyword: 'tenant', message: 'unknown tenant' }] }));
        return;
      }

      const result = await validateRequest(schema, payload.instance, {
        tenantId,
        fetcher: options.fetcher,
      });

      res.writeHead(result.ok ? 200 : 422, { 'content-type': 'application/json' });
      res.end(JSON.stringify(result));
      return;
    }

    res.writeHead(404, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ ok: false, errors: [{ path: '', keyword: 'route', message: 'not found' }] }));
  });
}

if (require.main === module) {
  const port = Number(process.env.PORT || DEFAULT_PORT);
  createServer().listen(port, () => {
    console.log(`[schemabind] listening on ${port}`);
  });
}

module.exports = { createServer };