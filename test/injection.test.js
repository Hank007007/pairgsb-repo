'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { createApp } = require('../lib/app');
const { Store, encrypt } = require('../lib/store');

const KEY = Buffer.from('envseal-test-key', 'utf8');

function makeStore() {
  const document = {
    shared: {
      DB_USER: { value: encrypt(KEY, 'webuser') },
    },
  };
  return new Store(document, KEY);
}

function makePipelines() {
  return new Map([
    ['web', { pipeline: 'web', scopes: ['shared'], max_duration_ms: 600000 }],
  ]);
}

function makePolicy() {
  return {
    scopes: [{ name: 'shared', priority: 10 }],
    rules: [
      { effect: 'allow', pipeline: 'web', branch: 'main', scope: 'shared' },
    ],
  };
}

async function withServer(fn) {
  const leasesDir = fs.mkdtempSync(path.join(os.tmpdir(), 'envseal-leases-'));
  const app = createApp({
    policy: makePolicy(),
    pipelines: makePipelines(),
    store: makeStore(),
    leasesDir,
  });

  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const base = `http://127.0.0.1:${port}`;

  try {
    await fn({ base, app, leasesDir });
  } finally {
    await new Promise((resolve) => app.server.close(resolve));
    fs.rmSync(leasesDir, { recursive: true, force: true });
  }
}

test('issues a single-scope lease and writes one variable', async () => {
  await withServer(async ({ base, leasesDir }) => {
    const response = await fetch(`${base}/v1/lease`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        job: 'web-main',
        pipeline: 'web',
        branch: 'main',
        scopes: ['shared'],
        ttl_ms: 60000,
      }),
    });

    assert.equal(response.status, 200);
    const lease = await response.json();
    assert.equal(typeof lease.lease_id, 'string');

    const contents = fs.readFileSync(lease.env_path, 'utf8');
    assert.match(contents, /DB_USER=webuser/);

    const files = fs.readdirSync(leasesDir);
    assert.equal(files.length, 1);
  });
});
