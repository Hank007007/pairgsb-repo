'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { sniff, declaredDelimiter } = require('../src/dialect');

test('a .csv file without other evidence uses commas', () => {
  const result = sniff('name,email\nAda,ada@example.com\n', { filename: 'list.csv' });
  assert.equal(result.delimiter, ',');
});

test('a .tsv file without other evidence uses tabs', () => {
  const result = sniff('name\temail\nAda\tada@example.com\n', { filename: 'list.tsv' });
  assert.equal(result.delimiter, '\t');
});

test('the filename extension decides the declared delimiter', () => {
  assert.equal(declaredDelimiter('export.CSV', ''), ',');
  assert.equal(declaredDelimiter('export.tsv', ''), '\t');
  assert.equal(declaredDelimiter('export.xlsx', ''), null);
});

test('the media type is used when the filename carries no extension', () => {
  assert.equal(declaredDelimiter('download', 'text/tab-separated-values'), '\t');
  assert.equal(declaredDelimiter('download', 'text/csv'), ',');
});

test('sniff accepts a Buffer', () => {
  const result = sniff(Buffer.from('a\tb\n1\t2\n'), { filename: 'x.tsv' });
  assert.equal(result.delimiter, '\t');
});

test('sniff returns the decoded text unchanged', () => {
  const text = 'name,email\nAda,ada@example.com\n';
  assert.equal(sniff(text, { filename: 'list.csv' }).text, text);
});
