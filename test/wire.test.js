import test from 'node:test';
import assert from 'node:assert/strict';

import { SchemaRegistry } from '../lib/schema/registry.js';
import { resolveType } from '../lib/schema/resolve.js';
import { decodeMessage } from '../lib/wire/decode.js';
import { encodeMessage } from '../lib/wire/encode.js';
import { loadMessage, loadSnapshot } from '../tools/fixtures.js';

function registry() {
  return new SchemaRegistry(loadSnapshot('telemetry.v1.json'));
}

test('a small message round-trips through decode and encode', () => {
  const reg = registry();
  const schema = resolveType(reg, 'Telemetry');
  const bytes = loadMessage('telemetry-basic.bin');

  const message = decodeMessage(bytes, schema);
  assert.equal(message.device_id, 'edge-1');
  assert.equal(message.sequence, 42);
  assert.deepEqual([...message.payload], [0xde, 0xad]);
  assert.equal(message.reading.sensor_id, 's1');
  assert.equal(message.reading.millivolts, 7);

  const again = encodeMessage(message, schema);
  assert.deepEqual(again, bytes);
});

test('decoding a message with no fields yields an empty object', () => {
  const schema = resolveType(registry(), 'Telemetry');
  const message = decodeMessage(loadMessage('telemetry-empty.bin'), schema);
  assert.deepEqual(message, {});
});

test('a nested message is decoded into a nested object', () => {
  const schema = resolveType(registry(), 'Telemetry');
  const message = decodeMessage(loadMessage('telemetry-basic.bin'), schema);
  assert.equal(typeof message.reading, 'object');
  assert.equal(message.reading.sensor_id, 's1');
});
