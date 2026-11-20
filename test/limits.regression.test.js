'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const { parseSheet } = require('../src');
const { LIMITS, check } = require('../src/limits');

function fixture(name) {
  return fs.readFileSync(path.join(__dirname, '..', 'fixtures', name));
}

test('an oversized decoded field is rejected before it becomes a row', () => {
  const result = parseSheet(fixture('oversized-cell.csv'), {
    filename: 'oversized-cell.csv',
  });
  assert.equal(result.error.code, 'fatal_cell_too_large');
  assert.equal(result.error.row, 2);
  assert.equal(result.error.column, 2);
  assert.deepEqual(result.rows, []);
});

test('a record with more than 64 fields is rejected, including the header', () => {
  const result = parseSheet(fixture('too-many-fields.csv'), {
    filename: 'too-many-fields.csv',
  });
  assert.equal(result.error.code, 'fatal_too_many_fields');
  assert.equal(result.error.row, 1);
  assert.equal(result.error.column, 65);
  assert.equal(result.header, null);
});

test('the check helper reports cell and record limits', () => {
  const big = 'x'.repeat(LIMITS.maxCellBytes + 1);
  const cellError = check({ fields: ['a', big], rowNumber: 4 });
  assert.equal(cellError.code, 'fatal_cell_too_large');
  assert.equal(cellError.row, 4);
  assert.equal(cellError.column, 2);

  const many = Array.from({ length: LIMITS.maxFieldsPerRecord + 1 }, (_, i) =>
    String(i),
  );
  assert.equal(
    check({ fields: many, rowNumber: 2 }).code,
    'fatal_too_many_fields',
  );

  const recordsError = check({ rowNumber: 50002, recordCount: 50001 });
  assert.equal(recordsError.code, 'fatal_too_many_records');
  assert.equal(recordsError.row, 50002);
});

test('exactly 50000 data records pass but the 50001st fails fatally', () => {
  const build = (count) =>
    'a,b\n' +
    Array.from({ length: count }, (_, i) => `v${i},1`).join('\n') +
    '\n';

  const ok = parseSheet(build(LIMITS.maxRecords), { filename: 'x.csv' });
  assert.equal(ok.error, null);
  assert.equal(ok.rows.length, LIMITS.maxRecords);

  const tooMany = parseSheet(build(LIMITS.maxRecords + 1), {
    filename: 'x.csv',
  });
  assert.equal(tooMany.error.code, 'fatal_too_many_records');
  assert.equal(tooMany.error.row, LIMITS.maxRecords + 2);
  assert.deepEqual(tooMany.rows, []);
});

test('a body larger than 8 MiB is rejected before decoding', () => {
  const body = Buffer.alloc(LIMITS.maxBodyBytes + 1, 0x61);
  const result = parseSheet(body, { filename: 'x.csv' });
  assert.equal(result.error.code, 'fatal_request_too_large');
  assert.deepEqual(result.rows, []);
});
