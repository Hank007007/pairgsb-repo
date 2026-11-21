const CRLF = '\r\n';

/**
 * Generate a multipart boundary string.
 *
 * @returns {string}
 */
function makeBoundary() {
  return '----=_Part_' + Math.random().toString(36).slice(2);
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
  const boundary = makeBoundary();

  const headers = [];
  headers.push('MIME-Version: 1.0');
  if (subject) headers.push('Subject: ' + subject);
  headers.push('Content-Type: multipart/mixed; boundary=' + boundary);

  const body = [];
  body.push('--' + boundary);
  body.push('Content-Type: text/calendar; method=REQUEST; charset=UTF-8');
  body.push('Content-Transfer-Encoding: 8bit');
  body.push('');
  body.push(calendar);

  if (attachment) {
    body.push('--' + boundary);
    body.push('Content-Type: ' + (attachment.type || 'text/plain') + '; charset=UTF-8');
    body.push('Content-Transfer-Encoding: 8bit');
    body.push('');
    body.push(attachment.description || '');
  }

  body.push('--' + boundary + '--');

  const message = headers.join(CRLF) + CRLF + CRLF + body.join('\n') + '\n';
  return { boundary, message };
}