'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const { parseSheet } = require('../src');

function fixture(name) {
  return fs.readFileSync(path.join(__dirname, '..', 'fixtures', name));
}

test('embedded newlines and doubled quotes decode into complete records', () => {
  for (const name of [
    'embedded-newline.csv',
    'notes.csv',
    'quoted-quotes.csv',
    'crlf-in-quotes.csv',
  ]) {
    const result = parseSheet(fixture(name), { filename: name });
    assert.equal(result.error, null, `${name}: ${result.error && result.error.message}`);
    assert.ok(result.rows.length > 0, name);
  }
});

test('every failing fixture returns a fatal code and zero rows', () => {
  const failing = [
    'duplicate-header.csv',
    'empty-header.csv',
    'escaped-quote.csv',
    'invalid-date.csv',
    'numeric-formula.csv',
    'oversized-cell.csv',
    'post-quote-junk.csv',
    'too-many-fields.csv',
    'unbalanced-quote.csv',
  ];

  for (const name of failing) {
    const result = parseSheet(fixture(name), {
      filename: name,
      types: { amount: 'number', booked: 'number', renewal: 'date' },
    });
    assert.match(result.error.code, /^fatal/, name);
    assert.deepEqual(result.rows, [], name);
    assert.equal(typeof result.error.row, 'number', name);
    assert.equal(typeof result.error.column, 'number', name);
  }
});

test('a field count mismatch is fatal and never returns partial rows', () => {
  const result = parseSheet('a,b\n1,2\n3,4,5\n6,7\n', {
    filename: 'x.csv',
  });
  assert.equal(result.error.code, 'fatal_field_count_mismatch');
  assert.equal(result.error.row, 3);
  assert.deepEqual(result.rows, []);
  assert.deepEqual(result.header, ['a', 'b']);
});

test('duplicate and empty headers are rejected with named positions', () => {
  const duplicate = parseSheet('account,owner,account\nx,y,z\n', {
    filename: 'x.csv',
  });
  assert.equal(duplicate.error.code, 'fatal_duplicate_header');
  assert.equal(duplicate.error.column, 3);
  assert.equal(duplicate.header, null);

  const empty = parseSheet('account,,note\nx,y,z\n', {
    filename: 'x.csv',
  });
  assert.equal(empty.error.code, 'fatal_empty_header');
  assert.equal(empty.error.column, 2);
  assert.equal(empty.header, null);
});

test('coercion failures are caught at the boundary and never escape', () => {
  const badDate = parseSheet('account,renewal\nAcme,next quarter\n', {
    filename: 'invalid-date.csv',
    types: { renewal: 'date' },
  });
  assert.equal(badDate.error.code, 'fatal_invalid_date');
  assert.equal(badDate.error.row, 2);
  assert.equal(badDate.error.column, 2);
  assert.deepEqual(badDate.rows, []);

  const badNumber = parseSheet('account,amount\nAcme,-2+3\n', {
    filename: 'numeric-formula.csv',
    types: { amount: 'number' },
  });
  assert.equal(badNumber.error.code, 'fatal_invalid_number');
  assert.deepEqual(badNumber.rows, []);
});

test('a header with zero data records remains valid', () => {
  const result = parseSheet('company,seats\n', { filename: 'empty.csv' });
  assert.equal(result.error, null);
  assert.deepEqual(result.header, ['company', 'seats']);
  assert.deepEqual(result.rows, []);
});

test('the BOM and the sep= hint do not leak into the header', () => {
  const bom = parseSheet(fixture('bom.csv'), { filename: 'bom.csv' });
  assert.deepEqual(bom.header, ['city', 'country']);
  assert.equal(bom.rows[0].city, 'Lisbon');

  const hint = parseSheet(fixture('hint-semicolon.csv'), {
    filename: 'hint-semicolon.csv',
    types: { booked: 'number' },
  });
  assert.deepEqual(hint.header, ['region', 'owner', 'quarter', 'booked']);
  assert.equal(hint.rows.length, 3);
  assert.equal(hint.rows[0].booked, 182400);
});

test('a Uint8Array and a string are accepted as input', () => {
  const csv = 'a,b\n1,2\n';
  const bytes = new TextEncoder().encode(csv);
  assert.equal(parseSheet(bytes, { filename: 'x.csv' }).error, null);
  assert.equal(parseSheet(csv, { filename: 'x.csv' }).error, null);
});

test('formula-like strings in numeric columns fail instead of coercing', () => {
  const result = parseSheet('account,amount\nAcme,@SUM(A1)\n', {
    filename: 'numeric-formula.csv',
    types: { amount: 'number' },
  });
  assert.equal(result.error.code, 'fatal_invalid_number');
  assert.deepEqual(result.rows, []);
});
