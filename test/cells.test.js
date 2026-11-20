'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { sanitizeCell, coerce, exportCell, quoteCell } = require('../src/cells');

test('text passes through unchanged', () => {
  assert.equal(coerce('Acme Inc.', 'text'), 'Acme Inc.');
});

test('an untyped column is treated as text', () => {
  assert.equal(coerce('0042'), '0042');
});

test('numbers are parsed', () => {
  assert.equal(coerce('1990.00', 'number'), 1990);
  assert.equal(coerce('640.5', 'number'), 640.5);
});

test('an empty number cell is null', () => {
  assert.equal(coerce('', 'number'), null);
});

test('an empty date cell is null', () => {
  assert.equal(coerce('', 'date'), null);
});

test('an ISO date is parsed to a UTC instant', () => {
  const value = coerce('2026-03-01', 'date');
  assert.ok(value instanceof Date);
  assert.equal(value.toISOString(), '2026-03-01T00:00:00.000Z');
});

test('an ISO date-time is parsed to a UTC instant', () => {
  const value = coerce('2026-03-01T09:30:00', 'date');
  assert.equal(value.toISOString(), '2026-03-01T09:30:00.000Z');
});

test('a leading equals sign gets a protective prefix on export', () => {
  const rendered = sanitizeCell('=SUM(A1:A9)');
  assert.ok(rendered.startsWith("'"), rendered);
  assert.ok(rendered.includes('=SUM(A1:A9)'), rendered);
});

test('ordinary text is unchanged on export', () => {
  assert.equal(sanitizeCell('Northwind Traders'), 'Northwind Traders');
  assert.equal(sanitizeCell(''), '');
});

test('null and undefined render as an empty cell', () => {
  assert.equal(exportCell(null), '');
  assert.equal(exportCell(undefined), '');
});

test('a decimal value exports as its text form', () => {
  assert.equal(exportCell(1990), '1990');
  assert.equal(exportCell(-12.5), '-12.5');
});

test('a date exports as an ISO instant', () => {
  const value = coerce('2026-03-01', 'date');
  assert.equal(exportCell(value, 'date'), '2026-03-01T00:00:00.000Z');
});

test('a cell containing a comma is quoted', () => {
  assert.equal(quoteCell('Acme, Inc.'), '"Acme, Inc."');
});

test('a cell with a newline is quoted', () => {
  assert.equal(quoteCell('line one\nline two'), '"line one\nline two"');
});
