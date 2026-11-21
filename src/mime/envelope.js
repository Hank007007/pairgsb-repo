import { randomBytes } from 'node:crypto';

const CRLF = '\r\n';
const TOKEN_ONLY = /^[A-Za-z0-9\-_.]+$/;
const MAX_BOUNDARY_TRIES = 32;

/**
 * Generate a random multipart boundary using the service's restricted
 * token-character subset.
 *
 * @returns {string}
 */
function makeBoundary() {
  return 'invitefence_' + randomBytes(24).toString('hex');
}

/**
 * Quote a header parameter value exactly as params.md section 2.1 requires.
 *
 * @param {string} value
 * @returns {string}
 */
function quoteHeaderParam(value) {
  if (TOKEN_ONLY.test(value)) return value;
  return '"' + value.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
}

/**
 * Reject header values that would embed a raw line break.
 *
 * @param {string} name
 * @param {string} value
 */
function assertSingleLine(name, value) {
  if (typeof value !== 'string' || /[\r\n]/.test(value)) {
    throw new TypeError(`${name} header value must not contain a raw line break`);
  }
}

/**
 * Wrap the calendar body and optional attachment description in a
 * multipart/mixed MIME message.
 *
 * @param {{ calendar: string, subject?: string, attachment?: object|null }} input
 * @returns {{ boundary: string, message: string }}
 */
export function buildEnvelope(input) {
  const { calendar = '', subject = '', attachment = null } = input || {};
  const attachmentBody = attachment ? attachment.description || '' : '';

  let boundary;
  for (let attempt = 0; attempt < MAX_BOUNDARY_TRIES; attempt += 1) {
    boundary = makeBoundary();
    if (!calendar.includes(boundary) && !attachmentBody.includes(boundary)) break;
    boundary = null;
  }
  if (!boundary) {
    throw new TypeError('could not generate a boundary absent from the message body');
  }

  const headers = [];
  headers.push('MIME-Version: 1.0');
  if (subject) {
    assertSingleLine('Subject', subject);
    headers.push('Subject: ' + subject);
  }
  headers.push('Content-Type: multipart/mixed; boundary=' + quoteHeaderParam(boundary));

  const lines = [];
  lines.push('--' + boundary);
  lines.push('Content-Type: text/calendar; method=REQUEST; charset=UTF-8');
  lines.push('Content-Transfer-Encoding: 8bit');
  lines.push('');

  const message =
    headers.join(CRLF) +
    CRLF +
    CRLF +
    lines.join(CRLF) +
    CRLF +
    calendar.replace(/\r?\n/g, CRLF) +
    CRLF +
    (attachment
      ? [
          '--' + boundary,
          'Content-Type: ' + (attachment.type || 'text/plain') + '; charset=UTF-8',
          'Content-Transfer-Encoding: 8bit',
          '',
          attachmentBody.replace(/\r?\n/g, CRLF),
          '',
        ].join(CRLF)
      : '') +
    '--' + boundary +
    '--' +
    CRLF;

  if (/(^|[^\r])\n/.test(message)) {
    throw new TypeError('MIME message contains a bare LF line ending');
  }

  return { boundary, message };
}
