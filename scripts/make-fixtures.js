'use strict';

const fs = require('node:fs');
const path = require('node:path');

const B = 'XyZ-boundary-42';

function body(parts, { close = true, epilogue = '', preamble = '' } = {}) {
  let out = preamble;
  for (const p of parts) {
    out += `--${B}\r\n`;
    for (const [k, v] of p.headers) out += `${k}: ${v}\r\n`;
    out += '\r\n';
    out += p.body;
    out += '\r\n';
  }
  if (close) out += `--${B}--`;
  out += epilogue;
  return out;
}

const fixtures = {
  // Ordinary request shapes.
  'simple-fields': {
    description: 'two plain fields',
    text: body([
      { headers: [['Content-Disposition', 'form-data; name="username"']], body: 'ada' },
      { headers: [['Content-Disposition', 'form-data; name="role"']], body: 'engineer' },
    ]),
  },
  'file-upload': {
    description: 'one file part with a content type',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="notes.txt"'],
          ['Content-Type', 'text/plain'],
        ],
        body: 'line one\nline two\n',
      },
    ]),
  },
  'empty-field': {
    description: 'a field with an empty body',
    text: body([
      { headers: [['Content-Disposition', 'form-data; name="note"']], body: '' },
    ]),
  },
  'quoted-filename': {
    description: 'quoted filename containing spaces and parentheses',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="Q3 report (final).txt"'],
          ['Content-Type', 'text/plain'],
        ],
        body: 'quarterly numbers\n',
      },
    ]),
  },
  'two-file-parts': {
    description: 'two file parts in one request',
    text: body([
      {
        headers: [['Content-Disposition', 'form-data; name="a"; filename="a.bin"']],
        body: 'AAAA',
      },
      {
        headers: [['Content-Disposition', 'form-data; name="b"; filename="b.bin"']],
        body: 'BBBBBB',
      },
    ]),
  },

  // Requests that exercise header and parameter shapes described by the policy.
  'folded-disposition': {
    description: 'Content-Disposition split across continuation lines',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data;\r\n name="avatar";\r\n filename="portrait.jpg"'],
          ['Content-Type', 'image/jpeg'],
        ],
        body: 'fake-jpeg-bytes',
      },
    ]),
  },
  'quoted-semicolon': {
    description: 'filename whose quoted value contains a semicolon and an equals sign',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="a;b=c.txt"'],
          ['Content-Type', 'text/plain'],
        ],
        body: 'alpha',
      },
    ]),
  },
  'extended-filename': {
    description: 'RFC 5987 filename* with a percent-encoded UTF-8 name',
    text: body([
      {
        headers: [
          ['Content-Disposition', "form-data; name=\"doc\"; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf"],
          ['Content-Type', 'application/pdf'],
        ],
        body: 'pdf-bytes',
      },
    ]),
  },
  'percent-in-filename': {
    description: 'plain filename carrying percent escapes',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="..%2f..%2fetc%2fpasswd"'],
        ],
        body: 'x',
      },
    ]),
  },
  'traversal-filename': {
    description: 'filename attempting to climb out with backslashes',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="..\\..\\boot.ini"'],
        ],
        body: 'x',
      },
    ]),
  },
  'absolute-filename': {
    description: 'absolute POSIX path in the filename',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="/etc/cron.d/x"'],
        ],
        body: 'x',
      },
    ]),
  },
  'drive-filename': {
    description: 'Windows drive-qualified path in the filename',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="C:\\Windows\\win.ini"'],
        ],
        body: 'x',
      },
    ]),
  },
  'nul-filename': {
    description: 'filename carrying a NUL byte before the extension',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="a\u0000.png"'],
        ],
        body: 'x',
      },
    ]),
  },
  'reserved-filename': {
    description: 'Windows reserved device name',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="CON.txt"'],
        ],
        body: 'x',
      },
    ]),
  },
  'nested-dotdot-filename': {
    description: 'repeated dot segments with doubled separators',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="doc"; filename="....//....//payload.sh"'],
        ],
        body: 'x',
      },
    ]),
  },
  'boundary-in-body': {
    description: 'part body embeds the boundary substring without a preceding CRLF',
    text: body([
      {
        headers: [
          ['Content-Disposition', 'form-data; name="note"'],
          ['Content-Type', 'text/plain'],
        ],
        body:
          'visible text --' +
          B +
          '\r\nContent-Disposition: form-data; name="ghost"\r\n\r\nghost body',
      },
    ]),
  },
  'trailing-epilogue': {
    description: 'closing delimiter followed by epilogue text',
    text: body(
      [
        {
          headers: [['Content-Disposition', 'form-data; name="note"']],
          body: 'payload',
        },
      ],
      { epilogue: '\r\nthis text is epilogue' }
    ),
  },
};

const manifest = [];
const dir = path.join(__dirname, '..', 'fixtures');

function addFixture(name, fixture) {
  const file = `${name}.http`;
  fs.writeFileSync(path.join(dir, file), Buffer.from(fixture.text, 'latin1'));
  const entry = {
    file,
    contentType: fixture.contentType || `multipart/form-data; boundary=${B}`,
    description: fixture.description,
  };
  if (fixture.limits) entry.limits = fixture.limits;
  manifest.push(entry);
}

for (const [name, fixture] of Object.entries(fixtures)) addFixture(name, fixture);

// Requests that push past the configured budgets.
function manyEmptyParts(count) {
  let out = '';
  for (let i = 0; i < count; i += 1) {
    out += `--${B}\r\nContent-Disposition: form-data; name="f${i}"\r\n\r\n\r\n`;
  }
  out += `--${B}--\r\n`;
  return out;
}

function headerHeavyPart(padBytes) {
  return (
    `--${B}\r\n` +
    `Content-Disposition: form-data; name="doc"; filename="x.txt"\r\n` +
    `X-Pad: ${'p'.repeat(padBytes)}\r\n` +
    `\r\n` +
    `v\r\n` +
    `--${B}--\r\n`
  );
}

function largeBodyPart(bytes) {
  return (
    `--${B}\r\n` +
    `Content-Disposition: form-data; name="doc"; filename="big.bin"\r\n` +
    `\r\n` +
    `${'z'.repeat(bytes)}\r\n` +
    `--${B}--\r\n`
  );
}

addFixture('many-parts', {
  description: 'forty empty parts against a part budget of eight',
  text: manyEmptyParts(40),
  limits: { maxParts: 8 },
});

addFixture('oversized-header', {
  description: 'one part whose header block is 60 KiB against a 4 KiB header budget',
  text: headerHeavyPart(60 * 1024),
  limits: { maxHeaderBytes: 4 * 1024 },
});

addFixture('oversized-part', {
  description: 'one 300 KiB part against a 64 KiB per-part budget',
  text: largeBodyPart(300 * 1024),
  limits: { maxPartBytes: 64 * 1024 },
});

addFixture('oversized-total', {
  description: 'five parts totalling more than the configured request budget',
  text: manyEmptyParts(5).replace(/\r\n\r\n\r\n/g, `\r\n\r\n${'q'.repeat(2048)}\r\n\r\n`),
  limits: { maxTotalBytes: 4096, maxPartBytes: 4096 },
});

addFixture('invalid-boundary', {
  description: 'boundary longer than seventy characters',
  contentType: `multipart/form-data; boundary=${'L'.repeat(90)}`,
  text: `--${'L'.repeat(90)}\r\nContent-Disposition: form-data; name="a"\r\n\r\nv\r\n--${'L'.repeat(90)}--\r\n`,
});

fs.writeFileSync(
  path.join(dir, 'manifest.json'),
  JSON.stringify(
    { contentType: `multipart/form-data; boundary=${B}`, requests: manifest },
    null,
    2
  ) + '\n'
);
