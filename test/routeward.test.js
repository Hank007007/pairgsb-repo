'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { createServer } = require('../src/server');

function startServer() {
  const server = createServer();
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address();
      resolve({ server, base: `http://127.0.0.1:${port}` });
    });
  });
}

function stopServer(server) {
  return new Promise((resolve) => server.close(resolve));
}

function request(base, pathname, { method = 'GET', cookie = '' } = {}) {
  const headers = {};
  if (cookie) {
    headers.cookie = cookie;
  }
  return fetch(base + pathname, { method, headers });
}

test('authorized request reaches the business handler', async () => {
  const { server, base } = await startServer();
  try {
    const res = await request(base, '/admin/users', {
      cookie: 'rw_session=sess_bob',
    });

    assert.equal(res.status, 200);
    const body = await res.json();
    assert.equal(body.handler, 'admin.users.list');
    assert.equal(body.tenant, 'acme');
  } finally {
    await stopServer(server);
  }
});

test('request without the required scope is rejected', async () => {
  const { server, base } = await startServer();
  try {
    const res = await request(base, '/admin/users', {
      cookie: 'rw_session=sess_dave',
    });

    assert.equal(res.status, 403);
    const body = await res.json();
    assert.equal(body.error, 'forbidden');
    assert.ok(body.missing.includes('admin'));
  } finally {
    await stopServer(server);
  }
});
