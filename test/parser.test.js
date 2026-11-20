'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { MultipartParser } = require('../src/parser');
const { parseMultipart } = require('../src/index');
const { MultipartError } = require('../src/errors');

const BOUNDARY = 'XyZ-boundary-42';
const CONTENT_TYPE = `multipart/form-data; boundary=${BOUNDARY}`;

function build(parts, close = true) {
  let out = '';
  for (const part of parts) {
    out += `--${BOUNDARY}\r\n`;
    for (const [name, value] of part.headers) out += `${name}: ${value}\r\n`;
    out += '\r\n';
    out += part.body;
    out += '\r\n';
  }
  if (close) out += `--${BOUNDARY}--\r\n`;
  return Buffer.from(out, 'latin1');
}

function parse(body, options = {}) {
  const parser = new MultipartParser({ contentType: CONTENT_TYPE, ...options });
  parser.write(body);
  return parser.end();
}

test('parses plain fields', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="username"']], body: 'ada' },
    { headers: [['Content-Disposition', 'form-data; name="role"']], body: 'engineer' },
  ]);

  const parts = parse(body);

  assert.equal(parts.length, 2);
  assert.deepEqual(parts[0], {
    name: 'username',
    filename: null,
    contentType: null,
    size: 3,
  });
  assert.equal(parts[1].name, 'role');
  assert.equal(parts[1].size, 8);
});

test('parses a file part with filename and content type', () => {
  const body = build([
    {
      headers: [
        ['Content-Disposition', 'form-data; name="doc"; filename="notes.txt"'],
        ['Content-Type', 'text/plain'],
      ],
      body: 'line one\nline two\n',
    },
  ]);

  const [part] = parse(body);

  assert.equal(part.name, 'doc');
  assert.equal(part.filename, 'notes.txt');
  assert.equal(part.contentType, 'text/plain');
  assert.equal(part.size, 18);
});

test('handles an empty field body', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="note"']], body: '' },
  ]);

  const [part] = parse(body);

  assert.equal(part.name, 'note');
  assert.equal(part.filename, null);
  assert.equal(part.size, 0);
});

test('keeps spaces and parentheses inside a quoted filename', () => {
  const body = build([
    {
      headers: [
        ['Content-Disposition', 'form-data; name="doc"; filename="Q3 report (final).txt"'],
        ['Content-Type', 'text/plain'],
      ],
      body: 'quarterly numbers\n',
    },
  ]);

  const [part] = parse(body);

  assert.equal(part.filename, 'Q3 report (final).txt');
});

test('accepts the file disposition type', () => {
  const body = build([
    {
      headers: [
        ['Content-Disposition', 'file; name="blob"; filename="blob.bin"'],
        ['Content-Type', 'application/octet-stream'],
      ],
      body: 'blob',
    },
  ]);

  const [part] = parse(body);

  assert.equal(part.name, 'blob');
  assert.equal(part.filename, 'blob.bin');
});

test('normalises header names case-insensitively', () => {
  const body = build([
    {
      headers: [
        ['content-disposition', 'form-data; name="doc"; filename="x.txt"'],
        ['CONTENT-TYPE', 'text/plain'],
      ],
      body: 'x',
    },
  ]);

  const [part] = parse(body);

  assert.equal(part.contentType, 'text/plain');
  assert.equal(part.filename, 'x.txt');
});

test('produces the same result regardless of chunking', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="a"']], body: 'first' },
    {
      headers: [
        ['Content-Disposition', 'form-data; name="b"; filename="b.bin"'],
        ['Content-Type', 'application/octet-stream'],
      ],
      body: 'second payload',
    },
  ]);

  const whole = parse(body);

  for (const step of [1, 3, 7, 64]) {
    const parser = new MultipartParser({ contentType: CONTENT_TYPE });
    for (let i = 0; i < body.length; i += step) {
      parser.write(body.subarray(i, i + step));
    }
    assert.deepEqual(parser.end(), whole, `chunk size ${step}`);
  }
});

test('parseMultipart accepts a Buffer and returns parts', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="x"']], body: '1' },
  ]);

  const parts = parseMultipart(body, CONTENT_TYPE);

  assert.equal(parts.length, 1);
  assert.equal(parts[0].name, 'x');
  assert.equal(parts[0].size, 1);
});

test('end() is idempotent and returns the same parts', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="x"']], body: '1' },
  ]);

  const parser = new MultipartParser({ contentType: CONTENT_TYPE });
  parser.write(body);
  const first = parser.end();
  const second = parser.end();

  assert.equal(first.length, 1);
  assert.equal(second.length, 1);
});

test('rejects a part without Content-Disposition', () => {
  const body = build([
    { headers: [['Content-Type', 'text/plain']], body: 'x' },
  ]);

  assert.throws(
    () => parse(body),
    (err) => err instanceof MultipartError && err.status === 400 && err.code === 'MISSING_DISPOSITION'
  );
});

test('rejects an unsupported disposition type', () => {
  const body = build([
    { headers: [['Content-Disposition', 'attachment; filename="x.txt"']], body: 'x' },
  ]);

  assert.throws(
    () => parse(body),
    (err) => err instanceof MultipartError && err.code === 'UNSUPPORTED_DISPOSITION'
  );
});

test('rejects a request that ends before the closing delimiter', () => {
  const body = build(
    [{ headers: [['Content-Disposition', 'form-data; name="x"']], body: '1' }],
    false
  );

  assert.throws(
    () => parse(body),
    (err) => err instanceof MultipartError && err.code === 'TRUNCATED'
  );
});

test('rejects write() after end()', () => {
  const body = build([
    { headers: [['Content-Disposition', 'form-data; name="x"']], body: '1' },
  ]);

  const parser = new MultipartParser({ contentType: CONTENT_TYPE });
  parser.write(body);
  parser.end();

  assert.throws(
    () => parser.write(Buffer.from('more')),
    (err) => err instanceof MultipartError && err.code === 'REQUEST_ALREADY_ENDED'
  );
});

test('MultipartError serialises status, code and message', () => {
  const err = new MultipartError(413, 'PART_TOO_LARGE', 'too big');

  assert.deepEqual(err.toJSON(), {
    status: 413,
    code: 'PART_TOO_LARGE',
    message: 'too big',
  });
});
