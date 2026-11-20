'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { LIMITS, check } = require('../src/limits');

test('the published limits match the contract', () => {
  assert.equal(LIMITS.maxBodyBytes, 8 * 1024 * 1024);
  assert.equal(LIMITS.maxCellBytes, 32 * 1024);
  assert.equal(LIMITS.maxFieldsPerRecord, 64);
  assert.equal(LIMITS.maxRecords, 50000);
});

test('a small body passes', () => {
  assert.equal(check({ bytes: 1024 }), null);
});

test('a body larger than 8 MiB is rejected', () => {
  const result = check({ bytes: LIMITS.maxBodyBytes + 1 });
  assert.notEqual(result, null);
  assert.equal(result.code, 'fatal_request_too_large');
});
