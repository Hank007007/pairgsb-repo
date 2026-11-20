'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { decodeText, createTokenizer } = require('../src/tokenizer');

test('a quoted field may span physical lines including blank lines', () => {
  const decoded = decodeText('name,notes\n"Olive","first.\n\nsecond."\nKwan,plain\n', ',');
  assert.equal(decoded.error, null);
  assert.deepEqual(decoded.records, [
    ['name', 'notes'],
    ['Olive', 'first.\n\nsecond.'],
    ['Kwan', 'plain'],
  ]);
});

test('doubled quotes decode to a single quote', () => {
  const decoded = decodeText('phrase,owner\n"say ""hi"" twice",R.\n', ',');
  assert.equal(decoded.error, null);
  assert.deepEqual(decoded.records[1], ['say "hi" twice', 'R.']);
});

test('a backslash never escapes a closing quote', () => {
  const decoded = decodeText('account,note\nContoso,"He wrote \\"call me\\""\n', ',');
  assert.equal(decoded.error.code, 'fatal_unexpected_character');
  assert.equal(decoded.error.row, 2);
  assert.equal(decoded.records.length, 1);
});

test('text after a closing quote is a fatal positioned error', () => {
  const decoded = decodeText('account,note\nContoso,"closed" trailing\n', ',');
  assert.equal(decoded.error.code, 'fatal_unexpected_character');
  assert.equal(decoded.error.row, 2);
  assert.equal(decoded.error.column, 17);
});

test('an unclosed quote is fatal and the partial record is dropped', () => {
  const decoded = decodeText('account,note\nContoso,"never closed\nFabrikam,ok\n', ',');
  assert.equal(decoded.error.code, 'fatal_unterminated_quote');
  assert.equal(decoded.error.row, 2);
  assert.equal(decoded.error.column, 9);
  assert.deepEqual(decoded.records, [['account', 'note']]);
});

test('line endings inside quotes are preserved byte for byte', () => {
  const decoded = decodeText('account,note\r\nContoso,"first\r\nsecond"\r\n', ',');
  assert.equal(decoded.error, null);
  assert.deepEqual(decoded.records[1], ['Contoso', 'first\r\nsecond']);
});

test('lone CR, CRLF and LF each terminate exactly one record', () => {
  assert.deepEqual(decodeText('a;b\rc;d\r', ';').records, [
    ['a', 'b'],
    ['c', 'd'],
  ]);
  assert.deepEqual(decodeText('a,b\r\nc,d\r\n', ',').records, [
    ['a', 'b'],
    ['c', 'd'],
  ]);
  assert.deepEqual(decodeText('a,b\nc,d\n', ',').records, [
    ['a', 'b'],
    ['c', 'd'],
  ]);
});

test('an empty trailing line yields no record while a middle empty line does', () => {
  assert.deepEqual(decodeText('', ',').records, []);
  assert.deepEqual(decodeText('\n', ',').records, []);
  assert.deepEqual(decodeText('a,b\n', ',').records, [['a', 'b']]);
  assert.deepEqual(decodeText('a,b\n\nc,d\n', ',').records, [
    ['a', 'b'],
    [''],
    ['c', 'd'],
  ]);
});

test('a quote inside an unquoted field stays ordinary data', () => {
  const decoded = decodeText('label,size\nmonitor,5" screen\n', ',');
  assert.equal(decoded.error, null);
  assert.deepEqual(decoded.records[1], ['monitor', '5" screen']);
});

test('the tokenizer surface exposes the structural error', () => {
  const tokenizer = createTokenizer('a,b\nx,"never closed', ',');
  assert.deepEqual(tokenizer.nextRecord(), ['a', 'b']);
  assert.equal(tokenizer.nextRecord(), null);
  assert.equal(tokenizer.error.code, 'fatal_unterminated_quote');
});
