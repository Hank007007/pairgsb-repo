import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { materialize } from '../src/mount.js';
import { safeName } from '../src/names.js';
import { renewOnce } from '../src/renew.js';

const projectRoot = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const cliPath = path.join(projectRoot, 'src', 'cli.js');
const posixTest = process.platform !== 'win32' ? test : test.skip;

function makeTempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sidemount-rules-'));
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

function spawnSidecar(t, dir, port) {
  const child = spawn(process.execPath, [cliPath], {
    cwd: projectRoot,
    env: {
      ...process.env,
      SIDEMOUNT_SERVICE: 'demo-service',
      SIDEMOUNT_BROKER_URL: `http://127.0.0.1:${port}`,
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
  return { child, exit, stderr: () => stderr };
}

test('safeName only admits the documented pattern', () => {
  assert.equal(safeName('db_password'), true);
  assert.equal(safeName('a'), true);
  assert.equal(safeName('a'.repeat(64)), true);

  assert.equal(safeName('.env'), false);
  assert.equal(safeName('foo bar'), false);
  assert.equal(safeName('a'.repeat(70)), false);
  assert.equal(safeName('Foo'), false);
  assert.equal(safeName('1abc'), false);
  assert.equal(safeName('../escape'), false);
  assert.equal(safeName(''), false);
});

test('materialize skips inadmissible names without touching existing files', (t) => {
  const dir = makeTempDir(t);
  const existing = path.join(dir, '.env');
  fs.writeFileSync(existing, 'pre-existing');

  const written = materialize(dir, {
    '.env': 'injected',
    'foo bar': 'injected',
    ['a'.repeat(70)]: 'injected',
    api_key: 'real-secret',
  });

  assert.deepEqual(written, ['api_key']);
  assert.equal(fs.readFileSync(existing, 'utf8'), 'pre-existing');
  assert.equal(fs.existsSync(path.join(dir, 'foo bar')), false);
  assert.equal(fs.existsSync(path.join(dir, 'a'.repeat(70))), false);
  assert.equal(fs.readFileSync(path.join(dir, 'api_key'), 'utf8'), 'real-secret');
});

posixTest('materialize enforces 0700 directory and 0400 file modes', (t) => {
  const dir = path.join(makeTempDir(t), 'secrets');

  materialize(dir, { api_key: 'first' });
  assert.equal(fs.statSync(dir).mode & 0o777, 0o700);
  assert.equal(fs.statSync(path.join(dir, 'api_key')).mode & 0o777, 0o400);

  fs.chmodSync(path.join(dir, 'api_key'), 0o644);
  materialize(dir, { api_key: 'second' });
  assert.equal(fs.statSync(path.join(dir, 'api_key')).mode & 0o777, 0o400);
  assert.equal(fs.readFileSync(path.join(dir, 'api_key'), 'utf8'), 'second');
});

posixTest('materialize refuses to write through a symlink and fails the whole pass', (t) => {
  const dir = makeTempDir(t);
  const outside = path.join(makeTempDir(t), 'outside.txt');
  fs.writeFileSync(outside, 'original');
  fs.symlinkSync(outside, path.join(dir, 'api_key'));

  assert.throws(
    () => materialize(dir, { db_password: 'new-secret', api_key: 'new-secret' }),
    /not a regular file/,
  );

  assert.equal(fs.readFileSync(outside, 'utf8'), 'original');
  assert.equal(fs.lstatSync(path.join(dir, 'api_key')).isSymbolicLink(), true);
  assert.equal(fs.existsSync(path.join(dir, 'db_password')), false);
  assert.deepEqual(fs.readdirSync(dir), ['api_key']);
});

test('renewOnce removes names dropped from the renewed bundle', async (t) => {
  const dir = makeTempDir(t);
  const written = materialize(dir, {
    db_password: 'v1',
    api_key: 'v1',
    legacy_token: 'v1',
  });

  const broker = {
    async renewLease() {
      return { leaseId: 'lease-2', ttlSeconds: 120, secrets: { db_password: 'v2' } };
    },
  };

  const session = { dir, broker, leaseId: 'lease-1', leaseSeconds: 60, mounted: new Set(written) };
  await renewOnce(session);

  assert.equal(fs.readFileSync(path.join(dir, 'db_password'), 'utf8'), 'v2');
  assert.equal(fs.existsSync(path.join(dir, 'legacy_token')), false);
  assert.equal(fs.existsSync(path.join(dir, 'api_key')), false);
  assert.deepEqual([...session.mounted], ['db_password']);
});

test('a failed renewal leaves the previously mounted set intact', async (t) => {
  const dir = makeTempDir(t);
  const written = materialize(dir, { db_password: 'v1', api_key: 'v1' });

  const broker = {
    async renewLease() {
      throw new Error('broker unavailable');
    },
  };

  const session = { dir, broker, leaseId: 'lease-1', leaseSeconds: 60, mounted: new Set(written) };
  await assert.rejects(() => renewOnce(session), /broker unavailable/);

  assert.equal(fs.readFileSync(path.join(dir, 'db_password'), 'utf8'), 'v1');
  assert.equal(fs.readFileSync(path.join(dir, 'api_key'), 'utf8'), 'v1');
  assert.deepEqual([...session.mounted].sort(), ['api_key', 'db_password']);
});

async function runShutdownScenario(t, trigger) {
  const dir = makeTempDir(t);
  const server = await startBroker(t, (request, response) => {
    const url = new URL(request.url, 'http://127.0.0.1');
    if (request.method === 'GET' && url.pathname === '/v1/secrets/demo-service') {
      sendJson(response, 200, {
        leaseId: 'lease-1',
        ttlSeconds: 600,
        secrets: { db_password: 'shutdown-test', api_key: 'shutdown-test' },
      });
      return;
    }
    sendJson(response, 404, { error: 'not found' });
  });

  const { child, exit, stderr } = spawnSidecar(t, dir, server.address().port);
  await waitFor(() => fs.existsSync(path.join(dir, 'db_password')));

  trigger(child);

  const { code, signal } = await exit;
  assert.equal(signal, null, `sidecar terminated by signal; stderr: ${stderr()}`);
  assert.equal(code, 0, `sidecar exited with ${code}; stderr: ${stderr()}`);
  assert.deepEqual(fs.readdirSync(dir), [], 'secrets directory must be empty after shutdown');
}

test('stop removes every mounted file before exiting', async (t) => {
  await runShutdownScenario(t, (child) => child.stdin.write('stop\n'));
});

test('stdin EOF removes every mounted file before exiting', async (t) => {
  await runShutdownScenario(t, (child) => child.stdin.end());
});

posixTest('SIGTERM removes every mounted file before exiting', async (t) => {
  await runShutdownScenario(t, (child) => child.kill('SIGTERM'));
});
