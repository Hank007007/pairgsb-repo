import test from 'node:test';
import assert from 'node:assert/strict';

import { filterAttr } from '../src/index.js';

test('keeps an allowlisted numeric attribute with a valid length', () => {
  assert.deepEqual(filterAttr('width', '64', 'rect'), { name: 'width', value: '64' });
  assert.deepEqual(filterAttr('width', '2.5em', 'rect'), { name: 'width', value: '2.5em' });
});

test('drops an allowlisted numeric attribute with a non-numeric value', () => {
  assert.equal(filterAttr('width', 'wide', 'rect'), null);
});

test('keeps a valid paint value and a local reference', () => {
  assert.deepEqual(filterAttr('fill', '#4a90d9', 'rect'), { name: 'fill', value: '#4a90d9' });
  assert.deepEqual(filterAttr('fill', 'url(#bg)', 'rect'), { name: 'fill', value: 'url(#bg)' });
});

test('keeps an https reference on image', () => {
  assert.deepEqual(filterAttr('href', 'https://cdn.example.com/a.png', 'image'), {
    name: 'href',
    value: 'https://cdn.example.com/a.png',
  });
});

test('keeps the element-specific id and class attributes', () => {
  assert.deepEqual(filterAttr('id', 'gradient-1', 'linearGradient'), {
    name: 'id',
    value: 'gradient-1',
  });
  assert.deepEqual(filterAttr('class', 'bg layer', 'rect'), { name: 'class', value: 'bg layer' });
});

test('keeps path data unchanged', () => {
  const d = 'M12 64 C12 46 52 46 52 64 Z';
  assert.deepEqual(filterAttr('d', d, 'path'), { name: 'd', value: d });
});
