import { escapeText, foldLine } from './text.js';
import { paramList } from './params.js';

/**
 * Format a Date as an iCalendar UTC timestamp.
 *
 * @param {Date} date
 * @returns {string}
 */
function utcStamp(date) {
  return date
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d{3}Z$/, 'Z');
}

/**
 * Format a Date as a local floating timestamp (no zone suffix).
 *
 * @param {Date} date
 * @returns {string}
 */
function localStamp(date) {
  return utcStamp(date).replace(/Z$/, '');
}

/**
 * Compose a single content line from a name, parameters and a value.
 *
 * @param {string} name
 * @param {Record<string, unknown>|Map<string, unknown>|null|undefined} params
 * @param {string} value
 * @returns {string}
 */
function line(name, params, value) {
  const rendered = paramList(params);
  if (rendered) return name + ':' + rendered + value;
  return name + ':' + value;
}

/**
 * Build the VEVENT body of an .ics document.
 *
 * @param {object} event
 * @returns {string}
 */
export function buildEvent(event) {
  const {
    title = '',
    location = '',
    description = '',
    organizer = null,
    attendees = [],
    start,
    end,
    uid,
    sequence = 0,
    timeZone = null,
    rule = null,
    attachments = [],
  } = event;

  const out = [];
  out.push('BEGIN:VEVENT');
  out.push(line('UID', null, String(uid ?? '')));
  out.push(line('SEQUENCE', null, String(sequence)));
  out.push(line('DTSTAMP', null, utcStamp(new Date())));

  const dtParams = timeZone ? new Map([['TZID', timeZone]]) : null;
  out.push(line('DTSTART', dtParams, timeZone ? localStamp(start) : utcStamp(start)));
  out.push(line('DTEND', dtParams, timeZone ? localStamp(end) : utcStamp(end)));

  if (title !== undefined && title !== null) {
    out.push(line('SUMMARY', null, escapeText(title)));
  }
  if (location) {
    out.push(line('LOCATION', null, escapeText(location)));
  }
  if (description) {
    out.push(line('DESCRIPTION', null, escapeText(description)));
  }

  if (organizer) {
    const params = new Map([['CN', escapeText(organizer.name)]]);
    out.push(line('ORGANIZER', params, 'mailto:' + organizer.email));
  }

  for (const attendee of attendees) {
    const params = new Map();
    params.set('CN', escapeText(attendee.name));
    if (attendee.role) {
      params.set('ROLE', Array.isArray(attendee.role) ? attendee.role : [attendee.role]);
    }
    out.push(line('ATTENDEE', params, 'mailto:' + attendee.email));
  }

  if (rule) {
    out.push(line('RRULE', null, String(rule)));
  }

  for (const attachment of attachments) {
    const params = new Map([['FMTTYPE', attachment.type || 'application/octet-stream']]);
    if (attachment.altRep) params.set('ALTREP', attachment.altRep);
    out.push(line('ATTACH', params, attachment.url || escapeText(attachment.description || '')));
  }

  out.push('END:VEVENT');

  return out.map((contentLine) => foldLine(contentLine + '\r\n')).join('');
}