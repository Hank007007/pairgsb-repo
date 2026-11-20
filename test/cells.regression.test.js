'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const {
  coerce,
  exportCell,
  sanitizeCell,
  quoteCell,
  isGenuineNumber,
} = require('../src/cells');

test('number columns reject values parseFloat would have accepted', () => {
  for (const raw of ['-2+3', '+1 555 0100', '@SUM(A1:A9)', '1e3', 'NaN', 'Infinity', '12.3.4']) {
    assert.throws(() => coerce(raw, 'number'), (error) => {
      assert.equal(error.code, 'fatal_invalid_number');
      return true;
    }, raw);
  }
});

test('number columns accept signed plain decimals', () => {
  assert.equal(coerce('+12.50', 'number'), 12.5);
  assert.equal(coerce('-2', 'number'), -2);
  assert.equal(coerce('-.5', 'number'), -0.5);
  assert.equal(coerce('42', 'number'), 42);
});

test('date failures carry the fatal code instead of a bare TypeError', () => {
  assert.throws(
    () => coerce('next quarter', 'date'),
    (error) => error.code === 'fatal_invalid_date',
  );
});

test('all formula trigger characters are neutralised', () => {
  assert.equal(sanitizeCell('=1+1'), "'=1+1");
  assert.equal(sanitizeCell('+1 555 0100'), "'+1 555 0100");
  assert.equal(sanitizeCell('@SUM(A1:A9)'), "'@SUM(A1:A9)");
  assert.equal(sanitizeCell('-2+3'), "'-2+3");
  assert.equal(sanitizeCell(' =SUM(B1:B9)'), "' =SUM(B1:B9)");
  assert.equal(sanitizeCell('\tcmd'), "'\tcmd");
  assert.equal(sanitizeCell('\rfoo'), "'\rfoo");
});

test('genuine numbers and dates are not neutralised', () => {
  assert.equal(sanitizeCell('-12.5'), '-12.5');
  assert.equal(sanitizeCell('+8613800138000'), '+8613800138000');
  assert.equal(sanitizeCell('2026-01-31'), '2026-01-31');
  assert.equal(sanitizeCell('2026-01-31T09:30:00'), '2026-01-31T09:30:00');
  assert.ok(isGenuineNumber('+8613800138000'));
});

test('neutralisation is idempotent within one export pass', () => {
  assert.equal(sanitizeCell("'=1+1"), "'=1+1");
});

test('numeric and date typed values also go through neutralisation', () => {
  assert.equal(exportCell(-12.5), '-12.5');
  assert.equal(exportCell(1990), '1990');
  assert.equal(exportCell('@SUM(A1:A9)'), "'@SUM(A1:A9)");
});

test('the apostrophe prefix precedes doubled quotes inside a quoted field', () => {
  assert.equal(
    exportCell('=HYPERLINK("http://example.com","click")'),
    '"\'=HYPERLINK(""http://example.com"",""click"")"',
  );
});

test('embedded quotes are doubled when quoting', () => {
  assert.equal(quoteCell('he said "hi"'), '"he said ""hi"""');
});

test('empty cells follow the column type rules', () => {
  assert.equal(coerce('', 'number'), null);
  assert.equal(coerce('', 'date'), null);
  assert.equal(coerce('', 'text'), '');
  assert.equal(exportCell(null), '');
});
