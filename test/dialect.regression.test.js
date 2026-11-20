'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const { sniff, consumeHint } = require('../src/dialect');

function fixture(name) {
  return fs.readFileSync(path.join(__dirname, '..', 'fixtures', name));
}

test('a .tsv file is split on tabs even when notes contain commas', () => {
  const dialect = sniff(fixture('commas-in-notes.tsv'), {
    filename: 'commas-in-notes.tsv',
  });
  assert.equal(dialect.delimiter, '\t');
  assert.equal(dialect.error, null);
});

test('the UTF-8 BOM is removed from the decoded text', () => {
  const dialect = sniff(fixture('bom.csv'), { filename: 'bom.csv' });
  assert.equal(dialect.bom, true);
  assert.equal(dialect.text.startsWith('\uFEFF'), false);
  assert.equal(dialect.text.slice(0, 4), 'city');
});

test('a leading sep= hint is consumed and outranks the extension', () => {
  const dialect = sniff(fixture('hint-semicolon.csv'), {
    filename: 'hint-semicolon.csv',
  });
  assert.equal(dialect.hint, ';');
  assert.equal(dialect.delimiter, ';');
  assert.equal(dialect.text.startsWith('sep='), false);
  assert.equal(dialect.text.split(/\r\n|\r|\n/)[0], 'region;owner;quarter;booked');
});

test('a sep= hint after the first line is ordinary data', () => {
  const text = 'a,b\nsep=;\n1,2\n';
  const dialect = sniff(text, { filename: 'x.csv' });
  assert.equal(dialect.hint, null);
  assert.equal(dialect.delimiter, ',');
  assert.equal(dialect.text, text);
});

test('content structure overrides a wrong extension', () => {
  const dialect = sniff(Buffer.from('a;b\n1;2\n3;4\n'), {
    filename: 'wrong.csv',
  });
  assert.equal(dialect.delimiter, ';');
});

test('the declared delimiter wins when the evidence is ambiguous', () => {
  const dialect = sniff(Buffer.from('a\tb\n1\t2\n'), {
    filename: 'x.tsv',
  });
  assert.equal(dialect.delimiter, '\t');
});

test('content with no decodable structure is a fatal dialect error', () => {
  const dialect = sniff('plain header\nsecond row\n', { filename: 'x.dat' });
  assert.equal(dialect.error.code, 'fatal_undecodable_table');
  assert.equal(dialect.error.row, 0);
  assert.equal(dialect.error.column, 0);
});

test('semicolon files using lone CR line endings are recognised', () => {
  const dialect = sniff(fixture('cr-semicolon.csv'), {
    filename: 'cr-semicolon.csv',
  });
  assert.equal(dialect.delimiter, ';');
});

test('the hint parser tolerates surrounding whitespace and casing', () => {
  assert.deepEqual(consumeHint(' SEP=; \r\nrest'), {
    text: 'rest',
    hint: ';',
  });
  assert.deepEqual(consumeHint('sep=|\na,b'), {
    text: 'sep=|\na,b',
    hint: null,
  });
});
