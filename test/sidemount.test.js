import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { materialize } from '../src/mount.js';
import { renewOnce } from '../src/renew.js';

const projectRoot = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const cliPath = path.join(projectRoot, 'src', 'cli.js');

function makeTempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sidemount-test-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function startBroker(t, handler) {
  const server = http.createServer(handler);
  t.after(() => new Promise((resolve) => server.close(resolve)));
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => resolve(server));
  });
}

function sendJson(response, status, payload) {
  const body = JSON.stringify(payload);
  response.writeHead(status, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(body),
  });
  response.end(body);
}

async function waitFor(predicate, { timeoutMs = 10_000, intervalMs = 25 } = {}) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }
  throw new Error('timed out waiting for condition');
}

test('materialize writes each secret into the target directory', (t) => {
  const dir = path.join(makeTempDir(t), 'secrets');

  const written = materialize(dir, {
    db_password: 'first-secret',
    api_key: 'second-secret',
  });

  assert.deepEqual(written.sort(), ['api_key', 'db_password']);
  assert.equal(fs.readFileSync(path.join(dir, 'db_password'), 'utf8'), 'first-secret');
  assert.equal(fs.readFileSync(path.join(dir, 'api_key'), 'utf8'), 'second-secret');
});

test('renewOnce replaces the contents of already mounted secrets', async (t) => {
  const dir = makeTempDir(t);
  materialize(dir, { db_password: 'lease-one', api_key: 'lease-one' });

  const broker = {
    async renewLease(leaseId) {
      assert.equal(leaseId, 'lease-1');
      return {
        leaseId: 'lease-2',
        ttlSeconds: 120,
        secrets: { db_password: 'lease-two', api_key: 'lease-two' },
      };
    },
  };

  const session = { dir, broker, leaseId: 'lease-1', leaseSeconds: 60 };
  const written = await renewOnce(session);

  assert.deepEqual(written.sort(), ['api_key', 'db_password']);
  assert.equal(fs.readFileSync(path.join(dir, 'db_password'), 'utf8'), 'lease-two');
  assert.equal(fs.readFileSync(path.join(dir, 'api_key'), 'utf8'), 'lease-two');
  assert.equal(session.leaseId, 'lease-2');
});

test('the sidecar exits once stop arrives on stdin', async (t) => {
  const dir = makeTempDir(t);

  const server = await startBroker(t, (request, response) => {
    const url = new URL(request.url, 'http://127.0.0.1');
    if (request.method === 'GET' && url.pathname === '/v1/secrets/demo-service') {
      sendJson(response, 200, {
        leaseId: 'lease-1',
        ttlSeconds: 600,
        secrets: { db_password: 'stop-test', api_key: 'stop-test' },
      });
      return;
    }
    sendJson(response, 404, { error: 'not found' });
  });

  const { port } = server.address();
  const child = spawn(process.execPath, [cliPath], {
    cwd: projectRoot,
    env: {
      ...process.env,
      SIDEMOUNT_SERVICE: 'demo-service',
      SIDEMOUNT_BROKER_URL: `http://127.0.0.1:${port}`,
      SIDEMOUNT_IDENTITY_TOKEN: 'test-token',
      SIDEMOUNT_SECRETS_DIR: dir,
      SIDEMOUNT_LEASE_SECONDS: '600',
    },
    stdio: ['pipe', 'pipe', 'pipe'],
  });

  t.after(() => {
    if (child.exitCode === null && child.signalCode === null) child.kill();
  });

  let stderr = '';
  child.stderr.on('data', (chunk) => {
    stderr += chunk.toString();
  });

  const exit = new Promise((resolve) => child.once('exit', (code, signal) => resolve({ code, signal })));

  await waitFor(() => fs.existsSync(path.join(dir, 'db_password')));
  assert.equal(fs.readFileSync(path.join(dir, 'db_password'), 'utf8'), 'stop-test');

  child.stdin.write('stop\n');

  const { code, signal } = await exit;
  assert.equal(signal, null, `sidecar terminated by signal; stderr: ${stderr}`);
  assert.equal(code, 0, `sidecar exited with ${code}; stderr: ${stderr}`);
});
