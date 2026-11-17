import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { chainDigest, recordMac } from '../src/hmac.js';
import { Journal, listSegments, readSegment } from '../src/journal.js';
import { verifyChain } from '../src/verify.js';

const KEY = Buffer.from('test-key-do-not-use', 'utf8');
const ZERO_MAC = '0'.repeat(64);

async function withDir(fn) {
  const dir = await mkdtemp(path.join(tmpdir(), 'chainlog-verify-'));
  try {
    return await fn(dir);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}

function sample(id, overrides = {}) {
  return {
    id,
    ts: 1762900000,
    actor: 'alice',
    action: 'read',
    target: 'console',
    result: 'ok',
    meta: { ip: '10.0.0.1' },
    ...overrides,
  };
}

async function appendMany(dir, count, maxEntries = 100) {
  const journal = new Journal({ dir, maxEntries, key: KEY });
  await journal.open();
  for (let i = 1; i <= count; i += 1) {
    await journal.append(sample(`evt-${i}`));
  }
}

async function writeEntries(file, entries) {
  await writeFile(file, `${entries.map((entry) => JSON.stringify(entry)).join('\n')}\n`, 'utf8');
}

test('chainDigest 覆盖段号、条数和末条 mac', () => {
  const mac = 'a'.repeat(64);
  assert.match(chainDigest(0, 1, mac, KEY), /^[0-9a-f]{64}$/);
  assert.notEqual(chainDigest(0, 1, mac, KEY), chainDigest(1, 1, mac, KEY));
  assert.notEqual(chainDigest(0, 1, mac, KEY), chainDigest(0, 2, mac, KEY));
  assert.notEqual(chainDigest(0, 1, mac, KEY), chainDigest(0, 1, 'b'.repeat(64), KEY));
});

test('改业务字段报 record-mac', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3);
    const [file] = await listSegments(dir);
    const entries = await readSegment(file);
    entries[1].action = 'delete';
    await writeEntries(file, entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[0].reason, 'record-mac');
    assert.deepEqual(result.broken, { file, seq: 2, reason: 'record-mac' });
  }));

test('改 seq 或 segment 报 record-mac', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3);
    const [file] = await listSegments(dir);

    for (const [field, value] of [
      ['seq', 99],
      ['segment', 1],
    ]) {
      const entries = await readSegment(file);
      entries[1][field] = value;
      await writeEntries(file, entries);

      const result = await verifyChain(dir, KEY);
      assert.equal(result.ok, false);
      assert.equal(result.segments[0].reason, 'record-mac');
    }
  }));

test('改 prevMac 报 record-mac 或 chain-link', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3);
    const [file] = await listSegments(dir);
    const entries = await readSegment(file);
    entries[1].prevMac = ZERO_MAC;
    await writeEntries(file, entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.ok(['record-mac', 'chain-link'].includes(result.segments[0].reason));
  }));

test('删掉段中间一条报 chain-link', () =>
  withDir(async (dir) => {
    await appendMany(dir, 4);
    const [file] = await listSegments(dir);
    const entries = await readSegment(file);
    entries.splice(1, 1);
    await writeEntries(file, entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[0].reason, 'chain-link');
  }));

test('删掉某段最后一条报 segment-digest', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3);
    const [file] = await listSegments(dir);
    const entries = await readSegment(file);
    entries.pop();
    await writeEntries(file, entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[0].reason, 'segment-digest');
  }));

test('删掉段末条时后续段仍会给出结论', () =>
  withDir(async (dir) => {
    await appendMany(dir, 4, 2);
    const files = await listSegments(dir);
    const entries = await readSegment(files[0]);
    entries.pop();
    await writeEntries(files[0], entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments.length, files.length);
    assert.equal(result.segments[0].reason, 'segment-digest');
    assert.equal(result.segments[1].reason, 'chain-link');
    assert.equal(result.broken.reason, 'segment-digest');
  }));

test('整段替换成另一份同长度内容报 segment-digest 或 chain-link', () =>
  withDir(async (dir) => {
    await appendMany(dir, 6, 2);
    await withDir(async (donorDir) => {
      const donor = new Journal({ dir: donorDir, maxEntries: 2, key: KEY });
      await donor.open();
      for (let i = 1; i <= 6; i += 1) {
        await donor.append(sample(`other-${i}`, { actor: 'bob' }));
      }

      const targetFiles = await listSegments(dir);
      const donorFiles = await listSegments(donorDir);
      await writeFile(targetFiles[2], await readFile(donorFiles[2], 'utf8'), 'utf8');

      const result = await verifyChain(dir, KEY);
      assert.equal(result.ok, false);
      assert.ok(['segment-digest', 'chain-link'].includes(result.segments[2].reason));
    });
  }));

test('在某段末尾追加一条报 segment-digest', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3);
    const [file] = await listSegments(dir);
    const entries = await readSegment(file);
    const last = entries[entries.length - 1];
    const body = {
      ...sample('forged'),
      seq: last.seq + 1,
      segment: last.segment,
      prevMac: last.mac,
    };
    const forged = { ...body, mac: recordMac(body, KEY) };
    await writeEntries(file, [...entries, forged]);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[0].reason, 'segment-digest');
  }));

test('删掉整个中间段文件报 segment-link', () =>
  withDir(async (dir) => {
    await appendMany(dir, 6, 2);
    const files = await listSegments(dir);
    await rm(files[1]);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments.length, 2);
    assert.equal(result.segments[1].reason, 'segment-link');
  }));

test('删掉段文件后重写 manifest 仍然报错', () =>
  withDir(async (dir) => {
    await appendMany(dir, 6, 2);
    const files = await listSegments(dir);
    await rm(files[1]);

    const lastEntries = await readSegment(files[2]);
    const last = lastEntries[lastEntries.length - 1];
    await writeFile(
      path.join(dir, 'manifest.json'),
      JSON.stringify({
        segment: 2,
        seq: last.seq,
        prevMac: last.mac,
        count: lastEntries.length,
      }),
      'utf8',
    );

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments[1].reason, 'segment-link');
  }));

test('open 不使用被篡改的 manifest 决定写入位置', () =>
  withDir(async (dir) => {
    await appendMany(dir, 3, 2);
    await writeFile(
      path.join(dir, 'manifest.json'),
      JSON.stringify({ segment: 0, seq: 0, prevMac: ZERO_MAC, count: 0 }),
      'utf8',
    );

    const journal = new Journal({ dir, maxEntries: 2, key: KEY });
    await journal.open();
    const entry = await journal.append(sample('evt-4'));

    assert.equal(entry.seq, 4);
    assert.equal(entry.segment, 1);
    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, true);
  }));

test('断链后继续校验后续每一段', () =>
  withDir(async (dir) => {
    await appendMany(dir, 10, 2);
    const files = await listSegments(dir);
    const entries = await readSegment(files[1]);
    entries[0].action = 'delete';
    await writeEntries(files[1], entries);

    const result = await verifyChain(dir, KEY);
    assert.equal(result.ok, false);
    assert.equal(result.segments.length, files.length);
    assert.equal(result.segments[1].ok, false);
    assert.equal(result.segments[1].reason, 'record-mac');
    assert.equal(result.segments[0].ok, true);
    assert.equal(result.segments[2].ok, true);
    assert.equal(result.segments[3].ok, true);
    assert.equal(result.segments[4].ok, true);
    assert.equal(result.broken.reason, 'record-mac');
  }));
