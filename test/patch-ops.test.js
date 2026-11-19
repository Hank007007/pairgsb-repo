import test from 'node:test';
import assert from 'node:assert/strict';

import { applyPatch } from '../src/index.js';

test('pointer escapes are decoded before use', () => {
  const { document, errors } = applyPatch({ 'a/b': 1, 'til~de': 2 }, [
    { op: 'replace', path: '/a~1b', value: 3 },
    { op: 'replace', path: '/til~0de', value: 4 },
  ]);
  assert.deepEqual(errors, []);
  assert.deepEqual(document, { 'a/b': 3, 'til~de': 4 });
});

test('add appends with the dash segment', () => {
  const { document, errors } = applyPatch({ list: [1, 2] }, [
    { op: 'add', path: '/list/-', value: 3 },
  ]);
  assert.deepEqual(errors, []);
  assert.deepEqual(document.list, [1, 2, 3]);
});

test('add inserts before the addressed element', () => {
  const { document } = applyPatch({ list: ['a', 'c'] }, [
    { op: 'add', path: '/list/1', value: 'b' },
  ]);
  assert.deepEqual(document.list, ['a', 'b', 'c']);
});

test('move removes the source and inserts at the destination', () => {
  const { document, errors } = applyPatch({ a: 1, list: [1, 2, 3] }, [
    { op: 'move', from: '/list/0', path: '/list/2' },
  ]);
  assert.deepEqual(errors, []);
  assert.deepEqual(document.list, [2, 3, 1]);
});

test('copy leaves the source in place', () => {
  const { document } = applyPatch({ a: { b: 1 } }, [
    { op: 'copy', from: '/a', path: '/c' },
  ]);
  assert.deepEqual(document, { a: { b: 1 }, c: { b: 1 } });
  assert.notEqual(document.a, document.c);
});

test('test compares deeply and reports a mismatch', () => {
  const equal = applyPatch({ a: { b: [1, 2] } }, [
    { op: 'test', path: '/a', value: { b: [1, 2] } },
  ]);
  assert.deepEqual(equal.errors, []);

  const unequal = applyPatch({ a: { b: [1, 2] } }, [
    { op: 'test', path: '/a', value: { b: [2, 1] } },
  ]);
  assert.equal(unequal.errors.length, 1);
  assert.equal(unequal.errors[0].code, 'E_PATCH_TEST');
});

test('replace on the empty pointer swaps the whole document', () => {
  const { document, errors } = applyPatch({ a: 1 }, [
    { op: 'replace', path: '', value: { b: 2 } },
  ]);
  assert.deepEqual(errors, []);
  assert.deepEqual(document, { b: 2 });
  assert.equal(Object.getPrototypeOf(document), Object.prototype);
});

test('operations are applied in order', () => {
  const { document } = applyPatch({ n: 0 }, [
    { op: 'replace', path: '/n', value: 1 },
    { op: 'add', path: '/m', value: 2 },
    { op: 'remove', path: '/n' },
  ]);
  assert.deepEqual(document, { m: 2 });
});

test('remove and replace on absent paths report E_PATCH_NOT_FOUND', () => {
  const { errors } = applyPatch({ a: 1 }, [
    { op: 'remove', path: '/missing' },
    { op: 'replace', path: '/alsoMissing', value: 1 },
  ]);
  assert.deepEqual(
    errors.map((entry) => entry.code),
    ['E_PATCH_NOT_FOUND', 'E_PATCH_NOT_FOUND'],
  );
});

test('a relative path is rejected', () => {
  const { errors } = applyPatch({ a: 1 }, [{ op: 'add', path: 'a/b', value: 2 }]);
  assert.equal(errors[0].code, 'E_PATCH_PATH');
});
