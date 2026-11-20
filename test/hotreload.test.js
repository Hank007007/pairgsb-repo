import test from 'node:test';
import assert from 'node:assert/strict';

import { compileBundle, createBundleStore, watchBundle, loadBundle } from '../src/bundle.js';

const ONE = { id: 'a', host: 'a.example.net', scheme: 'bearer', value: 'tok_a' };
const TWO = { id: 'b', host: 'b.example.net', scheme: 'api-key', value: 'key_b' };

test('explicit reload advances the revision and swaps the store', async () => {
  const initial = compileBundle({ secrets: [ONE] }, 1);
  const store = createBundleStore(initial);
  let text = JSON.stringify({ secrets: [ONE, TWO] });

  const watcher = watchBundle({
    path: '<memory>',
    store,
    reader: async () => text,
    debounceMs: 1,
  });

  const next = await watcher.reload();
  assert.equal(next.revision, 2);
  assert.equal(store.current.revision, 2);
  assert.equal(store.current.entries.length, 2);

  text = JSON.stringify({ secrets: [TWO] });
  const third = await watcher.reload();
  assert.equal(third.revision, 3);
  assert.equal(store.current.entries[0].id, 'b');

  watcher.close();
});

test('a failed reload leaves the previous revision active', async () => {
  const initial = compileBundle({ secrets: [ONE] }, 4);
  const store = createBundleStore(initial);
  let text = JSON.stringify({ secrets: [ONE] });
  const watcher = watchBundle({
    path: '<memory>',
    store,
    reader: async () => text,
    debounceMs: 1,
  });

  text = '{ this is not json';
  await assert.rejects(watcher.reload());
  assert.equal(store.current.revision, 4);
  assert.equal(store.current.entries[0].id, 'a');

  watcher.close();
});

test('concurrent reloads share a single in-flight load', async () => {
  const initial = compileBundle({ secrets: [ONE] }, 1);
  const store = createBundleStore(initial);
  let reads = 0;
  const watcher = watchBundle({
    path: '<memory>',
    store,
    reader: async () => {
      reads += 1;
      return JSON.stringify({ secrets: [ONE] });
    },
    debounceMs: 1,
  });

  const [a, b] = await Promise.all([watcher.reload(), watcher.reload()]);
  assert.equal(a, b);
  assert.equal(reads, 1);

  watcher.close();
});

test('loadBundle exposes the source path', async () => {
  const bundle = await loadBundle('/virtual/x.json', 3, async () => JSON.stringify({ secrets: [ONE] }));
  assert.equal(bundle.source, '/virtual/x.json');
  assert.equal(bundle.revision, 3);
});
