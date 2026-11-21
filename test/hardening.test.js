import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { handleInvite } from '../src/http/invite.js';
import { escapeText, foldLine } from '../src/ics/text.js';
import { quoteParam, quoteCn, paramList } from '../src/ics/params.js';
import { buildEnvelope } from '../src/mime/envelope.js';
import {
  parseRRule,
  expandOccurrences,
  toRRuleString,
  MAX_INSTANCES,
  RuleError,
} from '../src/ics/recur.js';

const here = dirname(fileURLToPath(import.meta.url));
const fixture = (name) => JSON.parse(readFileSync(join(here, 'fixtures', name), 'utf8'));

const start = '2026-03-02T09:00:00.000Z';
const baseRequest = () => ({ ...fixture('simple-request.json') });
const linesOf = (ics) => ics.split('\r\n');
const logicalLinesOf = (ics) =>
  ics
    .replace(/\r\n /g, '')
    .split('\r\n')
    .filter((line) => line.length > 0);

test('escapeText handles commas, semicolons and every line break form', () => {
  assert.equal(escapeText('a,b;c'), 'a\\,b\\;c');
  assert.equal(escapeText('x\r\ny\nz\rq'), 'x\\ny\\nz\\nq');
  assert.equal(escapeText('back\\nslash'), 'back\\\\nslash');
  assert.doesNotMatch(escapeText('a\r\nb'), /[\r\n]/);
});

test('foldLine measures UTF-8 octets and preserves continuation spaces', () => {
  assert.equal(foldLine('A'.repeat(75)), 'A'.repeat(75));
  const folded = foldLine('A'.repeat(76));
  assert.deepEqual(
    folded.split('\r\n').map((line) => Buffer.byteLength(line, 'utf8')),
    [75, 2],
  );

  const cjk = 'SUMMARY:' + '中'.repeat(60);
  const cjkFolded = foldLine(cjk);
  for (const line of cjkFolded.split('\r\n')) {
    assert.ok(Buffer.byteLength(line, 'utf8') <= 75);
  }
  assert.equal(cjkFolded.replace(/\r\n /g, ''), cjk);

  const escaped = 'SUMMARY:' + '\\,'.repeat(60);
  const escapeFolded = foldLine(escaped);
  for (const physical of escapeFolded.split('\r\n')) {
    assert.ok(!/(^|[^\\])\\$/.test(physical));
  }
  assert.equal(escapeFolded.replace(/\r\n /g, ''), escaped);
});

test('parameter values quote per the token rule and escape correctly', () => {
  assert.equal(quoteParam('CHAIR'), 'CHAIR');
  assert.equal(quoteParam(''), '""');
  assert.equal(quoteParam('CHAIR,REQ-PARTICIPANT'), '"CHAIR,REQ-PARTICIPANT"');
  assert.equal(quoteParam('a:b'), '"a:b"');
  assert.equal(quoteParam('he said "hi"'), '"he said \\"hi\\""');
  assert.equal(quoteParam('back\\slash'), '"back\\\\slash"');
  assert.throws(() => quoteParam('a\nb'), /line break/);
});

test('quoteCn does not double escape the backslashes TEXT escaping introduced', () => {
  assert.equal(quoteCn('Alice\\, Bob'), '"Alice\\, Bob"');
  assert.equal(quoteCn('plain'), 'plain');
  assert.equal(quoteCn(''), '""');
  assert.equal(quoteCn('say "hi"'), '"say \\"hi\\""');
});

test('multi-valued parameters become repeated occurrences, never comma packing', () => {
  assert.equal(
    paramList(new Map([['ROLE', ['CHAIR', 'REQ-PARTICIPANT']]])),
    'ROLE=CHAIR;ROLE=REQ-PARTICIPANT',
  );
});

test('a display name with a line break cannot inject another ATTENDEE', () => {
  const result = handleInvite({
    start,
    attendees: [
      {
        name: 'Alice\nATTENDEE:mailto:mallory@example.com',
        email: 'alice@example.com',
      },
    ],
  });
  const attendeeLines = logicalLinesOf(result.ics).filter((line) =>
    line.startsWith('ATTENDEE'),
  );
  assert.equal(attendeeLines.length, 1);
  const logical = attendeeLines[0];
  assert.match(logical, /^ATTENDEE;CN="Alice\\nATTENDEE:mailto:mallory[^"]*"/);
  assert.match(logical, /:mailto:alice@example\.com$/);
});

test('a comma in the title is escaped in SUMMARY', () => {
  const result = handleInvite({ ...baseRequest(), title: 'Plan, review' });
  assert.match(result.ics, /\r\nSUMMARY:Plan\\, review\r\n/);
});

test('special characters in a CN stay inside the single quoted parameter value', () => {
  const result = handleInvite({
    start,
    attendees: [{ name: 'a;b:c"d\\e', email: 'carol@example.com' }],
  });
  const line = logicalLinesOf(result.ics).find((entry) => entry.startsWith('ATTENDEE'));
  assert.equal((line.match(/ATTENDEE/g) || []).length, 1);
  assert.match(line, /:mailto:carol@example\.com$/);
});

test('a role containing a comma is one quoted value; an array role is rejected', () => {
  const result = handleInvite({
    start,
    attendees: [{ name: 'Dave', email: 'dave@example.com', role: 'CHAIR,REQ-PARTICIPANT' }],
  });
  assert.match(
    linesOf(result.ics).find((line) => line.startsWith('ATTENDEE')),
    /ROLE="CHAIR,REQ-PARTICIPANT":mailto:/,
  );
  assert.throws(
    () =>
      handleInvite({
        start,
        attendees: [
          { name: 'Dave', email: 'dave@example.com', role: ['CHAIR', 'REQ-PARTICIPANT'] },
        ],
      }),
    /one role/,
  );
});

test('invalid mail addresses are rejected instead of becoming mailto values', () => {
  assert.throws(
    () => handleInvite({ ...baseRequest(), attendees: [{ email: 'not-an-email' }] }),
    /e-mail/,
  );
  assert.throws(
    () => handleInvite({ start, organizer: { name: 'A', email: 'bad@' } }),
    /e-mail/,
  );
  assert.doesNotThrow(() =>
    handleInvite({ start, attendees: [{ email: 'ok@example.com' }] }),
  );
});

test('missing fields are omitted while present empty fields keep an empty value', () => {
  const missing = handleInvite({ start });
  assert.ok(!linesOf(missing.ics).some((line) => line.startsWith('SUMMARY')));
  assert.ok(!linesOf(missing.ics).some((line) => line.startsWith('LOCATION')));

  const empty = handleInvite({ start, title: '', location: '', description: '' });
  assert.ok(linesOf(empty.ics).includes('SUMMARY:'));
  assert.ok(linesOf(empty.ics).includes('LOCATION:'));
  assert.ok(linesOf(empty.ics).includes('DESCRIPTION:'));
});

test('an ALTREP value with a line break is rejected', () => {
  assert.throws(
    () =>
      handleInvite({
        start,
        attachments: [{ description: 'd', altRep: 'https://example.com/\nBcc: x' }],
      }),
    /ALTREP/,
  );
});

test('every physical ICS line stays within 75 UTF-8 octets and round-trips', () => {
  const result = handleInvite({
    ...baseRequest(),
    title: '中'.repeat(80),
    description: '长'.repeat(80),
  });
  const physical = result.ics.split('\r\n').filter((line) => line.length > 0);
  for (const line of physical) {
    assert.ok(Buffer.byteLength(line, 'utf8') <= 75);
  }
  const unfolded = result.ics.replace(/\r\n /g, '');
  const summary = unfolded
    .split('\r\n')
    .find((line) => line.startsWith('SUMMARY:'));
  assert.equal(summary, 'SUMMARY:' + '中'.repeat(80));
});

test('the generated document is UTF-8 with CRLF, no BOM and no leading blank line', () => {
  const result = handleInvite(baseRequest());
  const bytes = Buffer.from(result.ics, 'utf8');
  assert.ok(!(bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf));
  assert.ok(!result.ics.startsWith('\r\n'));
  assert.ok(!/(^|[^\r])\n/.test(result.ics));
});

test('RRULE parsing rejects every unsupported or contradictory shape', () => {
  const rejects = [
    'FREQ=MINUTELY;COUNT=5',
    'FREQ=DAILY;BYSECOND=0;COUNT=3',
    'FREQ=DAILY;FREQ=WEEKLY;COUNT=3',
    'FREQ=DAILY;COUNT=3;UNTIL=20260401T000000Z',
    'FREQ=DAILY;X-FOO=1;COUNT=3',
    'COUNT=3',
    'FREQ=DAILY;INTERVAL=0;COUNT=3',
    'FREQ=DAILY;COUNT=0',
    'FREQ=DAILY;INTERVAL=1.5;COUNT=3',
    'FREQ=DAILY;COUNT=3;',
  ];
  for (const rule of rejects) {
    assert.throws(() => parseRRule(rule), RuleError, rule);
  }
  assert.throws(
    () => parseRRule('FREQ=DAILY;COUNT=1001'),
    new RegExp(String(MAX_INSTANCES)),
  );
});

test('COUNT above the shared limit fails fast without materialising the series', () => {
  const began = process.hrtime.bigint();
  assert.throws(
    () =>
      handleInvite({
        ...baseRequest(),
        recurrence: 'FREQ=MINUTELY;COUNT=100000',
      }),
    /not supported|expansion limit/,
  );
  assert.ok(Number(process.hrtime.bigint() - began) < 1_000_000_000);
});

test('an unbounded weekly rule fills the window and reports real busy minutes', () => {
  const result = handleInvite({
    ...baseRequest(),
    timeZone: 'Europe/Berlin',
    recurrence: 'FREQ=WEEKLY',
    start,
    windowEnd: '2026-06-01T00:00:00.000Z',
  });
  assert.ok(result.occurrences > 0);
  assert.equal(result.busyMinutes, result.occurrences * 30);
});

test('UNTIL is compared as a UTC instant whether supplied local or in Z form', () => {
  const window = { start: new Date(start), end: new Date('2026-04-01T00:00:00.000Z') };
  const withZ = expandOccurrences(
    parseRRule('FREQ=DAILY;UNTIL=20260310T120000Z', 'Europe/Berlin'),
    window,
    'Europe/Berlin',
  );
  const local = expandOccurrences(
    parseRRule('FREQ=DAILY;UNTIL=20260310T130000', 'Europe/Berlin'),
    window,
    'Europe/Berlin',
  );
  assert.equal(withZ.length, local.length);
  assert.equal(withZ.length, 9);

  const descriptor = parseRRule('FREQ=DAILY;UNTIL=20260310T130000', 'Europe/Berlin');
  assert.match(toRRuleString(descriptor), /UNTIL=20260310T120000Z$/);
});

test('an unknown time zone is rejected', () => {
  assert.throws(
    () =>
      handleInvite({
        ...baseRequest(),
        timeZone: 'Mars/Olympus',
        recurrence: 'FREQ=DAILY;COUNT=2',
      }),
    /IANA zone/,
  );
});

test('expansion over the window limit is an error, not a warning', () => {
  assert.throws(
    () =>
      handleInvite({
        ...baseRequest(),
        timeZone: 'UTC',
        recurrence: 'FREQ=DAILY',
        start: '2026-01-01T00:00:00.000Z',
        windowEnd: '2030-01-01T00:00:00.000Z',
      }),
    new RegExp(MAX_INSTANCES),
  );
});

test('the half-open window includes windowStart and excludes windowEnd', () => {
  const descriptor = parseRRule('FREQ=DAILY;COUNT=3', 'UTC');
  const startDate = new Date(start);
  const sameBounds = { start: startDate, end: new Date(start) };
  assert.deepEqual(expandOccurrences(descriptor, sameBounds, 'UTC'), []);
  const includesStart = {
    start: startDate,
    end: new Date(startDate.getTime() + 1000),
  };
  assert.equal(expandOccurrences(descriptor, includesStart, 'UTC').length, 1);
});

test('emitted DTSTART keeps the local wall time with a quoted TZID', () => {
  const result = handleInvite({
    ...baseRequest(),
    timeZone: 'Europe/Berlin',
    start: '2026-03-29T08:00:00.000Z',
  });
  assert.match(
    linesOf(result.ics).find((line) => line.startsWith('DTSTART')),
    /^DTSTART;TZID="Europe\/Berlin":20260329T100000$/,
  );
});

test('the envelope uses CRLF everywhere and never a bare LF', () => {
  const result = handleInvite({
    ...baseRequest(),
    attachments: [{ description: 'Agenda\nsecond line', type: 'text/plain' }],
  });
  assert.ok(!/(^|[^\r])\n/.test(result.envelope));
  assert.match(
    result.envelope,
    new RegExp('Content-Type: multipart/mixed; boundary=' + result.boundary),
  );
  assert.ok(result.envelope.endsWith('--' + result.boundary + '--\r\n'));
});

test('a header injection attempt through the title is rejected', () => {
  assert.throws(
    () => handleInvite({ ...baseRequest(), title: 'Hi\r\nBcc: evil@example.com' }),
    /line break/,
  );
});

test('the boundary never occurs inside a body part', () => {
  const calendar = 'BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n';
  const description = 'plain attachment body';
  const result = buildEnvelope({
    calendar,
    subject: 'S',
    attachment: { description, type: 'text/plain' },
  });
  assert.ok(!calendar.includes(result.boundary));
  assert.ok(!description.includes(result.boundary));
  assert.ok(result.message.includes('--' + result.boundary + '\r\n'));
  assert.ok(result.message.includes('--' + result.boundary + '--\r\n'));
});

test('a token boundary is written bare in the Content-Type header', () => {
  const result = buildEnvelope({ calendar: 'x' });
  assert.match(result.message, new RegExp('boundary=' + result.boundary + '\\r\\n'));
  assert.ok(!result.message.includes('boundary="' + result.boundary));
});
