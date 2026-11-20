import test from 'node:test';
import assert from 'node:assert/strict';

import { tokenize } from '../src/index.js';

test('tokenizes a start tag with simple attributes', () => {
  const tokens = tokenize('<rect x="1" y="2" fill="red"/>');

  assert.equal(tokens.length, 1);
  const [tag] = tokens;
  assert.equal(tag.type, 'startTag');
  assert.equal(tag.name, 'rect');
  assert.equal(tag.selfClosing, true);
  assert.deepEqual(tag.attrs, [
    { name: 'x', value: '1' },
    { name: 'y', value: '2' },
    { name: 'fill', value: 'red' },
  ]);
});

test('tokenizes text between elements as its own token', () => {
  const tokens = tokenize('<title>Hello</title>');

  assert.deepEqual(
    tokens.map((token) => token.type),
    ['startTag', 'text', 'endTag'],
  );
  assert.equal(tokens[1].raw, 'Hello');
});

test('recognises self-closing syntax', () => {
  const [tag] = tokenize('<circle r="4" />');

  assert.equal(tag.type, 'startTag');
  assert.equal(tag.selfClosing, true);
  assert.equal(tag.attrs.length, 1);
});

test('distinguishes end tags from start tags', () => {
  const [tag] = tokenize('</g>');

  assert.equal(tag.type, 'endTag');
  assert.equal(tag.name, 'g');
});

test('classifies comments and the XML declaration', () => {
  const tokens = tokenize('<?xml version="1.0"?><!-- note --><svg/>');

  assert.equal(tokens[0].type, 'pi');
  assert.equal(tokens[1].type, 'comment');
  assert.equal(tokens[2].type, 'startTag');
});
