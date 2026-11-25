import test from 'node:test';
import assert from 'node:assert/strict';
import { readVarint, writeVarint, varintSize } from '../lib/wire/varint.js';

test('readVarint decodes a small single-byte value', () => {
  const { value, next } = readVarint(Buffer.from([0x08, 0xff]));
  assert.equal(value, 8n);
  assert.equal(next, 1);
});

test('readVarint decodes a multi-byte value', () => {
  const { value, next } = readVarint(Buffer.from([0xac, 0x02]));
  assert.equal(value, 300n);
  assert.equal(next, 2);
});

test('writeVarint round-trips small values', () => {
  for (const value of [0, 1, 127, 128, 300, 16384]) {
    const encoded = writeVarint(value);
    const { value: decoded, next } = readVarint(encoded);
    assert.equal(decoded, BigInt(value));
    assert.equal(next, encoded.length);
  }
});

test('writeVarint emits the shortest encoding', () => {
  assert.deepEqual([...writeVarint(0)], [0x00]);
  assert.deepEqual([...writeVarint(1)], [0x01]);
  assert.deepEqual([...writeVarint(127)], [0x7f]);
  assert.deepEqual([...writeVarint(128)], [0x80, 0x01]);
  assert.deepEqual([...writeVarint(300)], [0xac, 0x02]);
});

test('writeVarint accepts bigint values', () => {
  const encoded = writeVarint(2n ** 35n);
  const { value, next } = readVarint(encoded);
  assert.equal(value, 2n ** 35n);
  assert.equal(next, encoded.length);
});

test('varintSize reports the length of the shortest encoding', () => {
  assert.equal(varintSize(0n), 1);
  assert.equal(varintSize(127n), 1);
  assert.equal(varintSize(128n), 2);
  assert.equal(varintSize(2n ** 63n), 10);
});
