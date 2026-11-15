import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { canonicalize } from '../src/canonical.js';
import { macEquals, recordMac } from '../src/hmac.js';
import { Journal, listSegments, readSegment } from '../src/journal.js';
import { verifyChain } from '../src/verify.js';

const KEY = Buffer.from('test-key-do-not-use', 'utf8');

async function withDir(fn) {
  const dir = await mkdtemp(path.join(tmpdir(), 'chainlog-'));
  try {
    return await fn(dir);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}

function sample(overrides = {}) {
  return {
    id: 'evt-1',
    ts: 1762900000,
    actor: 'alice',
    action: 'login',
    target: 'console',
    result: 'ok',
    meta: { ip: '10.0.0.1' },
    ...overrides,
  };
}

test('canonicalize 按固定字段顺序拼接', () => {
  assert.equal(canonicalize({ action: 'login', actor: 'alice' }), 'actor=alice\naction=login');
});

test('canonicalize 跳过缺失字段', () => {
  assert.equal(canonicalize({ id: 'x' }), 'id=x');
});

test('canonicalize 对象字段用 JSON', () => {
  assert.equal(canonicalize({ meta: { ip: '10.0.0.1' } }), 'meta={"ip":"10.0.0.1"}');
});

test('recordMac 对同一记录稳定', () => {
  const a = recordMac(sample(), KEY);
  const b = recordMac(sample(), KEY);
  assert.equal(a, b);
  assert.match(a, /^[0-9a-f]{64}$/);
});

test('recordMac 对改动过的记录不同', () => {
  assert.notEqual(recordMac(sample(), KEY), recordMac(sample({ action: 'delete' }), KEY));
});

test('macEquals 对相同值返回 true', () => {
  const mac = recordMac(sample(), KEY);
  assert.equal(macEquals(mac, mac), true);
});

test('macEquals 对不等长返回 false', () => {
  assert.equal(macEquals('ab', 'abcd'), false);
});

test('append 之后能读回来', () =>
  withDir(async (dir) => {
    const journal = new Journal({ dir, maxEntries: 100, key: KEY });
    await journal.open();
    const entry = await journal.append(sample());
    assert.equal(entry.seq, 1);
    assert.equal(entry.segment, 0);

    const files = await listSegments(dir);
    assert.equal(files.length, 1);
    const entries = await readSegment(files[0]);
    assert.equal(entries.length, 1);
    assert.equal(entries[0].id, 'evt-1');
  }));

test('未改动的单段日志校验通过', () =>
  withDir(async (dir) => {
    const journal = new Journal({ dir, maxEntries: 100, key: KEY });
    await journal.open();
    await journal.append(sample());
    await journal.append(sample({ id: 'evt-2' }));

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, true);
    assert.equal(result.segments.length, 1);
  }));

test('删掉中间一条会被发现', () =>
  withDir(async (dir) => {
    const journal = new Journal({ dir, maxEntries: 100, key: KEY });
    await journal.open();
    await journal.append(sample({ id: 'evt-1' }));
    await journal.append(sample({ id: 'evt-2' }));
    await journal.append(sample({ id: 'evt-3' }));

    const files = await listSegments(dir);
    const entries = await readSegment(files[0]);
    const kept = [entries[0], entries[2]];
    await writeFile(files[0], `${kept.map((e) => JSON.stringify(e)).join('\n')}\n`, 'utf8');

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[0].reason, 'chain-link');
  }));

test('manifest 记录当前写入位置', () =>
  withDir(async (dir) => {
    const journal = new Journal({ dir, maxEntries: 100, key: KEY });
    await journal.open();
    await journal.append(sample());

    const manifest = JSON.parse(await readFile(path.join(dir, 'manifest.json'), 'utf8'));
    assert.equal(manifest.seq, 1);
    assert.equal(manifest.count, 1);
    assert.equal(manifest.segment, 0);
  }));

test('重新 open 之后 seq 接着走', () =>
  withDir(async (dir) => {
    const first = new Journal({ dir, maxEntries: 100, key: KEY });
    await first.open();
    await first.append(sample());

    const second = new Journal({ dir, maxEntries: 100, key: KEY });
    await second.open();
    const entry = await second.append(sample({ id: 'evt-2' }));
    assert.equal(entry.seq, 2);
  }));
