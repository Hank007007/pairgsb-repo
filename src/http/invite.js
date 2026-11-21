import { buildEvent } from '../ics/serialize.js';
import {
  parseRRule,
  expandOccurrences,
  toRRuleString,
  isKnownZone,
  MAX_INSTANCES,
} from '../ics/recur.js';
import { buildEnvelope } from '../mime/envelope.js';

const DEFAULT_DURATION_MINUTES = 30;
const DEFAULT_WINDOW_DAYS = 90;
const MAIL_ADDRESS = /^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$/;

function fail(message) {
  throw new TypeError(message);
}

/**
 * Validate a syntactically usable mail address.
 *
 * @param {unknown} value
 * @returns {string}
 */
function requireEmail(value) {
  const address = value === undefined || value === null ? '' : String(value);
  if (address.length === 0 || address.length > 254 || !MAIL_ADDRESS.test(address)) {
    fail(`invalid e-mail address: ${address || '(empty)'}`);
  }
  return address;
}

/**
 * Coerce an attendee record into the validated triple used by the renderer.
 *
 * @param {object} raw
 * @returns {{ name: string, email: string, role: string }}
 */
function parseAttendee(raw) {
  const source = raw || {};
  if (source.role !== undefined && typeof source.role !== 'string') {
    fail('an attendee has exactly one role supplied as a string');
  }
  return {
    name: source.name === undefined || source.name === null ? '' : String(source.name),
    email: requireEmail(source.email),
    role: source.role || 'REQ-PARTICIPANT',
  };
}

/**
 * Add minutes to a Date.
 *
 * @param {Date} date
 * @param {number} minutes
 * @returns {Date}
 */
function addMinutes(date, minutes) {
  return new Date(date.getTime() + minutes * 60 * 1000);
}

function requireDate(value, label) {
  if (value === undefined || value === null || value === '') return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) fail(`${label} is not a valid date`);
  return date;
}

function optionalText(value) {
  if (value === undefined || value === null) return undefined;
  return String(value);
}

/**
 * Build the full VCALENDAR document around the event body.
 *
 * @param {object} event
 * @returns {string}
 */
function wrapCalendar(event) {
  const body = buildEvent(event);
  return [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//invitefence//calendar invite//EN',
    'CALSCALE:GREGORIAN',
    'METHOD:REQUEST',
    body,
    'END:VCALENDAR',
    '',
  ].join('\r\n');
}

/**
 * Handle one invite request: build the calendar invite, wrap it in MIME, and
 * account for the busy time the series occupies.
 *
 * @param {object} payload
 * @returns {object}
 */
export function handleInvite(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    fail('invite payload must be an object');
  }
  if (!payload.start) fail('invite payload requires a start time');

  const start = requireDate(payload.start, 'start');

  let durationMinutes = DEFAULT_DURATION_MINUTES;
  if (payload.durationMinutes !== undefined && payload.durationMinutes !== null) {
    durationMinutes = Number(payload.durationMinutes);
    if (!Number.isFinite(durationMinutes) || durationMinutes <= 0) {
      fail('durationMinutes must be a positive number');
    }
  }

  const explicitEnd = requireDate(payload.end, 'end');
  const end = explicitEnd ?? addMinutes(start, durationMinutes);
  if (end <= start) fail('end must be after start');

  if (
    payload.sequence !== undefined &&
    payload.sequence !== null &&
    (!Number.isInteger(payload.sequence) || payload.sequence < 0)
  ) {
    fail('sequence must be a non-negative integer');
  }

  const timeZone = payload.timeZone ? String(payload.timeZone) : null;
  if (timeZone && !isKnownZone(timeZone)) {
    fail(`time zone ${timeZone} is not a recognised IANA zone`);
  }

  const organizer = payload.organizer
    ? {
        name: payload.organizer.name === undefined || payload.organizer.name === null
          ? ''
          : String(payload.organizer.name),
        email: requireEmail(payload.organizer.email),
      }
    : null;

  const attendees = Array.isArray(payload.attendees)
    ? payload.attendees.map(parseAttendee)
    : payload.attendees === undefined
      ? []
      : fail('attendees must be an array');

  const attachments = Array.isArray(payload.attachments)
    ? payload.attachments.map((item) => ({
        description: item?.description === undefined || item?.description === null
          ? ''
          : String(item.description),
        type: item?.type ?? 'text/plain',
        url: item?.url ?? '',
        altRep: item?.altRep ?? '',
      }))
    : payload.attachments === undefined
      ? []
      : fail('attachments must be an array');

  for (const attachment of attachments) {
    if (attachment.altRep && /[\r\n]/.test(attachment.altRep)) {
      fail('ALTREP values must not contain a line break');
    }
  }

  if (
    payload.recurrence !== undefined &&
    payload.recurrence !== null &&
    typeof payload.recurrence !== 'string'
  ) {
    fail('recurrence must be an RRULE string');
  }
  const rawRule = payload.recurrence || null;
  const parsedRule = rawRule ? parseRRule(rawRule, timeZone) : null;
  const rule = parsedRule ? toRRuleString(parsedRule) : null;

  const windowEnd =
    requireDate(payload.windowEnd, 'windowEnd') ??
    addMinutes(start, DEFAULT_WINDOW_DAYS * 24 * 60);
  if (windowEnd <= start) fail('windowEnd must be after start');

  const occurrences = parsedRule
    ? expandOccurrences(parsedRule, { start, end: windowEnd }, timeZone)
    : [start];

  if (occurrences.length > MAX_INSTANCES) {
    fail(`expansion exceeds the limit of ${MAX_INSTANCES} instances`);
  }

  const busyMinutes = occurrences.length * durationMinutes;

  const uid = payload.uid || 'invite-' + start.getTime() + '@invitefence.local';
  const ics = wrapCalendar({
    title: optionalText(payload.title),
    location: optionalText(payload.location),
    description: optionalText(payload.description),
    organizer,
    attendees,
    start,
    end,
    uid,
    sequence: payload.sequence ?? 0,
    timeZone,
    rule,
    attachments,
  });

  const subject = payload.title ? String(payload.title) : 'Meeting invitation';
  const envelope = buildEnvelope({
    calendar: ics,
    subject,
    attachment: attachments[0] || null,
  });

  return {
    ics,
    envelope: envelope.message,
    boundary: envelope.boundary,
    busyMinutes,
    occurrences: occurrences.length,
    attendees,
  };
}
