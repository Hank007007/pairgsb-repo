'use strict';

const { createServer, listen } = require('../src/server');
const { issueToken } = require('../src/auth/token');
const { seed } = require('../src/seed');
const FULL_SCOPES = [
  'attr:profile:read',
  'attr:profile:write',
  'attr:groups:read',
  'attr:groups:write',
  'attr:roles:read',
  'attr:roles:write',
  'attr:active:read',
  'attr:active:write',
];

async function startServer(options = {}) {
  const { server, app } = createServer(options);
  if (options.seed !== false) {
    seed(app);
  }
  const address = await listen(server, 0);
  const base = `http://127.0.0.1:${address.port}`;
  return {
    server,
    app,
    base,
    async close() {
      await new Promise((resolve) => server.close(resolve));
    },
  };
}

function tokenFor(tenant, scopes = FULL_SCOPES, options = {}) {
  return issueToken({ sub: options.sub || `client-${tenant}`, tenant, scopes });
}

async function request(base, method, path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (options.token) {
    headers.authorization = `Bearer ${options.token}`;
  }
  let body;
  if (options.body !== undefined) {
    headers['content-type'] = 'application/scim+json';
    body = typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
  }
  const response = await fetch(`${base}${path}`, { method, headers, body });
  const text = await response.text();
  let json = null;
  if (text.length > 0) {
    try {
      json = JSON.parse(text);
    } catch (err) {
      json = null;
    }
  }
  return {
    status: response.status,
    headers: response.headers,
    body: json,
    text,
  };
}

module.exports = { startServer, tokenFor, request, FULL_SCOPES };
