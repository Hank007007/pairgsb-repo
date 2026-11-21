import { escapeText } from '../ics/text.js';
import { buildEvent } from '../ics/serialize.js';
import { parseRRule, expandOccurrences } from '../ics/recur.js';
import { buildEnvelope } from '../mime/envelope.js';

const DEFAULT_DURATION_MINUTES = 30;
const DEFAULT_WINDOW_DAYS = 90;
const MAX_INSTANCES = 500; // same ceiling as docs/recur.md §2

/**
 * Coerce an incoming attendee record into the shape used by the serializer.
 *
 * @param {object} raw
 * @returns {{ name: string, email: string, role: string }}
 */
function parseAttendee(raw) {
  const source = raw || {};
  return {
    name: source.name === undefined || source.name === null ? '' : String(source.name),
    email: source.email === undefined || source.email === null ? '' : String(source.email),
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
  if (!payload || typeof payload !== 'object') {
    throw new TypeError('invite payload must be an object');
  }
  if (!payload.start) {
    throw new TypeError('invite payload requires a start time');
  }

  const start = new Date(payload.start);
  if (Number.isNaN(start.getTime())) {
    throw new TypeError('start time is not a valid date');
  }

  const durationMinutes = Number.isFinite(Number(payload.durationMinutes))
    ? Number(payload.durationMinutes)
    : DEFAULT_DURATION_MINUTES;
  const end = payload.end ? new Date(payload.end) : addMinutes(start, durationMinutes);

  const organizer = payload.organizer
    ? {
        name: payload.organizer.name ?? '',
        email: payload.organizer.email ?? '',
      }
    : null;

  const attendees = Array.isArray(payload.attendees)
    ? payload.attendees.map(parseAttendee)
    : [];

  const attachments = Array.isArray(payload.attachments)
    ? payload.attachments.map((item) => ({
        description: item?.description ?? '',
        type: item?.type ?? 'text/plain',
        url: item?.url ?? '',
        altRep: item?.altRep ?? '',
      }))
    : [];

  const rule = payload.recurrence || null;
  const parsedRule = rule ? parseRRule(rule) : null;

  const windowEnd = payload.windowEnd
    ? new Date(payload.windowEnd)
    : addMinutes(start, DEFAULT_WINDOW_DAYS * 24 * 60);

  let occurrences;
  if (parsedRule) {
    occurrences = expandOccurrences(parsedRule, { start, end: windowEnd }, payload.timeZone);
  } else {
    occurrences = [start];
  }

  if (occurrences.length > MAX_INSTANCES) {
    console.warn(
      'invite series has ' + occurrences.length + ' occurrences, above the configured ceiling',
    );
  }

  const busyMinutes = occurrences.length * durationMinutes;

  const uid = payload.uid || 'invite-' + start.getTime() + '@invitefence.local';
  const ics = wrapCalendar({
    title: payload.title ?? '',
    location: payload.location ?? '',
    description: payload.description ?? '',
    organizer,
    attendees,
    start,
    end,
    uid,
    sequence: payload.sequence ?? 0,
    timeZone: payload.timeZone || null,
    rule,
    attachments,
  });

  const envelope = buildEnvelope({
    calendar: ics,
    subject: payload.title || 'Meeting invitation',
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