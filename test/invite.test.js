import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { handleInvite } from '../src/http/invite.js';

const here = dirname(fileURLToPath(import.meta.url));
const fixture = (name) => JSON.parse(readFileSync(join(here, 'fixtures', name), 'utf8'));

test('a simple meeting produces an ics document with a DTSTART', () => {
  const result = handleInvite(fixture('simple-request.json'));

  assert.match(result.ics, /BEGIN:VCALENDAR\r\n/);
  assert.match(result.ics, /BEGIN:VEVENT\r\n/);
  assert.match(result.ics, /\r\nDTSTART:/);
  assert.match(result.ics, /\r\nSUMMARY:Sprint review\r\n/);
  assert.match(result.ics, /\r\nLOCATION:Room 1\r\n/);
  assert.match(result.ics, /END:VEVENT\r\n/);
  assert.match(result.ics, /END:VCALENDAR\r\n/);
});

test('a simple meeting is wrapped in a MIME message with a boundary', () => {
  const result = handleInvite(fixture('simple-request.json'));

  assert.match(result.ics, /ATTENDEE/);
  assert.ok(result.boundary.length > 0);
  assert.match(result.envelope, /Content-Type: multipart\/mixed; boundary=/);
  assert.ok(result.envelope.includes('--' + result.boundary));
  assert.ok(result.busyMinutes > 0);
});