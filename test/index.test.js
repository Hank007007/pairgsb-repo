'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseSheet } = require('../src');

test('parses a small comma separated sheet', () => {
  const csv = 'company,seats\nNorthwind,24\nContoso,8\n';
  const result = parseSheet(csv, {
    filename: 'accounts.csv',
    types: { seats: 'number' },
  });

  assert.equal(result.error, null);
  assert.deepEqual(result.header, ['company', 'seats']);
  assert.equal(result.rows.length, 2);
  assert.deepEqual(result.rows[0], { company: 'Northwind', seats: 24 });
  assert.deepEqual(result.rows[1], { company: 'Contoso', seats: 8 });
});

test('typed columns are coerced', () => {
  const csv = 'account,seats,renewal\nAcme,12,2026-01-31\n';
  const result = parseSheet(csv, {
    filename: 'accounts.csv',
    types: { seats: 'number', renewal: 'date' },
  });

  assert.equal(result.error, null);
  assert.equal(result.rows[0].seats, 12);
  assert.ok(result.rows[0].renewal instanceof Date);
  assert.equal(result.rows[0].renewal.toISOString(), '2026-01-31T00:00:00.000Z');
});

test('parses a tab separated sheet', () => {
  const tsv = 'region\towner\nEMEA\tPriya\nAPAC\tMarco\n';
  const result = parseSheet(tsv, { filename: 'pipeline.tsv' });

  assert.equal(result.error, null);
  assert.deepEqual(result.header, ['region', 'owner']);
  assert.deepEqual(result.rows, [
    { region: 'EMEA', owner: 'Priya' },
    { region: 'APAC', owner: 'Marco' },
  ]);
});

test('quoted fields keep their delimiters', () => {
  const csv = 'account,owner\n"Acme, Inc.","Lee, Sam"\n';
  const result = parseSheet(csv, { filename: 'accounts.csv' });

  assert.equal(result.error, null);
  assert.deepEqual(result.rows, [{ account: 'Acme, Inc.', owner: 'Lee, Sam' }]);
});

test('a header plus no data records is valid', () => {
  const result = parseSheet('company,seats\n', { filename: 'empty.csv' });

  assert.equal(result.error, null);
  assert.deepEqual(result.header, ['company', 'seats']);
  assert.deepEqual(result.rows, []);
});

test('an empty upload reports a positioned failure', () => {
  const result = parseSheet('', { filename: 'empty.csv' });

  assert.notEqual(result.error, null);
  assert.match(result.error.code, /^fatal/);
  assert.equal(result.error.row, 0);
  assert.equal(result.error.column, 0);
  assert.deepEqual(result.rows, []);
});

test('a multi-row sheet returns every data record', () => {
  const csv = 'company,seats\nNorthwind,24\nContoso,8\nFabrikam,120\n';
  const result = parseSheet(csv, { filename: 'accounts.csv' });

  assert.equal(result.error, null);
  assert.equal(result.rows.length, 3);
});
