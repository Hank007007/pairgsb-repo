import assert from 'node:assert/strict';
import test from 'node:test';

import { TtlCache } from '../src/cache.js';

function makeClock(start = 0) {
  let now = start;
  return {
    now: () => now,
    advance: (ms) => {
      now += ms;
    },
  };
}

test('a missing key reads as undefined', () => {
  const cache = new TtlCache({ ttlMs: 1_000 });
  assert.equal(cache.get('absent'), undefined);
  assert.equal(cache.size, 0);
});

test('values round trip through the cache and can be removed', () => {
  const cache = new TtlCache({ ttlMs: 1_000 });
  const value = { allow: true, matchedPolicy: 'rule-a', reason: 'matched' };
  cache.set('k', value);
  cache.set('other', 1);
  assert.deepEqual(cache.get('k'), value);
  assert.equal(cache.delete('other'), true);
  assert.equal(cache.delete('other'), false);
  cache.clear();
  assert.equal(cache.size, 0);
});

test('entries expire once the ttl elapses', () => {
  const clock = makeClock();
  const cache = new TtlCache({ ttlMs: 500, clock: clock.now });
  cache.set('k', 'v');

  clock.advance(499);
  assert.equal(cache.get('k'), 'v');

  clock.advance(2);
  assert.equal(cache.get('k'), undefined);
  assert.equal(cache.size, 0);
});

test('a per-entry ttl overrides the default', () => {
  const clock = makeClock();
  const cache = new TtlCache({ ttlMs: 500, clock: clock.now });
  cache.set('short', 'v', 100);
  cache.set('long', 'v', 400);

  clock.advance(200);
  assert.equal(cache.get('short'), undefined);
  assert.equal(cache.get('long'), 'v');
});

test('the cache evicts the oldest entry past its size cap', () => {
  const cache = new TtlCache({ ttlMs: 1_000, maxEntries: 2 });
  cache.set('a', 1);
  cache.set('b', 2);
  cache.set('c', 3);
  assert.equal(cache.size, 2);
  assert.equal(cache.get('a'), undefined);
  assert.equal(cache.get('b'), 2);
  assert.equal(cache.get('c'), 3);
});
